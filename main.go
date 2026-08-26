// claude-notify: cross-platform tray daemon that surfaces Claude Code
// task-completion and attention events as native OS notifications.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const version = "0.2.0"

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
	case "uninstall":
		runUninstall()
	case "peek": // debug: show what a transcript resolves to
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: claude-notify peek <transcript.jsonl>")
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
	case "version", "--version", "-v":
		fmt.Println("claude-notify " + version)
	default:
		fmt.Fprintln(os.Stderr, "usage: claude-notify [daemon|hook|install|uninstall|version]")
		os.Exit(2)
	}
}
