package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Keep the whole test binary off the developer's real home: the first
// dataPath call migrates ~/.claude-notify for real.
func TestMain(m *testing.M) {
	home, _ := os.MkdirTemp("", "agent-notify-test-home-")
	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

func TestMigrateDataDir(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, legacyDirName)
	os.MkdirAll(legacy, 0o755)
	os.WriteFile(filepath.Join(legacy, "config.json"), []byte(`{"lang":"ko"}`), 0o644)

	migrateDataDir(home)

	got, err := os.ReadFile(filepath.Join(home, dataDirName, "config.json"))
	if err != nil || string(got) != `{"lang":"ko"}` {
		t.Fatalf("config not moved: %q %v", got, err)
	}
	if isRealDir(legacy) {
		t.Fatal("legacy path should be a link now, not a folder")
	}
	// an old daemon still writing to the legacy path lands in the new dir
	if err := os.WriteFile(filepath.Join(legacy, "stats.json"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, dataDirName, "stats.json")); err != nil {
		t.Fatal("write through legacy link missed the new dir")
	}
	migrateDataDir(home) // idempotent
	if dataDirFor(home) != filepath.Join(home, dataDirName) {
		t.Fatal("should resolve to the new dir")
	}
}

func TestDataDirFallsBackToLegacy(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, legacyDirName)
	os.MkdirAll(legacy, 0o755)
	// migration not done yet (e.g. rename failed): never start an empty new dir
	migrateOnce.Do(func() {}) // burn the once so dataDirFor won't migrate
	if got := dataDirFor(home); got != legacy {
		t.Fatalf("want legacy %s, got %s", legacy, got)
	}
}
