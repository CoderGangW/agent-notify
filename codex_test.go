package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The desktop-app config shape that broke real installs: notify already
// owned by codex-computer-use.exe, table sections below.
const codexDesktopConfig = `model = "gpt-5.6-sol"
model_reasoning_effort = "high"
notify = [ "C:\\Users\\R2SOFT\\AppData\\Local\\OpenAI\\Codex\\bin\\codex-computer-use.exe", "turn-ended" ]
service_tier = "default"
[windows]
sandbox = "elevated"

[projects.'C:\Personal\work']
trust_level = "trusted"
`

func setupCodexHome(t *testing.T, config string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", "")
	if config != "" {
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestParseTOMLStringArray(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		ok   bool
	}{
		{`[ "C:\\Users\\x.exe", "turn-ended" ]`, []string{`C:\Users\x.exe`, "turn-ended"}, true},
		{`['C:\Users\x.exe']`, []string{`C:\Users\x.exe`}, true},
		{`[]`, []string{}, true},
		{`["a", "b"] # comment`, []string{"a", "b"}, true},
		{`["quote \" and \u0041"]`, []string{`quote " and A`}, true},
		{`"not-an-array"`, nil, false},
		{`[ "unterminated`, nil, false},
		{`[ 42 ]`, nil, false},
		{`["a"] trailing`, nil, false},
	}
	for _, c := range cases {
		got, err := parseTOMLStringArray(c.in)
		if c.ok != (err == nil) {
			t.Errorf("%s: err=%v, want ok=%v", c.in, err, c.ok)
			continue
		}
		if !c.ok {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestTomlStringRoundTrip(t *testing.T) {
	for _, s := range []string{`C:\Users\R2SOFT\bin\x.exe`, `with "quote"`, "한글 경로/bin", "tab\there"} {
		got, rest, err := parseTOMLString(tomlString(s))
		if err != nil || rest != "" || got != s {
			t.Errorf("round trip %q: got %q rest %q err %v", s, got, rest, err)
		}
	}
}

func TestInstallChainsForeignNotify(t *testing.T) {
	setupCodexHome(t, codexDesktopConfig)

	if codexHooked() {
		t.Fatal("hooked before install")
	}
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	if !codexHooked() {
		t.Fatal("not hooked after install")
	}
	chain := loadCodexChain()
	if len(chain) != 2 || chain[1] != "turn-ended" || !strings.HasSuffix(chain[0], "codex-computer-use.exe") {
		t.Fatalf("chain not captured: %v", chain)
	}

	// the notify line must stay top-level and be the only one
	lines, idx, header, argv, err := readCodexNotify()
	if err != nil || idx < 0 || !isOurCodexArgv(argv) {
		t.Fatalf("notify line wrong: idx=%d argv=%v err=%v", idx, argv, err)
	}
	if header >= 0 && idx > header {
		t.Fatal("notify ended up under a table header")
	}
	if strings.Count(strings.Join(lines, "\n"), "notify = [") != 1 {
		t.Fatal("duplicate notify lines")
	}
	// untouched sections survive
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"[windows]", "[projects.'C:\\Personal\\work']", `model = "gpt-5.6-sol"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("lost config content: %s", want)
		}
	}
}

func TestInstallFreshConfigInsertsAboveTables(t *testing.T) {
	setupCodexHome(t, "[projects.'C:\\x']\ntrust_level = \"trusted\"\n")

	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(codexConfigPath())
	lines := strings.Split(string(data), "\n")
	if !strings.HasPrefix(lines[0], "notify = [") {
		t.Fatalf("notify not top-level: first line %q", lines[0])
	}
	if !codexHooked() {
		t.Fatal("not hooked")
	}
	if len(loadCodexChain()) != 0 {
		t.Fatal("chain saved with no foreign owner")
	}
}

func TestInstallNoConfig(t *testing.T) {
	setupCodexHome(t, "")
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	if !codexHooked() {
		t.Fatal("not hooked")
	}
}

func TestRepairAfterAppReclaimsSlot(t *testing.T) {
	setupCodexHome(t, codexDesktopConfig)
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}

	// desktop app update rewrites config.toml with a new runtime path
	stomped := strings.Replace(codexDesktopConfig,
		`Codex\\bin\\codex-computer-use.exe`, `Codex\\bin2\\codex-computer-use.exe`, 1)
	if err := os.WriteFile(codexConfigPath(), []byte(stomped), 0o644); err != nil {
		t.Fatal(err)
	}
	if codexHooked() {
		t.Fatal("stomp not detected")
	}

	codexRepairHook()
	if !codexHooked() {
		t.Fatal("repair did not reclaim the slot")
	}
	chain := loadCodexChain()
	if len(chain) != 2 || !strings.Contains(chain[0], `bin2`) {
		t.Fatalf("chain not refreshed to the app's new path: %v", chain)
	}
}

func TestRepairNeedsOptIn(t *testing.T) {
	setupCodexHome(t, codexDesktopConfig)
	codexRepairHook() // no chain file: user never installed
	if codexHooked() {
		t.Fatal("repair ran without prior install")
	}
}

func TestUninstallRestoresOriginal(t *testing.T) {
	setupCodexHome(t, codexDesktopConfig)
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	if !uninstallCodexHook() {
		t.Fatal("uninstall reported no change")
	}
	_, idx, _, argv, err := readCodexNotify()
	if err != nil || idx < 0 || len(argv) != 2 || argv[1] != "turn-ended" ||
		!strings.HasSuffix(argv[0], "codex-computer-use.exe") {
		t.Fatalf("original notify not restored: idx=%d argv=%v err=%v", idx, argv, err)
	}
	if len(loadCodexChain()) != 0 {
		t.Fatal("chain file not removed")
	}
	if uninstallCodexHook() {
		t.Fatal("second uninstall touched a slot we no longer own")
	}
}

func TestUninstallRemovesLineWhenNoChain(t *testing.T) {
	setupCodexHome(t, "model = \"gpt-5.6-sol\"\n")
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	if !uninstallCodexHook() {
		t.Fatal("uninstall reported no change")
	}
	data, _ := os.ReadFile(codexConfigPath())
	if strings.Contains(string(data), "notify") {
		t.Fatalf("notify line not removed: %s", data)
	}
	if !strings.Contains(string(data), "gpt-5.6-sol") {
		t.Fatal("lost config content")
	}
}

func TestInstallRefusesUnparsableNotify(t *testing.T) {
	setupCodexHome(t, "notify = { bad = true }\n")
	if err := installCodexHook(); err == nil {
		t.Fatal("expected error on unparsable notify")
	}
	data, _ := os.ReadFile(codexConfigPath())
	if !strings.Contains(string(data), "{ bad = true }") {
		t.Fatal("unparsable notify was rewritten")
	}
}

func TestNotifyInsideTableIgnored(t *testing.T) {
	// a notify key inside a table is not the top-level slot
	setupCodexHome(t, "[sometool]\nnotify = [\"x\"]\n")
	if codexHooked() {
		t.Fatal("table-scoped notify treated as top-level")
	}
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(codexConfigPath())
	lines := strings.Split(string(data), "\n")
	if !strings.HasPrefix(lines[0], "notify = [") {
		t.Fatalf("our notify not inserted top-level: %q", lines[0])
	}
	if !strings.Contains(string(data), "notify = [\"x\"]") {
		t.Fatal("table-scoped notify was touched")
	}
	if len(loadCodexChain()) != 0 {
		t.Fatal("chained a table-scoped notify")
	}
}

// wrappedConfig renders the desktop app's newer shape: its helper wrapping
// a previous notify command via --previous-notify (openai/codex#28404).
func wrappedConfig(t *testing.T, prev []string) string {
	t.Helper()
	wrapped, _ := json.Marshal(prev)
	argv := []string{`C:\Users\R2SOFT\AppData\Local\OpenAI\Codex\bin\codex-computer-use.exe`, "turn-ended", "--previous-notify", string(wrapped)}
	return "model = \"gpt-5.6-sol\"\n" + codexNotifyValue(argv) + "\n[projects.'C:\\x']\ntrust_level = \"trusted\"\n"
}

func putCodexConfig(t *testing.T, cfg string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(codexConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexConfigPath(), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWrappedByHelperCountsAsHooked(t *testing.T) {
	home := setupCodexHome(t, "")
	ourExe := filepath.Join(home, "agent-notify.exe")
	if err := os.WriteFile(ourExe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := wrappedConfig(t, []string{ourExe, "codex-hook"})
	putCodexConfig(t, cfg)

	if !codexHooked() {
		t.Fatal("helper wrapping our hook must count as hooked")
	}
	if codexOwnsSlot() {
		t.Fatal("wrapped slot reported as directly owned")
	}
	// install is a no-op: rewriting the app's wrapper would restart its war
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(codexConfigPath())
	if string(data) != cfg {
		t.Fatalf("install rewrote the app's wrapper:\n%s", data)
	}
	// and repair must stay quiet even with a chain backup lying around
	if err := saveCodexChain([]string{`C:\old\helper.exe`, "turn-ended"}); err != nil {
		t.Fatal(err)
	}
	codexRepairHook()
	data, _ = os.ReadFile(codexConfigPath())
	if string(data) != cfg {
		t.Fatalf("repair rewrote the app's wrapper:\n%s", data)
	}
}

func TestWrappedStaleBinaryRepointed(t *testing.T) {
	setupCodexHome(t, "")
	cfg := wrappedConfig(t, []string{`C:\gone\claude-notify.exe`, "codex-hook"})
	putCodexConfig(t, cfg)
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	_, idx, _, argv, err := readCodexNotify()
	if err != nil || idx < 0 || len(argv) != 4 || argv[2] != "--previous-notify" {
		t.Fatalf("wrapper shape lost: %v (err %v)", argv, err)
	}
	if !strings.HasSuffix(argv[0], "codex-computer-use.exe") {
		t.Fatalf("helper prefix lost: %v", argv)
	}
	_, prev := codexPreviousNotify(argv)
	if !isOurCodexArgv(prev) || strings.Contains(prev[0], `C:\gone`) || !fileExists(prev[0]) {
		t.Fatalf("wrapped command not repointed at the live binary: %v", prev)
	}
	if !codexHooked() {
		t.Fatal("not hooked after repoint")
	}
	if len(loadCodexChain()) != 0 {
		t.Fatal("must not chain the helper that is already calling us")
	}
}

func TestUninstallStripsPreviousNotify(t *testing.T) {
	home := setupCodexHome(t, "")
	ourExe := filepath.Join(home, "agent-notify.exe")
	if err := os.WriteFile(ourExe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := wrappedConfig(t, []string{ourExe, "codex-hook"})
	putCodexConfig(t, cfg)
	if !uninstallCodexHook() {
		t.Fatal("uninstall reported no change")
	}
	_, idx, _, argv, err := readCodexNotify()
	if err != nil || idx < 0 || len(argv) != 2 || argv[1] != "turn-ended" {
		t.Fatalf("helper not left standalone: %v (err %v)", argv, err)
	}
	if codexHooked() {
		t.Fatal("still hooked after uninstall")
	}
}

func TestArgvMentionsUsGuard(t *testing.T) {
	ours := []string{`C:\a\agent-notify.exe`, "codex-hook"}
	wrapped, _ := json.Marshal(ours)
	cases := map[string][]string{
		"direct":      ours,
		"wrapped":     {`C:\h.exe`, "turn-ended", "--previous-notify", string(wrapped)},
		"loose":       {`C:\h.exe`, "run", "something codex-hook something"},
		"other-owner": {`C:\h.exe`, "turn-ended", "--previous-notify", `["uv","run","notify.py"]`},
		"plain":       {`C:\h.exe`, "turn-ended"},
	}
	for name, argv := range cases {
		want := name != "other-owner" && name != "plain"
		if got := codexArgvMentionsUs(argv); got != want {
			t.Errorf("%s: mentions-us = %v, want %v", name, got, want)
		}
	}
}

// relayProbe wires a chain to a shell script that records the relay env
// marker and the payload it received.
func relayProbe(t *testing.T, home string) (out string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell probe")
	}
	out = filepath.Join(home, "relay.out")
	script := filepath.Join(home, "probe.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$AGENT_NOTIFY_CODEX_RELAY|$2\" > \""+out+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveCodexChain([]string{"/bin/sh", script, "turn-ended"}); err != nil {
		t.Fatal(err)
	}
	return out
}

func waitFile(path string) (string, bool) {
	for i := 0; i < 40; i++ {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data)), true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", false
}

func TestChainRelaysWithMarkerWhenSlotIsOurs(t *testing.T) {
	home := setupCodexHome(t, "")
	if err := installCodexHook(); err != nil {
		t.Fatal(err)
	}
	out := relayProbe(t, home)
	chainCodexNotify(`{"type":"agent-turn-complete"}`)
	got, ok := waitFile(out)
	if !ok {
		t.Fatal("helper was not relayed to")
	}
	if got != `1|{"type":"agent-turn-complete"}` {
		t.Fatalf("relay got %q", got)
	}
}

func TestChainSkipsWhenSlotIsNotOurs(t *testing.T) {
	home := setupCodexHome(t, codexDesktopConfig) // slot held by the app
	out := relayProbe(t, home)
	chainCodexNotify(`{}`)
	if _, ok := waitFile(out); ok {
		t.Fatal("relayed although the app holds the slot (helper → us → helper loop)")
	}
}
