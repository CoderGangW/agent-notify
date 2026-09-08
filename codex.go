package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Codex CLI integration: ~/.codex/config.toml `notify` runs a program with
// one JSON argument per event. `agent-notify codex-hook` adapts that to a
// daemon event, so Codex turns land in the same tray/window/notifications.
//
// Codex has a single notify slot, and the Codex desktop app claims it for
// its Computer Use helper. Two shapes of that exist in the wild:
//
//   - the helper alone: `[helper, "turn-ended"]`. Install saves that argv
//     to codex-chain.json, takes the slot, and codex-hook relays every
//     event to the helper — both integrations stay live off one
//     registration.
//   - the helper wrapping whatever was there: `[helper, "turn-ended",
//     "--previous-notify", "<json argv>"]` (openai/codex#28404). When the
//     wrapped argv is ours the app is already calling us, so that counts
//     as hooked and nothing is rewritten — and codex-hook must NOT relay
//     to the helper, or helper → us → helper would recurse forever.

func codexConfigPath() string { return homePath(".codex", "config.toml") }
func codexChainPath() string  { return homePath(".claude-notify", "codex-chain.json") }

// codexRelayEnv marks a helper process we spawned; the helper's own
// --previous-notify callback inherits it, and that callback must not
// deliver a second time (or relay again).
const codexRelayEnv = "AGENT_NOTIFY_CODEX_RELAY"

// runCodexHook is invoked by Codex as `agent-notify codex-hook <json>`.
func runCodexHook() {
	defer os.Exit(0)
	if len(os.Args) < 3 || os.Getenv(codexRelayEnv) != "" {
		return
	}
	payload := os.Args[2]
	// Relay first, before any filtering: the chained program must see every
	// event Codex emits, not only the types we understand.
	chainCodexNotify(payload)

	var n struct {
		Type          string   `json:"type"`
		ThreadID      string   `json:"thread-id"`
		CWD           string   `json:"cwd"`
		InputMessages []string `json:"input-messages"`
		Last          string   `json:"last-assistant-message"`
	}
	if json.Unmarshal([]byte(payload), &n) != nil {
		return
	}
	if n.Type != "agent-turn-complete" {
		return
	}

	title := ""
	if len(n.InputMessages) > 0 {
		title = condense(n.InputMessages[0], 60)
	}
	if title == "" {
		title = "Codex"
	}
	cwd := n.CWD
	if cwd == "" {
		cwd, _ = os.Getwd() // older payloads: notify runs in the session's cwd
	}
	activate := ""
	if runtime.GOOS == "darwin" {
		activate = os.Getenv("__CFBundleIdentifier")
	}
	mux := muxContext()
	deliver(Event{
		// thread-id keys the event like a session id: the Windows toast
		// click-to-focus path looks the event up by it
		SessionID: n.ThreadID,
		CWD:       cwd,
		Kind:      "done",
		Source:    "codex",
		Title:     title,
		Activate:  activate,
		Mux:       mux,
		Message:   condense(n.Last, 180),
		Time:      time.Now(),
	})
}

// chainCodexNotify relays the raw notify JSON to the command that owned the
// notify slot before our install (the Codex desktop app). Fire-and-forget:
// a stale path after an app update must never block our own delivery.
// Relays only while we hold the slot directly — if the slot is the app's
// wrapper calling us back, the helper already ran for this event.
func chainCodexNotify(payload string) {
	argv := loadCodexChain()
	if len(argv) == 0 || codexArgvMentionsUs(argv) { // self-loop guard
		return
	}
	if !codexOwnsSlot() {
		return
	}
	if !fileExists(argv[0]) {
		return // app updated past this path; the next repair re-captures it
	}
	args := append(append([]string{}, argv[1:]...), payload)
	cmd := exec.Command(argv[0], args...)
	cmd.Env = append(os.Environ(), codexRelayEnv+"=1")
	hideConsole(cmd) // Windows: the app's helper is a console binary
	_ = cmd.Start()
}

// codexOwnsSlot reports whether the top-level notify is exactly our
// command (not the app's wrapper around it).
func codexOwnsSlot() bool {
	_, idx, _, argv, err := readCodexNotify()
	return idx >= 0 && err == nil && isOurCodexArgv(argv)
}

// codexPreviousNotify finds the desktop app's `--previous-notify <json>`
// pair in a helper argv and decodes the wrapped argv. valueIdx is the
// index of the JSON argument, -1 when the pair is absent or malformed.
func codexPreviousNotify(argv []string) (valueIdx int, prev []string) {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] != "--previous-notify" {
			continue
		}
		if json.Unmarshal([]byte(argv[i+1]), &prev) != nil {
			return -1, nil
		}
		return i + 1, prev
	}
	return -1, nil
}

// codexWrapsUs reports whether argv is the app's helper wrapping our
// command via --previous-notify.
func codexWrapsUs(argv []string) bool {
	_, prev := codexPreviousNotify(argv)
	return isOurCodexArgv(prev)
}

// codexArgvMentionsUs is the broad loop guard: our command anywhere in
// argv, wrapped or otherwise.
func codexArgvMentionsUs(argv []string) bool {
	if isOurCodexArgv(argv) || codexWrapsUs(argv) {
		return true
	}
	for _, a := range argv {
		if strings.Contains(a, "codex-hook") {
			return true
		}
	}
	return false
}

type codexChain struct {
	Argv []string `json:"argv"`
}

func loadCodexChain() []string {
	data, err := os.ReadFile(codexChainPath())
	if err != nil {
		return nil
	}
	var c codexChain
	if json.Unmarshal(data, &c) != nil {
		return nil
	}
	return c.Argv
}

func saveCodexChain(argv []string) error {
	path := codexChainPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.Marshal(codexChain{Argv: argv})
	return os.WriteFile(path, data, 0o644)
}

// isOurCodexArgv reports whether a notify argv already points at our
// codex-hook subcommand.
func isOurCodexArgv(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	if len(argv) >= 2 && argv[1] == "codex-hook" {
		return true
	}
	base := strings.ToLower(filepath.Base(argv[0]))
	return strings.Contains(base, "agent-notify") || strings.Contains(base, "claude-notify")
}

var codexNotifyKey = regexp.MustCompile(`^\s*notify\s*=`)

// readCodexNotify loads config.toml and locates the top-level notify
// assignment. notifyIdx/headerIdx are -1 when absent; headerIdx is the
// first table header, i.e. where a fresh top-level line must go. Only the
// region above the first header counts — a notify key inside [projects.*]
// or any other table is not the one Codex reads.
func readCodexNotify() (lines []string, notifyIdx, headerIdx int, argv []string, parseErr error) {
	data, _ := os.ReadFile(codexConfigPath())
	lines = strings.Split(string(data), "\n")
	notifyIdx, headerIdx = -1, -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			headerIdx = i
			break
		}
		if notifyIdx < 0 && codexNotifyKey.MatchString(l) {
			notifyIdx = i
		}
	}
	if notifyIdx >= 0 {
		eq := strings.Index(lines[notifyIdx], "=")
		argv, parseErr = parseTOMLStringArray(lines[notifyIdx][eq+1:])
	}
	return
}

func writeCodexConfig(lines []string) error {
	out := strings.Join(lines, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(codexConfigPath(), []byte(out), 0o644)
}

// codexHooked reports whether ~/.codex/config.toml's top-level notify slot
// runs our hook.
func codexHooked() bool {
	_, idx, _, argv, err := readCodexNotify()
	return idx >= 0 && err == nil && (isOurCodexArgv(argv) || codexWrapsUs(argv))
}

// installCodexHook wires the notify hook into ~/.codex/config.toml,
// chaining any existing owner (see the file comment).
func installCodexHook() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	exe = installBinary(exe)
	ourArgv := []string{exe, "codex-hook"}
	line := codexNotifyValue(ourArgv)

	path := codexConfigPath()
	lines, notifyIdx, headerIdx, argv, parseErr := readCodexNotify()

	if notifyIdx >= 0 {
		if parseErr != nil {
			// a notify we can't parse we also can't preserve: hands off
			return fmt.Errorf(T("codex.already")+"\n  %s", path, line)
		}
		if vi, prev := codexPreviousNotify(argv); vi >= 0 && isOurCodexArgv(prev) {
			// the app wrapped us: it already calls our hook, so leave its
			// wrapper alone — rewriting would only restart its rewrite. Only
			// the wrapped command is repointed when it names another copy
			// (an old name, a gone binary) so updates keep reaching it.
			if prev[0] == exe {
				return nil
			}
			wrapped, _ := json.Marshal(ourArgv)
			argv[vi] = string(wrapped)
			lines[notifyIdx] = codexNotifyValue(argv)
			return writeCodexConfig(lines)
		}
		if len(argv) > 0 && !isOurCodexArgv(argv) {
			// foreign owner (Codex desktop app): remember its command so
			// codex-hook relays events to it
			if err := saveCodexChain(argv); err != nil {
				return err
			}
		}
		lines[notifyIdx] = line
		return writeCodexConfig(lines)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if headerIdx >= 0 {
		// keep the new line top-level: above the first table header
		lines = append(lines[:headerIdx], append([]string{line}, lines[headerIdx:]...)...)
	} else {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, line)
	}
	return writeCodexConfig(lines)
}

// codexRepairHook re-installs our hook after the Codex desktop app
// reclaims the notify slot (it rewrites config.toml on updates). Acts only
// when a chain backup exists — proof the user opted in — so one hook
// registration survives app updates; re-installing re-captures the app's
// new command into the chain.
var (
	codexRepairMu   sync.Mutex
	codexRepairLast time.Time
	codexRepairHits int
)

func codexRepairHook() {
	_, idx, _, argv, err := readCodexNotify()
	if idx < 0 || err != nil || len(argv) == 0 {
		return // slot empty or unparsable: nothing to reclaim safely
	}
	if _, prev := codexPreviousNotify(argv); isOurCodexArgv(prev) {
		// the app's wrapper calls us; only make sure it calls the canonical
		// copy (a release-asset name would never see an update). One-shot
		// and deterministic — no backoff needed.
		if prev[0] != installDest() {
			_ = installCodexHook()
		}
		return
	}
	if len(loadCodexChain()) == 0 || codexHooked() {
		return
	}
	codexRepairMu.Lock()
	defer codexRepairMu.Unlock()
	// The running desktop app fights back: it watches config.toml and
	// reasserts its command, restarting its helper on every rewrite (a
	// visibly strobing window on Windows). A slot that held for an hour
	// was a one-off rewrite (app update) — reset the strike count; a slot
	// snapping back within minutes is an active war — re-take at most
	// every 10 minutes and give up for this run after 3 rounds.
	if time.Since(codexRepairLast) > time.Hour {
		codexRepairHits = 0
	}
	if codexRepairHits >= 3 || time.Since(codexRepairLast) < 10*time.Minute {
		return
	}
	codexRepairLast = time.Now()
	codexRepairHits++
	_ = installCodexHook()
}

// uninstallCodexHook gives the notify slot back: the chained original
// command if one was saved, else the line is removed. Reports whether it
// changed the config; a slot we don't own is left untouched.
func uninstallCodexHook() bool {
	lines, idx, _, argv, err := readCodexNotify()
	if idx < 0 || err != nil {
		return false
	}
	chain := loadCodexChain()
	switch {
	case codexWrapsUs(argv):
		// the app's wrapper: drop only our --previous-notify pair
		vi, _ := codexPreviousNotify(argv)
		argv = append(append([]string{}, argv[:vi-1]...), argv[vi+1:]...)
		lines[idx] = codexNotifyValue(argv)
	case !isOurCodexArgv(argv):
		return false
	case len(chain) > 0 && !isOurCodexArgv(chain):
		lines[idx] = codexNotifyValue(chain)
	default:
		lines = append(lines[:idx], lines[idx+1:]...)
	}
	if writeCodexConfig(lines) != nil {
		return false
	}
	_ = os.Remove(codexChainPath())
	return true
}

// runInstallCodex is the CLI entry (`agent-notify install-codex`).
func runInstallCodex() {
	if err := installCodexHook(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf(T("codex.installed")+"\n", codexConfigPath())
	if chain := loadCodexChain(); len(chain) > 0 {
		fmt.Printf(T("codex.chained")+"\n", chain[0])
	}
}

// codexNotifyValue renders argv as a `notify = [...]` TOML line.
func codexNotifyValue(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = tomlString(a)
	}
	return "notify = [" + strings.Join(parts, ", ") + "]"
}

// tomlString renders s as a TOML basic string.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// parseTOMLStringArray parses a single-line TOML array of strings, e.g.
// `[ "C:\\x.exe", 'literal' ]`. Anything else — non-string elements,
// unknown escapes, a multi-line array — errors, so callers refuse to
// rewrite a value they could not faithfully restore.
func parseTOMLStringArray(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return nil, fmt.Errorf("notify is not an array")
	}
	rest := s[1:]
	out := []string{}
	for {
		rest = strings.TrimLeft(rest, " \t,")
		if rest == "" {
			return nil, fmt.Errorf("unterminated array")
		}
		if rest[0] == ']' {
			if tail := strings.TrimSpace(rest[1:]); tail != "" && !strings.HasPrefix(tail, "#") {
				return nil, fmt.Errorf("trailing content after array")
			}
			return out, nil
		}
		val, next, err := parseTOMLString(rest)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
		rest = next
	}
}

func parseTOMLString(s string) (val, rest string, err error) {
	if s == "" {
		return "", "", fmt.Errorf("empty value")
	}
	switch s[0] {
	case '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return "", "", fmt.Errorf("unterminated literal string")
		}
		return s[1 : 1+end], s[2+end:], nil
	case '"':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			c := s[i]
			switch c {
			case '"':
				return b.String(), s[i+1:], nil
			case '\\':
				i++
				if i >= len(s) {
					return "", "", fmt.Errorf("truncated escape")
				}
				switch s[i] {
				case '"', '\\':
					b.WriteByte(s[i])
				case 'b':
					b.WriteByte('\b')
				case 't':
					b.WriteByte('\t')
				case 'n':
					b.WriteByte('\n')
				case 'f':
					b.WriteByte('\f')
				case 'r':
					b.WriteByte('\r')
				case 'u', 'U':
					n := 4
					if s[i] == 'U' {
						n = 8
					}
					if i+n >= len(s) {
						return "", "", fmt.Errorf("truncated unicode escape")
					}
					code, err := strconv.ParseUint(s[i+1:i+1+n], 16, 32)
					if err != nil {
						return "", "", fmt.Errorf("bad unicode escape")
					}
					b.WriteRune(rune(code))
					i += n
				default:
					return "", "", fmt.Errorf("unknown escape \\%c", s[i])
				}
			default:
				b.WriteByte(c)
			}
		}
		return "", "", fmt.Errorf("unterminated string")
	}
	return "", "", fmt.Errorf("array element is not a string")
}
