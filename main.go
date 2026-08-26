// claude-notify: cross-platform tray daemon that surfaces Claude Code
// task-completion and attention events as native OS notifications.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

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
	case "install":
		runInstall()
	case "uninstall":
		runUninstall()
	case "peek": // debug: show what a transcript resolves to
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: claude-notify peek <transcript.jsonl>")
			os.Exit(2)
		}
		title, summary := transcriptInfo(os.Args[2])
		fmt.Printf("title:   %q\nsummary: %q\n", title, summary)
		if len(os.Args) > 3 {
			fmt.Printf("vscode:  %q\nnamed:   %q\n", vscodeTitle(os.Args[3]), sessionName(os.Args[3]))
		}
	case "version", "--version", "-v":
		fmt.Println("claude-notify " + version)
	default:
		fmt.Fprintln(os.Stderr, "usage: claude-notify [daemon|hook|install|uninstall|version]")
		os.Exit(2)
	}
}
