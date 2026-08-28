package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Antigravity model quota. There is no official usage API and no token
// count in the hook payload or local DBs, but the agy binary itself
// reads a live per-pool quota meter from the same private Code Assist
// endpoint the IDE uses. We replay that call:
//
//  1. read agy's OAuth credential from the OS keyring (gemini:antigravity)
//  2. discover agy's OAuth client id/secret from the installed binary
//  3. refresh the access token
//  4. POST v1internal:fetchAvailableModels with the Antigravity IDE
//     headers and read models[].quotaInfo.remainingFraction
//
// remainingFraction is a real, token-weighted meter (it drops as you
// spend), but it is POOLED: every Gemini model shares one Google pool,
// and every Anthropic + OpenAI model shares one Vertex pool together.
// So we surface two gauges, not one per model. Everything here is
// best-effort — any failure degrades to "unavailable", never a crash.

type agyPool struct {
	Key      string    `json:"key"`   // "google" | "vertex"
	Label    string    `json:"label"` // "Gemini" | "Claude + GPT"
	Fraction float64   `json:"fraction"`
	ResetsAt time.Time `json:"resetsAt,omitempty"`
}

type agyQuotaReport struct {
	Pools     []agyPool `json:"pools"`
	Account   string    `json:"account,omitempty"`
	Error     string    `json:"error,omitempty"`
	FetchedAt time.Time `json:"fetchedAt"`
}

type agyQuotaFetcher struct {
	mu     sync.Mutex
	last   agyQuotaReport
	client agyOAuthClient // memoized winning id/secret
}

type agyOAuthClient struct{ id, secret string }

var agyQuota = &agyQuotaFetcher{}

func (f *agyQuotaFetcher) report() agyQuotaReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Since(f.last.FetchedAt) < 60*time.Second {
		return f.last
	}
	r := f.fetch()
	if r.Error != "" && len(f.last.Pools) > 0 && time.Since(f.last.FetchedAt) < 30*time.Minute {
		// Keep slightly stale gauges through a transient failure.
		f.last.Error = r.Error
		f.last.FetchedAt = time.Now()
		return f.last
	}
	f.last = r
	return f.last
}

func (f *agyQuotaFetcher) fetch() agyQuotaReport {
	r := agyQuotaReport{FetchedAt: time.Now()}

	refresh, email := agyKeyringCredential()
	r.Account = email
	if refresh == "" {
		r.Error = T("agyquota.nocreds")
		return r
	}
	access, err := f.refreshAccessToken(refresh)
	if err != "" {
		r.Error = err
		return r
	}
	pools, err := agyFetchPools(access)
	if err != "" {
		r.Error = err
		return r
	}
	if len(pools) == 0 {
		r.Error = T("agyquota.none")
		return r
	}
	r.Pools = pools
	return r
}

// refreshAccessToken swaps agy's refresh token for a fresh access token.
// The OAuth client id/secret are not public, so we lift them from the
// installed agy binary and try each id×secret pair, memoizing the winner.
func (f *agyQuotaFetcher) refreshAccessToken(refresh string) (access, errMsg string) {
	candidates := []agyOAuthClient{}
	if f.client.id != "" {
		candidates = append(candidates, f.client)
	}
	candidates = append(candidates, agyOAuthClients()...)
	if len(candidates) == 0 {
		return "", T("agyquota.noclient")
	}

	last := ""
	for _, c := range candidates {
		form := url.Values{
			"client_id":     {c.id},
			"client_secret": {c.secret},
			"refresh_token": {refresh},
			"grant_type":    {"refresh_token"},
		}
		req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token",
			strings.NewReader(form.Encode()))
		if err != nil {
			last = err.Error()
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
		if err != nil {
			last = T("agyquota.network")
			continue
		}
		var tok struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&tok)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK && tok.AccessToken != "" {
			f.client = c // remember the pair that worked
			return tok.AccessToken, ""
		}
		last = resp.Status
	}
	if last == "" {
		last = T("agyquota.refresh")
	}
	return "", last
}

// agyFetchPools calls fetchAvailableModels and folds per-model quota into
// the Google and Vertex pools.
func agyFetchPools(access string) (pools []agyPool, errMsg string) {
	req, err := http.NewRequest("POST",
		"https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels",
		bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	// These identify the caller as the Antigravity IDE; without the
	// Client-Metadata marker the backend 404s the non-Google models.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
		"AppleWebKit/537.36 (KHTML, like Gecko) Antigravity/1.0.0 "+
		"Chrome/138.0.7204.235 Electron/37.3.1 Safari/537.36")
	req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	req.Header.Set("Client-Metadata", `{"ideType":"ANTIGRAVITY","platform":"MACOS","pluginType":"GEMINI"}`)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, T("agyquota.network")
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, T("agyquota.expired")
	case resp.StatusCode >= 300:
		return nil, resp.Status
	}

	var body struct {
		Models map[string]struct {
			IsInternal bool `json:"isInternal"`
			QuotaInfo  struct {
				RemainingFraction *float64  `json:"remainingFraction"`
				ResetTime         time.Time `json:"resetTime"`
			} `json:"quotaInfo"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err.Error()
	}

	// Fold every model into its pool, keeping the min fraction (all
	// models in a pool report the same value; min is defensive) and the
	// earliest reset.
	type acc struct {
		frac  float64
		reset time.Time
		seen  bool
	}
	pool := map[string]*acc{}
	fold := func(key string, frac float64, reset time.Time) {
		a := pool[key]
		if a == nil {
			a = &acc{frac: frac, reset: reset, seen: true}
			pool[key] = a
			return
		}
		if frac < a.frac {
			a.frac = frac
		}
		if !reset.IsZero() && (a.reset.IsZero() || reset.Before(a.reset)) {
			a.reset = reset
		}
	}
	for id, m := range body.Models {
		if m.IsInternal || strings.HasPrefix(id, "tab_") || strings.HasPrefix(id, "tab-") {
			continue
		}
		if m.QuotaInfo.RemainingFraction == nil {
			continue
		}
		fold(agyPoolKey(id), *m.QuotaInfo.RemainingFraction, m.QuotaInfo.ResetTime)
	}

	// Stable order: Vertex (Claude+GPT) then Google.
	for _, p := range []struct{ key, label string }{
		{"vertex", "Claude + GPT"},
		{"google", "Gemini"},
	} {
		if a := pool[p.key]; a != nil && a.seen {
			pools = append(pools, agyPool{Key: p.key, Label: p.label, Fraction: a.frac, ResetsAt: a.reset})
		}
	}
	return pools, ""
}

// agyPoolKey classifies a model id into its shared quota pool.
func agyPoolKey(id string) string {
	s := strings.ToLower(id)
	if strings.HasPrefix(s, "gemini") {
		return "google"
	}
	// Claude and GPT/OSS share one Vertex pool; anything non-Gemini
	// lands there too rather than vanishing.
	return "vertex"
}

// agyKeyringCredential returns agy's refresh token and account email.
// agy stores its OAuth blob in the OS keyring; the go-keyring service/
// account split isn't documented and a stray match must not shadow a
// good one, so we gather every candidate source and return the first
// that actually yields a refresh token. The gemini-cli oauth_creds.json
// (flat token) is the file fallback.
func agyKeyringCredential() (refresh, email string) {
	var blobs [][]byte
	if runtime.GOOS == "darwin" {
		// agy stores its token via go-keyring under service "gemini",
		// account "antigravity" (verified on-device).
		if out, err := exec.Command("security", "find-generic-password",
			"-s", "gemini", "-a", "antigravity", "-w").Output(); err == nil {
			if b := bytes.TrimSpace(out); len(b) > 0 {
				blobs = append(blobs, b)
			}
		}
	}
	// File fallback (non-darwin, or keyring miss). This is the gemini-cli
	// token, whose refresh belongs to a different OAuth client, so it is
	// tried last — only the keyring token is guaranteed agy's.
	if b, _ := os.ReadFile(homePath(".gemini", "oauth_creds.json")); len(b) > 0 {
		blobs = append(blobs, b)
	}
	for _, data := range blobs {
		if rt, em := agyParseToken(data); rt != "" {
			return rt, em
		}
	}
	return "", ""
}

// agyParseToken pulls a refresh token out of either the keyring blob
// ({"token":{...},"auth_method":...}) or a flat gemini-cli token.
func agyParseToken(data []byte) (refresh, email string) {
	// zalando/go-keyring (what agy uses) base64-wraps the secret behind a
	// "go-keyring-base64:" sentinel; unwrap it before parsing JSON.
	if s := bytes.TrimPrefix(data, []byte("go-keyring-base64:")); len(s) < len(data) {
		if dec, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(s))); err == nil {
			data = dec
		}
	}
	var blob struct {
		Token json.RawMessage `json:"token"`
	}
	tokenJSON := data
	if json.Unmarshal(data, &blob) == nil && len(blob.Token) > 0 {
		tokenJSON = blob.Token
	}
	var tok struct {
		RefreshToken string `json:"refresh_token"`
		Email        string `json:"email"`
	}
	if json.Unmarshal(tokenJSON, &tok) != nil {
		return "", ""
	}
	return tok.RefreshToken, tok.Email
}

var (
	agyClientRE    = regexp.MustCompile(`[0-9]{10,}-[a-z0-9]{16,}\.apps\.googleusercontent\.com`)
	agySecretRE    = regexp.MustCompile(`GOCSPX-[A-Za-z0-9_-]{28}`)
	agyClientsOnce sync.Once
	agyClientsMemo []agyOAuthClient
)

// agyOAuthClients scans the installed agy binary once for its embedded
// OAuth client ids and secrets, returning every id×secret combination.
func agyOAuthClients() []agyOAuthClient {
	agyClientsOnce.Do(func() {
		bin := findCLI("agy")
		if bin == "" {
			return
		}
		data, err := os.ReadFile(bin)
		if err != nil {
			return
		}
		ids := uniqueMatches(agyClientRE, data)
		secrets := uniqueMatches(agySecretRE, data)
		for _, id := range ids {
			for _, sec := range secrets {
				agyClientsMemo = append(agyClientsMemo, agyOAuthClient{id: id, secret: sec})
			}
		}
	})
	return agyClientsMemo
}

func uniqueMatches(re *regexp.Regexp, data []byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAll(data, -1) {
		s := string(m)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
