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
	"time"
)

// Codex CLI integration: ~/.codex/config.toml `notify` runs a program with
// one JSON argument per event. `agent-notify codex-hook` adapts that to a
// daemon event, so Codex turns land in the same tray/window/notifications.
//
// Codex has a single notify slot, and the Codex desktop app claims it for
// its own binary. When that happens, install saves the app's command to
// codex-chain.json and codex-hook relays every event to it — both the
// desktop app and our notifications stay live off one registration.

func codexConfigPath() string { return homePath(".codex", "config.toml") }
func codexChainPath() string  { return homePath(".claude-notify", "codex-chain.json") }

// runCodexHook is invoked by Codex as `agent-notify codex-hook <json>`.
func runCodexHook() {
	defer os.Exit(0)
	if len(os.Args) < 3 {
		return
	}
	payload := os.Args[2]
	// Relay first, before any filtering: the chained program must see every
	// event Codex emits, not only the types we understand.
	chainCodexNotify(payload)

	var n struct {
		Type          string   `json:"type"`
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
	cwd, _ := os.Getwd() // notify runs in the session's working directory
	activate := ""
	if runtime.GOOS == "darwin" {
		activate = os.Getenv("__CFBundleIdentifier")
	}
	mux := muxContext()
	deliver(Event{
		CWD:      cwd,
		Kind:     "done",
		Source:   "codex",
		Title:    title,
		Activate: activate,
		Mux:      mux,
		Message:  condense(n.Last, 180),
		Time:     time.Now(),
	})
}

// chainCodexNotify relays the raw notify JSON to the command that owned the
// notify slot before our install (the Codex desktop app). Fire-and-forget:
// a stale path after an app update must never block our own delivery.
func chainCodexNotify(payload string) {
	argv := loadCodexChain()
	if len(argv) == 0 || isOurCodexArgv(argv) { // self-loop guard
		return
	}
	if !fileExists(argv[0]) {
		return // app updated past this path; the next repair re-captures it
	}
	args := append(append([]string{}, argv[1:]...), payload)
	_ = exec.Command(argv[0], args...).Start()
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
	return idx >= 0 && err == nil && isOurCodexArgv(argv)
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
	line := codexNotifyValue([]string{exe, "codex-hook"})

	path := codexConfigPath()
	lines, notifyIdx, headerIdx, argv, parseErr := readCodexNotify()

	if notifyIdx >= 0 {
		if parseErr != nil {
			// a notify we can't parse we also can't preserve: hands off
			return fmt.Errorf(T("codex.already")+"\n  %s", path, line)
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
func codexRepairHook() {
	if len(loadCodexChain()) == 0 || codexHooked() {
		return
	}
	_, idx, _, argv, err := readCodexNotify()
	if idx < 0 || err != nil || len(argv) == 0 {
		return // slot empty or unparsable: nothing to reclaim safely
	}
	_ = installCodexHook()
}

// uninstallCodexHook gives the notify slot back: the chained original
// command if one was saved, else the line is removed. Reports whether it
// changed the config; a slot we don't own is left untouched.
func uninstallCodexHook() bool {
	lines, idx, _, argv, err := readCodexNotify()
	if idx < 0 || err != nil || !isOurCodexArgv(argv) {
		return false
	}
	chain := loadCodexChain()
	if len(chain) > 0 && !isOurCodexArgv(chain) {
		lines[idx] = codexNotifyValue(chain)
	} else {
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
