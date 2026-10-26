package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The expected values are Codex's own trusted_hash fixtures
// (codex-rs core/tests/suite/hooks.rs, plugin PreToolUse/PostToolUse with
// matcher ^Bash$ and no timeout, which normalizes to 600 s).
func TestCodexHookHash(t *testing.T) {
	cmd := "python3 -I -S ${PLUGIN_ROOT}/hooks/pre_tool_use_hook.py"
	cases := map[string]string{
		"pre_tool_use":  "sha256:c9ee1aca1b5d8c72b99df6571c87d61350b09f7f4594ae30e7250a2027e9d76b",
		"post_tool_use": "sha256:9eef43ad6f26b1f9b94f318a50445f8bb62c363da08152a2966b73d308bf42ba",
	}
	for ev, want := range cases {
		if got := codexHookHash(ev, "^Bash$", cmd, 600); got != want {
			t.Errorf("%s: got %s, want %s", ev, got, want)
		}
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCodexHooksInstall(t *testing.T) {
	home := setupCodexHome(t, "\uFEFFmodel = \"gpt-x\"\n\n[hooks.state.\"KEEP\"]\nenabled = false\n")
	codexMode = "hooks"
	exe := filepath.Join(home, ".local", "bin", "agent-notify")
	// a user Stop hook already sits at group 0 and must keep its position
	userHooks := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done"}]}]}}`
	if err := os.WriteFile(codexHooksPath(), []byte(userHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ { // second round: idempotent
		if err := installCodexHooks(exe); err != nil {
			t.Fatal(err)
		}
	}
	var doc struct {
		Hooks map[string][]struct {
			Hooks []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(readFile(t, codexHooksPath())), &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Hooks["Stop"][0].Hooks[0]["command"]; got != "say done" {
		t.Fatalf("user Stop hook moved: group 0 = %v", got)
	}
	if len(doc.Hooks["Stop"]) != 2 || len(doc.Hooks["PermissionRequest"]) != 1 {
		t.Fatalf("unexpected groups: %+v", doc.Hooks)
	}
	cmd := codexHookCommand(exe)
	cfg := readFile(t, codexConfigPath())
	for key, ev := range map[string]string{":stop:1:0": "stop", ":permission_request:0:0": "permission_request"} {
		header := "[hooks.state.'" + codexHooksPath() + key + "']"
		if n := strings.Count(cfg, header); n != 1 {
			t.Fatalf("%s: %d tables, want 1\n%s", header, n, cfg)
		}
		if !strings.Contains(cfg, header+"\ntrusted_hash = \""+codexHookHash(ev, "", cmd, codexHookTimeout)+"\"") {
			t.Fatalf("%s: wrong or missing hash\n%s", header, cfg)
		}
	}
	if !strings.Contains(cfg, "[hooks.state.\"KEEP\"]\nenabled = false") {
		t.Fatalf("unrelated table disturbed:\n%s", cfg)
	}
	if strings.HasPrefix(cfg, "\uFEFF") {
		t.Fatal("BOM kept")
	}
}

func TestCodexTrustRepairAndUninstall(t *testing.T) {
	home := setupCodexHome(t, "")
	codexMode = "hooks"
	t.Setenv("LOCALAPPDATA", "")
	exe := installDest() // what codexTrustOK expects the handler to run
	_ = home
	if err := installCodexHooks(exe); err != nil {
		t.Fatal(err)
	}
	if !codexTrustOK() {
		t.Fatal("fresh install not trusted")
	}
	// an app rewrite drops the trust: detected, then fixed by a reinstall
	cfg := readFile(t, codexConfigPath())
	cfg = strings.Replace(cfg, "sha256:", "sha256:00", 1)
	if err := os.WriteFile(codexConfigPath(), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if codexTrustOK() {
		t.Fatal("tampered trust not detected")
	}
	if err := installCodexHooks(exe); err != nil {
		t.Fatal(err)
	}
	if !codexTrustOK() || strings.Count(readFile(t, codexConfigPath()), "[hooks.state.") != 2 {
		t.Fatalf("trust not repaired in place:\n%s", readFile(t, codexConfigPath()))
	}
	// a user group added after ours must keep its index when we leave
	var doc map[string]any
	_ = json.Unmarshal([]byte(readFile(t, codexHooksPath())), &doc)
	h := doc["hooks"].(map[string]any)
	h["Stop"] = append(h["Stop"].([]any), map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "say later"}}})
	b, _ := json.Marshal(doc)
	_ = os.WriteFile(codexHooksPath(), b, 0o644)
	if !uninstallCodexHooks() {
		t.Fatal("uninstall reported no change")
	}
	out := readFile(t, codexHooksPath())
	if strings.Contains(out, "codex-hook") {
		t.Fatalf("our handler left behind:\n%s", out)
	}
	_ = json.Unmarshal([]byte(out), &doc)
	stop := doc["hooks"].(map[string]any)["Stop"].([]any)
	if len(stop) != 2 || !strings.Contains(fmt.Sprint(stop[1]), "say later") {
		t.Fatalf("user group shifted: %v", stop)
	}
}
