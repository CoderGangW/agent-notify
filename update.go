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
// runs, and swaps it into the stable install path. A detached helper
// then relaunches the new version (launchctl kickstart / open) while
// the old daemon exits; elsewhere the update applies on next start.

const releaseAPI = "https://api.github.com/repos/CoderGangW/agent-notify/releases/latest"

// autoUpdateLoop only CHECKS and notifies — applying an update always
// takes an explicit user click (an auto-applying updater would hand a
// compromised release instant, silent code execution on every install).
func (s *daemonState) autoUpdateLoop() {
	time.Sleep(2 * time.Minute) // let startup settle
	notified := ""
	for {
		if !loadConfig().DisableAutoUpdate {
			if info, err := checkUpdate(); err == nil {
				s.mu.Lock()
				s.pendingUpdate = info
				s.mu.Unlock()
				if info.Available && notified != info.Latest {
					notified = info.Latest
					deliverNotification(Event{
						Kind:    "done",
						Title:   "agent-notify",
						Message: fmt.Sprintf(T("update.found"), "v"+info.Latest),
						Time:    time.Now(),
					})
				}
			}
		}
		time.Sleep(6 * time.Hour)
	}
}

// releaseInfo is one check against the latest GitHub release.
type releaseInfo struct {
	Latest    string // version without the v prefix
	AssetURL  string // download url for this platform, "" if missing
	AppZipURL string // macOS: the signed .app bundle zip, "" if missing
	Available bool
}

// appZipAsset picks the signed macOS bundle out of a release's assets.
func appZipAsset(assets []releaseAsset) string {
	for _, a := range assets {
		if a.Name == "agent-notify-macos-universal.app.zip" {
			return a.URL
		}
	}
	return ""
}

// platformAsset picks this platform's binary out of a release's assets.
func platformAsset(assets []releaseAsset) string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	want := fmt.Sprintf("agent-notify-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
	for _, a := range assets {
		if a.Name == want {
			return a.URL
		}
	}
	return ""
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// listReleases returns recent releases (newest first) that carry this
// platform's asset — the tray menu's version picker, which doubles as a
// downgrade path when a release misbehaves.
func listReleases(limit int) ([]releaseInfo, error) {
	url := fmt.Sprintf("%s?per_page=%d", strings.TrimSuffix(releaseAPI, "/latest"), limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "agent-notify/"+version)
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release api: %s", resp.Status)
	}
	var rels []struct {
		TagName string         `json:"tag_name"`
		Assets  []releaseAsset `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return nil, err
	}
	out := make([]releaseInfo, 0, len(rels))
	for _, rel := range rels {
		info := releaseInfo{
			Latest:    strings.TrimPrefix(rel.TagName, "v"),
			AssetURL:  platformAsset(rel.Assets),
			AppZipURL: appZipAsset(rel.Assets),
		}
		if info.AssetURL != "" {
			out = append(out, info)
		}
	}
	return out, nil
}

func checkUpdate() (releaseInfo, error) {
	var info releaseInfo
	req, err := http.NewRequest("GET", releaseAPI, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("User-Agent", "agent-notify/"+version)
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("release api: %s", resp.Status)
	}
	var rel struct {
		TagName string         `json:"tag_name"`
		Assets  []releaseAsset `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return info, err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.Available = versionNewer(info.Latest, version)
	info.AssetURL = platformAsset(rel.Assets)
	info.AppZipURL = appZipAsset(rel.Assets)
	return info, nil
}

// applyUpdate downloads, verifies, and swaps the binary. When restartOK
// and running as an installed copy (CLI path or app bundle), a detached
// helper relaunches the new version and this process exits, reporting
// restarted=true (the exit is deferred a beat so an HTTP caller gets its
// response first).
func applyUpdate(info releaseInfo, restartOK bool) (bool, error) {
	if info.AssetURL == "" {
		return false, fmt.Errorf("no asset for %s-%s in v%s", runtime.GOOS, runtime.GOARCH, info.Latest)
	}
	target := installDest()
	if target == "" {
		return false, fmt.Errorf("no install dir")
	}
	tmp, err := downloadTo(info.AssetURL, filepath.Dir(target))
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp)

	// Sanity gate: the downloaded binary must run and report the new version.
	verify := exec.Command(tmp, "version")
	hideConsole(verify)
	out, err := verify.Output()
	if err != nil || !strings.Contains(string(out), info.Latest) {
		return false, fmt.Errorf("downloaded binary failed verification (%v: %q)", err, out)
	}

	if runtime.GOOS == "windows" {
		// A running exe can't be overwritten, but it can be renamed away.
		old := target + ".old"
		_ = os.Remove(old)
		_ = os.Rename(target, old)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(target+".old", target) // Windows: put the old one back
		return false, err
	}
	// launchd may exec the app-bundle copy (nicer Login Items entry):
	// keep it in sync. Swap in the release's signed bundle whole; only if
	// that fails, patch the binary in and ad-hoc re-sign (which drops the
	// release signature — see swapAppBundle).
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat(appBundleBin); err == nil {
			swapErr := fmt.Errorf("release has no app bundle")
			if info.AppZipURL != "" {
				swapErr = swapAppBundle(info.AppZipURL, info.Latest, appBundlePath())
			}
			if swapErr != nil {
				fmt.Println("bundle swap failed, patching in place:", swapErr)
				if data, err := os.ReadFile(target); err == nil {
					btmp := appBundleBin + ".new"
					if os.WriteFile(btmp, data, 0o755) == nil && os.Rename(btmp, appBundleBin) == nil {
						_ = exec.Command("codesign", "--force", "-s", "-",
							"--identifier", "com.codergangw.claude-notify",
							appBundlePath()).Run()
					}
				}
			}
		}
	}
	fmt.Println("updated to v" + info.Latest)

	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	if restartOK && runtime.GOOS == "windows" && strings.EqualFold(self, target) { // EvalSymlinks normalizes case
		// The running exe now sits at .old; hand off to the new one, whose
		// listen-retry loop waits for this port to free up.
		cmd := exec.Command(target, "daemon")
		detach(cmd)
		if err := cmd.Start(); err == nil {
			_ = cmd.Process.Release()
			go func() {
				time.Sleep(700 * time.Millisecond)
				os.Exit(0)
			}()
			return true, nil
		}
	}
	if restartOK && runtime.GOOS == "darwin" && (self == target || self == appBundleBin) {
		// Hand the relaunch to a detached helper. Reload the launchd job
		// (bootout + bootstrap) rather than kickstart: our exit(0) below
		// satisfies the job's KeepAlive SuccessfulExit=false semaphore,
		// and a satisfied job ignores kickstart on newer macOS — which
		// left the app dead (or relaunched unmanaged via open, losing
		// the daemon-arg log redirection) after an in-app update. A
		// fresh bootstrap resets the semaphore and RunAtLoad starts the
		// new version under launchd; without a plist (autostart off),
		// fall back to the app bundle, then the CLI binary. We exit 0 so
		// launchd doesn't also respawn on top of the helper's instance.
		home, _ := os.UserHomeDir()
		plist := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
		script := fmt.Sprintf(
			"sleep 1; launchctl bootout gui/%d/%s 2>/dev/null; sleep 0.5; "+
				"launchctl bootstrap gui/%d %q 2>/dev/null || open -b %s 2>/dev/null || (%q daemon >/dev/null 2>&1 &)",
			os.Getuid(), launchdLabel, os.Getuid(), plist, launchdLabel, target)
		helper := exec.Command("/bin/sh", "-c", script)
		detach(helper) // must outlive us: launchd kills our process group on exit
		if err := helper.Start(); err == nil {
			go func() {
				time.Sleep(700 * time.Millisecond)
				os.Exit(0)
			}()
			return true, nil
		}
	}
	return false, nil
}

// appBundlePath is the installed bundle, /Applications/agent-notify.app.
func appBundlePath() string {
	return filepath.Dir(filepath.Dir(filepath.Dir(appBundleBin)))
}

// swapAppBundle replaces bundle with the release's signed .app. Taking
// the bundle whole keeps the release signature and its team ID: patching
// the binary inside and ad-hoc re-signing dropped it, so the TCC identity
// shifted every update and Login Items listed "Unknown Developer".
func swapAppBundle(zipURL, want, bundle string) error {
	parent := filepath.Dir(bundle) // stage beside it so the swap is a rename
	zip, err := downloadTo(zipURL, parent)
	if err != nil {
		return err
	}
	defer os.Remove(zip)
	stage, err := os.MkdirTemp(parent, ".agent-notify-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // takes the old bundle with it
	if out, err := exec.Command("ditto", "-x", "-k", zip, stage).CombinedOutput(); err != nil {
		return fmt.Errorf("unzip: %v: %s", err, out)
	}
	fresh := filepath.Join(stage, "agent-notify.app")
	if out, err := exec.Command("codesign", "--verify", "--strict", fresh).CombinedOutput(); err != nil {
		return fmt.Errorf("bundle signature: %v: %s", err, out)
	}
	out, err := exec.Command(filepath.Join(fresh, "Contents", "MacOS", "agent-notify"), "version").Output()
	if err != nil || !strings.Contains(string(out), want) {
		return fmt.Errorf("bundle failed verification (%v: %q)", err, out)
	}
	old := filepath.Join(stage, "old.app")
	if err := os.Rename(bundle, old); err != nil {
		return err
	}
	if err := os.Rename(fresh, bundle); err != nil {
		_ = os.Rename(old, bundle)
		return err
	}
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
