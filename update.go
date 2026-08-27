package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Auto-update, ulio-style but tokenless (public repo): the daemon checks
// the latest GitHub release, downloads the matching binary, verifies it
// runs, and swaps it into the stable install path. Under launchd the
// daemon then exits non-zero so KeepAlive respawns the new version;
// elsewhere the update applies on next start.

const releaseAPI = "https://api.github.com/repos/CoderGangW/agent-notify/releases/latest"

func startAutoUpdate() {
	if loadConfig().DisableAutoUpdate {
		return
	}
	time.Sleep(2 * time.Minute) // let startup settle
	for {
		if err := checkAndApplyUpdate(); err != nil {
			fmt.Println("update check:", err)
		}
		time.Sleep(6 * time.Hour)
	}
}

func checkAndApplyUpdate() error {
	if loadConfig().DisableAutoUpdate {
		return nil
	}
	req, err := http.NewRequest("GET", releaseAPI, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "agent-notify/"+version)
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("release api: %s", resp.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if !versionNewer(latest, version) {
		return nil
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	want := fmt.Sprintf("agent-notify-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
	assetURL := ""
	for _, a := range rel.Assets {
		if a.Name == want {
			assetURL = a.URL
		}
	}
	if assetURL == "" {
		return fmt.Errorf("no asset %s in %s", want, rel.TagName)
	}

	target := installDest()
	if target == "" {
		return fmt.Errorf("no install dir")
	}
	tmp, err := downloadTo(assetURL, filepath.Dir(target))
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	// Sanity gate: the downloaded binary must run and report the new version.
	out, err := exec.Command(tmp, "version").Output()
	if err != nil || !strings.Contains(string(out), latest) {
		return fmt.Errorf("downloaded binary failed verification (%v: %q)", err, out)
	}

	if runtime.GOOS == "windows" {
		// A running exe can't be overwritten, but it can be renamed away.
		old := target + ".old"
		_ = os.Remove(old)
		_ = os.Rename(target, old)
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	fmt.Println("updated to", rel.TagName)

	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	if runtime.GOOS == "darwin" && self == target && os.Getppid() == 1 {
		// launchd KeepAlive(SuccessfulExit=false) restarts us as the new
		// version; the event feed and config are all on disk.
		os.Exit(1)
	}
	deliverNotification(Event{
		Kind:    "done",
		Title:   "agent-notify",
		Message: fmt.Sprintf(T("update.ready"), rel.TagName),
		Time:    time.Now(),
	})
	return nil
}

// installDest mirrors installBinary's target path.
func installDest() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	name := "agent-notify"
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" && filepath.Separator == '\\' {
		return filepath.Join(localAppData, "agent-notify", name+".exe")
	}
	return filepath.Join(home, ".local", "bin", name)
}

func downloadTo(url, dir string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "agent-notify/"+version)
	client := http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}
	f, err := os.CreateTemp(dir, ".agent-notify-update-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// versionNewer reports whether a > b for dotted numeric versions (extra
// segments count, like ulio's 4-segment hotfix tags).
func versionNewer(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var ai, bi int
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			return ai > bi
		}
	}
	return false
}
