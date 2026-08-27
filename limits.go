package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Plan limits come from the same endpoint `claude /usage` uses. It is
// unofficial: failures degrade to "unavailable", never to a crash. The
// stored refresh token is never touched — an expired access token just
// reports an error until Claude Code itself refreshes it.

type limitBucket struct {
	Key         string    `json:"key"` // five_hour, seven_day, ...
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resetsAt"`
}

type limitsReport struct {
	Buckets   []limitBucket `json:"buckets"`
	Plan      string        `json:"plan,omitempty"` // subscription type from stored credentials
	Error     string        `json:"error,omitempty"`
	FetchedAt time.Time     `json:"fetchedAt"`
}

type limitsFetcher struct {
	mu   sync.Mutex
	last limitsReport
}

var limits = &limitsFetcher{}

func (l *limitsFetcher) report() limitsReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.last.FetchedAt) < 60*time.Second {
		return l.last
	}
	r := fetchLimits()
	if r.Error != "" && len(l.last.Buckets) > 0 && time.Since(l.last.FetchedAt) < 30*time.Minute {
		// Keep showing slightly stale data through transient failures.
		l.last.Error = r.Error
		l.last.FetchedAt = time.Now()
		return l.last
	}
	l.last = r
	return l.last
}

func fetchLimits() limitsReport {
	r := limitsReport{FetchedAt: time.Now()}
	token, plan := oauthCredentials()
	r.Plan = plan
	if token == "" {
		r.Error = T("limits.nocreds")
		return r
	}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-code/"+claudeCLIVersion())

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		r.Error = T("limits.network")
		return r
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		r.Error = T("limits.expired")
		return r
	case resp.StatusCode >= 300:
		r.Error = resp.Status
		return r
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		r.Error = err.Error()
		return r
	}
	for key, msg := range raw {
		var b struct {
			Utilization float64   `json:"utilization"`
			ResetsAt    time.Time `json:"resets_at"`
		}
		if json.Unmarshal(msg, &b) != nil || b.ResetsAt.IsZero() {
			continue
		}
		r.Buckets = append(r.Buckets, limitBucket{Key: key, Utilization: b.Utilization, ResetsAt: b.ResetsAt})
	}
	rank := map[string]int{"five_hour": 0, "seven_day": 1, "seven_day_sonnet": 2, "seven_day_opus": 3}
	sort.Slice(r.Buckets, func(i, j int) bool {
		ri, iok := rank[r.Buckets[i].Key]
		rj, jok := rank[r.Buckets[j].Key]
		if iok != jok {
			return iok
		}
		if ri != rj {
			return ri < rj
		}
		return r.Buckets[i].Key < r.Buckets[j].Key
	})
	return r
}

// oauthCredentials reads Claude Code's stored OAuth token and subscription
// type: macOS keychain first, then ~/.claude/.credentials.json.
func oauthCredentials() (token, plan string) {
	var data []byte
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("security", "find-generic-password",
			"-s", "Claude Code-credentials", "-w").Output()
		if err == nil {
			data = out
		}
	}
	if data == nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ""
		}
		b, err := os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
		if err != nil {
			return "", ""
		}
		data = b
	}
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken      string `json:"accessToken"`
			ExpiresAt        int64  `json:"expiresAt"` // ms epoch
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
	}
	if json.Unmarshal(data, &creds) != nil {
		return "", ""
	}
	c := creds.ClaudeAiOauth
	plan = strings.Title(strings.ReplaceAll(c.SubscriptionType, "_", " "))
	if c.ExpiresAt > 0 && time.Now().UnixMilli() > c.ExpiresAt {
		return "", plan
	}
	return c.AccessToken, plan
}

var (
	cliVersionOnce sync.Once
	cliVersion     = "2.0.0"
)

// claudeCLIVersion asks the installed CLI once; the endpoint rate-limits
// unknown user agents hard, so a real version string matters.
func claudeCLIVersion() string {
	cliVersionOnce.Do(func() {
		claude := findCLI("claude")
		if claude == "" {
			return
		}
		out, err := exec.Command(claude, "--version").Output()
		if err != nil {
			return
		}
		// "2.0.62 (Claude Code)" — first field is the version.
		if f := strings.Fields(strings.TrimSpace(string(out))); len(f) > 0 {
			cliVersion = f[0]
		}
	})
	return cliVersion
}
