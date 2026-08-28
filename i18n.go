package main

import (
	"embed"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// UI languages: en (default), ko, zh. All strings live in i18n/*.json —
// one file per locale, shared with the window frontend (the daemon serves
// the same embedded files over /i18n/). Resolution order: config.json
// "lang" override, then system locale.

//go:embed i18n/*.json
var i18nFS embed.FS

var (
	messagesOnce sync.Once
	messages     map[string]map[string]string
)

func loadMessages() {
	messagesOnce.Do(func() {
		messages = map[string]map[string]string{}
		for _, l := range []string{"en", "ko", "zh"} {
			data, err := i18nFS.ReadFile("i18n/" + l + ".json")
			if err != nil {
				continue
			}
			var m map[string]string
			if json.Unmarshal(data, &m) == nil {
				messages[l] = m
			}
		}
	})
}

var (
	langMu   sync.Mutex
	langOnce sync.Once
	curLang  string
)

// T returns the message for key in the current language, falling back to
// English, then to the key itself.
func T(key string) string {
	loadMessages()
	langMu.Lock()
	lang := curLang
	langMu.Unlock()
	if lang == "" {
		langOnce.Do(func() {
			setLang(resolveLang(loadConfig().Lang))
		})
		langMu.Lock()
		lang = curLang
		langMu.Unlock()
	}
	if m := messages[lang]; m != nil && m[key] != "" {
		return m[key]
	}
	if m := messages["en"][key]; m != "" {
		return m
	}
	return key
}

func setLang(lang string) {
	langMu.Lock()
	curLang = lang
	langMu.Unlock()
}

// resolveLang maps a config setting ("", "auto", "en", "ko", "zh") to a
// supported language, consulting the system locale for auto.
func resolveLang(setting string) string {
	loadMessages()
	setting = strings.ToLower(strings.TrimSpace(setting))
	if messages[setting] != nil {
		return setting
	}
	return matchLang(systemLocale())
}

func matchLang(locale string) string {
	l := strings.ToLower(locale)
	switch {
	case strings.HasPrefix(l, "ko"):
		return "ko"
	case strings.HasPrefix(l, "zh"):
		return "zh"
	default:
		return "en"
	}
}

var (
	localeOnce sync.Once
	localeVal  = "en"
)

// systemLocale is cached: it's consulted on every /api/state poll, and
// the fallback probes below spawn a process (the Windows one flashed a
// PowerShell console every second before the cache).
func systemLocale() string {
	localeOnce.Do(func() {
		for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
			if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
				localeVal = v
				return
			}
		}
		switch runtime.GOOS {
		case "darwin":
			// launchd agents and Finder-launched apps carry no locale env.
			if out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output(); err == nil {
				localeVal = strings.TrimSpace(string(out))
			}
		case "windows":
			cmd := exec.Command("powershell", "-NoProfile", "-Command", "(Get-Culture).Name")
			hideConsole(cmd)
			if out, err := cmd.Output(); err == nil {
				localeVal = strings.TrimSpace(string(out))
			}
		}
	})
	return localeVal
}
