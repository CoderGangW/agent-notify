package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// UI languages: en (default), ko, zh. Resolution order: config.json "lang"
// override, then system locale. The window frontend keeps its own dict in
// app.js — keep both in sync when adding keys.

var messages = map[string]map[string]string{
	"en": {
		"notif.done":            "Task complete",
		"notif.attention":       "Input needed",
		"limits.nocreds":        "Claude credentials not found",
		"limits.network":        "Network error",
		"limits.expired":        "Token expired — run Claude Code to refresh",
		"install.binary":        "Installed binary: %s",
		"install.backup":        "Backup saved: %s",
		"install.hooks":         "Hooks registered (new %d, repointed %d): %s",
		"install.command":       "Registered command: %s",
		"install.already":       "Hooks already installed: %s",
		"install.autostartFail": "Autostart registration failed (start manually): %v",
		"install.autostartOK":   "Autostart at login registered (daemon started)",
		"uninstall.autostart":   "Autostart removed",
		"uninstall.none":        "No hooks registered",
		"uninstall.removed":     "Removed %d hook(s): %s",
		"codex.installed":       "Codex notify hook registered: %s",
		"codex.already":         "%s already has a notify setting; set it manually to:",
	},
	"ko": {
		"notif.done":            "작업 완료",
		"notif.attention":       "입력 필요",
		"limits.nocreds":        "Claude 인증 정보를 찾을 수 없음",
		"limits.network":        "네트워크 오류",
		"limits.expired":        "토큰 만료 — Claude Code 실행하면 갱신됨",
		"install.binary":        "바이너리 설치: %s",
		"install.backup":        "백업 저장: %s",
		"install.hooks":         "hook 등록 완료 (신규 %d, 경로 갱신 %d): %s",
		"install.command":       "등록된 명령: %s",
		"install.already":       "hook 이미 설치되어 있음: %s",
		"install.autostartFail": "자동 시작 등록 실패 (수동 실행 필요): %v",
		"install.autostartOK":   "로그인 시 자동 시작 등록 완료 (데몬 지금 시작됨)",
		"uninstall.autostart":   "자동 시작 등록 제거 완료",
		"uninstall.none":        "등록된 hook 없음",
		"uninstall.removed":     "hook %d개 제거 완료: %s",
		"codex.installed":       "Codex notify hook 등록 완료: %s",
		"codex.already":         "%s에 이미 notify 설정 있음. 수동으로 이렇게 설정:",
	},
	"zh": {
		"notif.done":            "任务完成",
		"notif.attention":       "需要输入",
		"limits.nocreds":        "未找到 Claude 凭据",
		"limits.network":        "网络错误",
		"limits.expired":        "令牌已过期 — 运行 Claude Code 即可刷新",
		"install.binary":        "已安装二进制文件：%s",
		"install.backup":        "已保存备份：%s",
		"install.hooks":         "hook 注册完成（新增 %d，路径更新 %d）：%s",
		"install.command":       "已注册命令：%s",
		"install.already":       "hook 已安装：%s",
		"install.autostartFail": "开机自启注册失败（需手动启动）：%v",
		"install.autostartOK":   "已注册登录自启（守护进程已启动）",
		"uninstall.autostart":   "已移除开机自启",
		"uninstall.none":        "未注册任何 hook",
		"uninstall.removed":     "已移除 %d 个 hook：%s",
		"codex.installed":       "Codex notify hook 已注册：%s",
		"codex.already":         "%s 已有 notify 设置，请手动改为：",
	},
}

var (
	langMu   sync.Mutex
	langOnce sync.Once
	curLang  string
)

// T returns the message for key in the current language, falling back to
// English, then to the key itself.
func T(key string) string {
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

func systemLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
			return v
		}
	}
	switch runtime.GOOS {
	case "darwin":
		// launchd agents and Finder-launched apps carry no locale env.
		if out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	case "windows":
		if out, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-Culture).Name").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "en"
}
