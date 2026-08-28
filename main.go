// agent-notify: cross-platform tray daemon that surfaces Claude Code
// task-completion and attention events as native OS notifications.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const version = "0.7.0"

func main() {
	cmd := "daemon"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "daemon":
		runDaemon()
	case "hook":
		runHook()
	case "summarize-notify": // detached child spawned by `hook`
		runSummarizeNotify()
	case "install":
		runInstall()
	case "install-codex":
		runInstallCodex()
	case "codex-hook": // called by Codex CLI's notify setting
		runCodexHook()
	case "gemini-hook": // called by Gemini CLI's hooks
		runGeminiHook()
	case "cursor-hook": // called by Cursor CLI's hooks
		runCursorHook()
	case "antigravity-hook": // called by Antigravity CLI's hooks
		runAntigravityHook()
	case "install-antigravity":
		if err := installAntigravityHook(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf(T("install.agentHooks")+"\n", "Antigravity", antigravityHooksPath())
	case "install-gemini":
		if err := installGeminiHook(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf(T("install.agentHooks")+"\n", "Gemini", geminiSettingsPath())
	case "install-opencode":
		if err := installOpencodeHook(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf(T("install.opencodePlugin")+"\n", opencodePluginPath())
	case "install-cursor":
		if err := installCursorHook(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf(T("install.agentHooks")+"\n", "Cursor", cursorHooksPath())
	case "uninstall":
		runUninstall()
	case "peek": // debug: show what a transcript resolves to
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: agent-notify peek <transcript.jsonl>")
			os.Exit(2)
		}
		info := transcriptInfo(os.Args[2])
		fmt.Printf("title:   %q\nrequest: %q\nreport:  %q\n", info.Title, condense(info.LastUser, 120), condense(info.LastAssistant, 180))
		if len(os.Args) > 3 {
			vs, bundle := vscodeTitle(os.Args[3])
			fmt.Printf("vscode:  %q (%s)\nnamed:   %q\n", vs, bundle, sessionName(os.Args[3]))
		}
	case "stats": // debug: dump usage + limits JSON
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(usage.report())
		_ = enc.Encode(limits.report())
	case "show": // open the dashboard window of the running daemon
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/show", daemonPort), "", nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "daemon not running")
			os.Exit(1)
		}
		resp.Body.Close()
	case "version", "--version", "-v":
		fmt.Println("agent-notify " + version)
	default:
		fmt.Fprintln(os.Stderr, "usage: agent-notify [daemon|hook|install|install-codex|uninstall|stats|version]")
		os.Exit(2)
	}
}
