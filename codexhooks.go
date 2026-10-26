package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// codexHookHash reproduces Codex's trust fingerprint for one command
// handler: sha256 over key-sorted, compact JSON of the normalized hook
// identity (codex-rs hooks/src/engine/discovery.rs hook_hash and
// config/src/fingerprint.rs version_for_toml). Since Codex 0.129 a user
// hook runs only when config.toml holds this hash as its trusted_hash.
func codexHookHash(eventKey, matcher, command string, timeoutSec int) string {
	id := map[string]any{
		"event_name": eventKey,
		"hooks": []any{map[string]any{
			"async": false, "command": command, "timeout": timeoutSec, "type": "command",
		}},
	}
	if matcher != "" {
		id["matcher"] = matcher
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // map keys come out sorted, like serde's sort_all_objects
	enc.SetEscapeHTML(false)     // serde_json leaves & < > as-is
	_ = enc.Encode(id)
	sum := sha256.Sum256(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Codex hooks integration (Codex ≥0.129). Hooks get the event JSON on
// stdin, which the legacy notify slot can't do: notify passes it as one
// argv element, and on Windows CreateProcess caps the command line at
// 32,767 chars — long turns simply never notified. Hooks also leave the
// notify slot to the Codex desktop app, ending the tug-of-war in codex.go.
//
// Codex only runs a user hook whose definition the user trusted: a
// [hooks.state.'<hooks.json path>:<event>:<group>:<handler>'] table in
// config.toml carrying its trusted_hash. Installing is the user's explicit
// opt-in, so we record that trust for our own handler alongside it.

const codexHookTimeout = 15 // seconds

var codexHookEvents = []struct{ name, key string }{
	{"Stop", "stop"},
	{"PermissionRequest", "permission_request"},
}

// codexHooksPath is the user-layer hooks.json; Codex canonicalizes a
// CODEX_HOME it is given, and the trust keys embed that exact path.
func codexHooksPath() string {
	home := codexHome(homePath(), os.Getenv)
	if os.Getenv("CODEX_HOME") != "" {
		if r, err := filepath.EvalSymlinks(home); err == nil {
			home = r
		}
	}
	return filepath.Join(home, "hooks.json")
}

// codexHookCommand runs through the user's shell — PowerShell on Windows.
// The forced success matters: a hook exiting 2 makes Codex continue the
// turn with its stderr as a new prompt.
func codexHookCommand(exe string) string {
	if runtime.GOOS == "windows" {
		return "& '" + strings.ReplaceAll(exe, "'", "''") + "' codex-hook | Out-Null; exit 0"
	}
	return "'" + strings.ReplaceAll(exe, "'", `'\''`) + "' codex-hook >/dev/null 2>&1 || true"
}

var (
	codexCapOnce sync.Once
	codexCap     bool
	codexMode    string // tests: "hooks" or "notify" pins the integration
)

// codexHooksCapable reports whether this machine's Codex runs trusted
// hooks (≥0.129, hooks not switched off). No CLI on PATH means only the
// desktop app, which bundles a current Codex.
func codexHooksCapable() bool {
	if codexMode != "" {
		return codexMode == "hooks"
	}
	codexCapOnce.Do(func() {
		if codexHooksDisabled() {
			return
		}
		bin := findCLI("codex")
		if bin == "" {
			codexCap = true
			return
		}
		cmd := exec.Command(bin, "--version")
		hideConsole(cmd)
		out, err := cmd.Output()
		if err != nil {
			return // unknown version: stay on notify
		}
		v := regexp.MustCompile(`\d+\.\d+\.\d+`).FindString(string(out))
		codexCap = v != "" && !versionNewer("0.129.0", v)
	})
	return codexCap
}

// codexHooksDisabled spots `hooks = false` (or the old `codex_hooks`) under
// [features] in config.toml.
func codexHooksDisabled() bool {
	data, _ := os.ReadFile(codexConfigPath())
	table := ""
	for _, l := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			table = t
			continue
		}
		if table == "[features]" && regexp.MustCompile(`^(codex_)?hooks\s*=\s*false\b`).MatchString(t) {
			return true
		}
	}
	return false
}

// codexHooksInstalled reports whether hooks.json carries our handler.
func codexHooksInstalled() bool {
	data, _ := os.ReadFile(codexHooksPath())
	return containsOurCommand(string(data))
}

// installCodexHooks writes our handler into hooks.json and trusts it.
func installCodexHooks(exe string) error {
	cmd := codexHookCommand(exe)
	idx, err := writeCodexHooksJSON(cmd)
	if err != nil {
		return err
	}
	return writeCodexTrust(codexTrustKeys(cmd, idx))
}

// writeCodexHooksJSON puts our handler under each event and returns its
// group index per event key. An existing group of ours is replaced in
// place: trust keys are positional, so moving or removing a group would
// silently untrust every user hook after it.
func writeCodexHooksJSON(cmd string) (map[string]int, error) {
	path := codexHooksPath()
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	idx := map[string]int{}
	for _, ev := range codexHookEvents {
		groups, _ := hooks[ev.name].([]any)
		entry := map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": cmd, "timeout": codexHookTimeout,
		}}}
		pos := -1
		for i, g := range groups {
			if b, _ := json.Marshal(g); containsOurCommand(string(b)) {
				pos = i
				break
			}
		}
		if pos < 0 {
			groups = append(groups, entry)
			pos = len(groups) - 1
		} else {
			groups[pos] = entry
		}
		hooks[ev.name] = groups
		idx[ev.key] = pos
	}
	doc["hooks"] = hooks
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep & and | readable in the PowerShell command
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return idx, os.WriteFile(path, buf.Bytes(), 0o644)
}

// codexTrustKeys maps each trust key for our handler to its hash. Windows
// gets both drive-letter cases (and, with CODEX_HOME, the \\?\ form
// canonicalize can produce): an unmatched key means the hook never runs.
func codexTrustKeys(cmd string, idx map[string]int) map[string]string {
	base := codexHooksPath()
	paths := []string{base}
	if runtime.GOOS == "windows" && len(base) > 1 && base[1] == ':' {
		d := base[:1]
		if strings.ToUpper(d) == d {
			d = strings.ToLower(d)
		} else {
			d = strings.ToUpper(d)
		}
		paths = append(paths, d+base[1:])
		if os.Getenv("CODEX_HOME") != "" {
			paths = append(paths, `\\?\`+base)
		}
	}
	out := map[string]string{}
	for _, ev := range codexHookEvents {
		hash := codexHookHash(ev.key, "", cmd, codexHookTimeout)
		for _, p := range paths {
			out[fmt.Sprintf("%s:%s:%d:0", p, ev.key, idx[ev.key])] = hash
		}
	}
	return out
}

// codexStateHeader recognizes a [hooks.state.<key>] table header.
func codexStateHeader(line string) (key string, ok bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "[hooks.state.") || !strings.HasSuffix(t, "]") || strings.HasPrefix(t, "[[") {
		return "", false
	}
	k, rest, err := parseTOMLString(strings.TrimSpace(t[len("[hooks.state.") : len(t)-1]))
	if err != nil || strings.TrimSpace(rest) != "" {
		return "", false
	}
	return k, true
}

// writeCodexTrust sets trusted_hash on each key's table in config.toml,
// editing an existing table in place (one Codex or an earlier install
// wrote; a duplicate table would make config.toml invalid) and appending
// the rest. Other keys in those tables, like enabled = false, are kept.
func writeCodexTrust(keys map[string]string) error {
	path := codexConfigPath()
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n")
	done := map[string]bool{}
	trustLine := regexp.MustCompile(`^\s*trusted_hash\s*=`)
	for i := 0; i < len(lines); i++ {
		k, ok := codexStateHeader(lines[i])
		hash, ours := keys[k]
		if !ok || !ours {
			continue
		}
		done[k] = true
		j := i + 1
		for ; j < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[j]), "["); j++ {
			if trustLine.MatchString(lines[j]) {
				lines[j] = "trusted_hash = " + tomlString(hash)
				break
			}
		}
		if j == len(lines) || strings.HasPrefix(strings.TrimSpace(lines[j]), "[") {
			lines = append(lines[:i+1], append([]string{"trusted_hash = " + tomlString(hash)}, lines[i+1:]...)...)
		}
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	var pending []string
	for k := range keys {
		if !done[k] {
			pending = append(pending, k)
		}
	}
	sort.Strings(pending)
	for _, k := range pending {
		key := "'" + k + "'"
		if strings.ContainsAny(k, "'\n") {
			key = tomlString(k) // a literal string can't hold these
		}
		lines = append(lines, "", "[hooks.state."+key+"]", "trusted_hash = "+tomlString(keys[k]))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// codexTrustOK reports whether config.toml still trusts our current
// handler — an app rewrite of config.toml or a regrouped hooks.json can
// leave it untrusted, and an untrusted hook silently never runs.
func codexTrustOK() bool {
	var doc struct {
		Hooks map[string][]any `json:"hooks"`
	}
	data, err := os.ReadFile(codexHooksPath())
	if err != nil || json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &doc) != nil {
		return true // nothing we can reason about; don't churn
	}
	cmd := codexHookCommand(installDest())
	idx := map[string]int{}
	for _, ev := range codexHookEvents {
		idx[ev.key] = -1
		for i, g := range doc.Hooks[ev.name] {
			if b, _ := json.Marshal(g); strings.Contains(string(b), "codex-hook") && containsOurCommand(string(b)) {
				idx[ev.key] = i
				break
			}
		}
		if idx[ev.key] < 0 {
			return false
		}
	}
	cfg, _ := os.ReadFile(codexConfigPath())
	have := map[string]string{}
	var cur string
	for _, l := range strings.Split(string(cfg), "\n") {
		if k, ok := codexStateHeader(l); ok {
			cur = k
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(l), "[") {
			cur = ""
			continue
		}
		if t := strings.TrimSpace(l); cur != "" && strings.HasPrefix(t, "trusted_hash") {
			if eq := strings.Index(t, "="); eq >= 0 {
				if v, _, err := parseTOMLString(strings.TrimSpace(t[eq+1:])); err == nil {
					have[cur] = v
				}
			}
		}
	}
	for k, want := range codexTrustKeys(cmd, idx) {
		if strings.HasPrefix(k, codexHooksPath()) && have[k] != want {
			return false // the primary key (exact path) must match
		}
	}
	return true
}

// uninstallCodexHooks takes our handler out of hooks.json. A group of ours
// that other groups follow is emptied rather than removed, so their
// positional trust keys keep pointing at the same hooks.
func uninstallCodexHooks() bool {
	path := codexHooksPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	doc := map[string]any{}
	if json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &doc) != nil {
		return false
	}
	hooks, _ := doc["hooks"].(map[string]any)
	changed := false
	for _, ev := range codexHookEvents {
		groups, _ := hooks[ev.name].([]any)
		for i := len(groups) - 1; i >= 0; i-- {
			if b, _ := json.Marshal(groups[i]); !containsOurCommand(string(b)) {
				continue
			}
			changed = true
			if i == len(groups)-1 {
				groups = groups[:i]
			} else {
				groups[i] = map[string]any{"hooks": []any{}}
			}
		}
		if len(groups) == 0 {
			delete(hooks, ev.name)
		} else {
			hooks[ev.name] = groups
		}
	}
	if !changed {
		return false
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
	return os.WriteFile(path, buf.Bytes(), 0o644) == nil
}

// runCodexStdinHook handles a Codex hook event (Stop, PermissionRequest)
// read from stdin.
func runCodexStdinHook() {
	data, err := readHookStdin()
	if err != nil {
		return
	}
	var in struct {
		SessionID  string          `json:"session_id"`
		TurnID     string          `json:"turn_id"`
		CWD        string          `json:"cwd"`
		Event      string          `json:"hook_event_name"`
		Transcript *string         `json:"transcript_path"`
		Last       string          `json:"last_assistant_message"`
		StopActive bool            `json:"stop_hook_active"`
		Tool       string          `json:"tool_name"`
		ToolInput  json.RawMessage `json:"tool_input"`
	}
	if json.Unmarshal(data, &in) != nil {
		return
	}
	// hidden helper threads (the desktop app fires a second Stop from one)
	// have no transcript; the user's own thread reports itself
	if in.Transcript == nil || *in.Transcript == "" {
		return
	}
	ev := Event{
		SessionID: in.SessionID, CWD: in.CWD, Source: "codex", Turn: in.TurnID, Time: time.Now(),
	}
	switch in.Event {
	case "Stop":
		if in.StopActive {
			return // another Stop hook continued the turn; the real end reports later
		}
		ev.Kind, ev.Message = "done", condense(in.Last, 180)
	case "PermissionRequest":
		ev.Kind = "attention"
		if d := toolDetail(in.ToolInput); d != "" {
			ev.Message = condense(in.Tool+": "+d, 180)
		} else {
			ev.Message = in.Tool
		}
	default:
		return
	}
	if ev.CWD == "" {
		ev.CWD, _ = os.Getwd()
	}
	ev.Title = codexNames(filepath.Join(codexHome(homePath(), os.Getenv), "session_index.jsonl"))[in.SessionID]
	if ev.Title == "" {
		ev.Title = projectName(ev.CWD)
	}
	ev.Title = condense(ev.Title, 60)
	if runtime.GOOS == "darwin" {
		ev.Activate = os.Getenv("__CFBundleIdentifier")
	}
	ev.Mux = muxContext()
	deliver(ev)
}
