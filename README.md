<div align="center">

<img src="assets/logo.png" width="96" alt="agent-notify logo"/>

<h1>agent-notify</h1>

<p><b>Never miss a finished coding-agent session — Claude Code, Codex, Antigravity, Gemini, opencode, Cursor.</b></p>

<p>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest"><img src="https://img.shields.io/github/v/release/CoderGangW/agent-notify?color=8a2be2" alt="latest release"/></a>
  <img src="https://img.shields.io/badge/platforms-macOS%20%7C%20Windows%20%7C%20Linux-blue" alt="platforms"/>
  <img src="https://img.shields.io/badge/Go-Wails%20v3-00ADD8?logo=go&logoColor=white" alt="Go + Wails"/>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Source--Available%20ND-orange" alt="license"/></a>
</p>

<p>
  🇺🇸 <b>English</b>&nbsp;&nbsp;·&nbsp;&nbsp;🇰🇷 <a href="docs/README.ko.md">한국어</a>&nbsp;&nbsp;·&nbsp;&nbsp;🇨🇳 <a href="docs/README.zh-CN.md">简体中文</a>
</p>

<p>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest/download/agent-notify-macos-universal.app.zip"><img src="https://img.shields.io/badge/macOS-Download-111111?style=for-the-badge&logo=apple&logoColor=white" alt="Download for macOS"/></a>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest/download/agent-notify-windows-amd64.exe"><img src="https://img.shields.io/badge/Windows-Download-0a66c2?style=for-the-badge" alt="Download for Windows"/></a>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest/download/agent-notify-linux-amd64"><img src="https://img.shields.io/badge/Linux-Download-f4a028?style=for-the-badge&logo=linux&logoColor=white" alt="Download for Linux"/></a>
</p>

<img src="assets/main_screen_en.png" width="360" alt="agent-notify dashboard"/>

</div>

You run several coding agents at once and lose track of which one just finished, which one is waiting for you, and how much of your plan you've burned. **agent-notify** is a small tray app for macOS, Windows and Linux that answers all three: it notifies you the moment a session finishes or needs input, jumps you back to the exact window it ran in, and shows your sessions, token usage and plan limits in one dashboard.

## What you get

**Know the moment a session is done — or stuck**
- ✅ Native OS notification when a task completes, 🔔 when the agent is waiting for your input or a permission
- The notification tells you *which* session: title = the session's own title (including titles set by the VSCode extension), body = a one-line AI summary of what it did (falls back to an excerpt of the last reply)
- Four notification modes — **On**, **Alerts only**, **Quiet** (badges, no banners), **Silent**

**Jump straight back to it**
- Click a notification, an event, or an active session and the exact window comes to the front — the IDE workspace (VSCode / Cursor / Windsurf), the terminal tab, or the multiplexer pane the session ran in
- Multi-root `.code-workspace` and subfolder sessions focus the window that already shows them instead of opening a new one

**One dashboard for every agent** — click the tray icon
- One tab per agent you choose; each tab has a setup checklist (installed · logged in · hook connected) with one-click fixes
- **Active sessions** with live status — working · waiting for input · idle
- **Session events** feed with unread badges, search and filters
- **Usage statistics** — hourly / daily / monthly token charts (scroll to zoom, drag to pan), per model
- **Plan limit gauges** — the same 5-hour and weekly utilization `/usage` shows, with reset countdowns
- English / 한국어 / 简体中文, light / dark / system theme, start at login, update notifications with one-click install

## Supported agents

| Agent | Task complete | Input needed | Live session status | Usage & limits | Set up with |
|---|:-:|:-:|:-:|---|---|
| **Claude Code** | ✅ | ✅ | ✅ | Token usage + plan limits | `agent-notify install` |
| **Codex CLI** | ✅ | — | — | — | `agent-notify install-codex` |
| **Gemini CLI** (beta) | ✅ | ✅ | ✅ | — | `agent-notify install-gemini` |
| **Antigravity CLI** (beta) | ✅ | — | ✅ | Model quota meter | `agent-notify install-antigravity` |
| **opencode** (beta) | ✅ | ✅ (permission requests) | ✅ | Token & cost usage | `agent-notify install-opencode` |
| **Cursor CLI** (beta) | ✅ | — | ✅ | — | `agent-notify install-cursor` |

Setup commands are one-shot: they add agent-notify to that agent's own hook/plugin config and can be undone with `agent-notify uninstall`. The dashboard's setup checklist runs them for you, so you rarely need the terminal.

## Click-to-focus support

| Where the session ran | What gets focused | Notes |
|---|---|---|
| VSCode / Cursor / Windsurf | the exact workspace window | multi-root workspaces and subfolders supported |
| tmux | the exact pane | switches window, pane and client |
| WezTerm | the exact pane | |
| iTerm2 | the exact session | |
| kitty | the window | needs `allow_remote_control` |
| GNU screen | the window | |
| Zellij | the pane (best effort) | needs a zellij with `focus-pane-with-id` |
| [cmux](https://cmux.com) | workspace + pane | set Settings → Automation → socket access to password/allowAll |
| any other terminal | the app window | |

Precision by platform: **macOS** — everything above. **Windows** — brings the session's window to the front. **Linux** — opens the session's project folder.

## Install

One line downloads the app, connects it to Claude Code and sets it to start at login:

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.sh | sh
```

```powershell
# Windows
irm https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.ps1 | iex
```

**macOS, no terminal:** download **agent-notify.app** (signed, universal) from the buttons above and open it — first launch connects Claude Code and registers login autostart by itself, then asks for the notification and folder-access permissions it needs.

With Go: `go install github.com/CoderGangW/agent-notify@latest && agent-notify install`.

Then add other agents from the dashboard's agent picker, or with the `install-*` commands in the table above. `agent-notify uninstall` removes every hook and the autostart entry.

## Platform notes

- **macOS** — native notifications with the app's own icon, no extra tools. First launch asks for notification and Desktop/Documents/Downloads access (the latter is what lets a click focus the exact IDE window).
- **Windows** — toast notifications, no extra dependencies, no console window.
- **Linux** — notifications via `notify-send`; the dashboard window needs `libgtk-3` and `libwebkit2gtk-4.1`, the tray icon needs `libayatana-appindicator`. Without them you still get hook-driven notifications.
- **AI summaries** use your existing `claude` CLI login (Haiku, a few tokens per notification). Turn them off in Settings or with `CLAUDE_NOTIFY_NO_AI=1`.

## Privacy

Everything stays on your machine. Token usage is computed from your local transcripts; plan limits are read with the credentials Claude Code already stores, read-only — nothing is refreshed, modified or uploaded. The only network calls are the ones you can see: the limit check, the optional AI summary, and the release check for updates.

## Roadmap

- [x] Click a notification / event to focus the session's IDE or terminal
- [x] Dashboard: sessions, token usage, plan limits
- [x] Codex, Gemini, Antigravity, opencode, Cursor
- [ ] Focus the exact session panel inside the VSCode extension (blocked on a deep-link API from the extension)
- [ ] Webhooks (ntfy / Slack / Telegram / Discord) for away-from-keyboard alerts
- [ ] Collect events from remote machines

## Release notes

Every release's changes live in [release_notes/](release_notes/) and on the [releases page](https://github.com/CoderGangW/agent-notify/releases).

## License

Source-available: you may use and redistribute unmodified copies freely (commercial use included), but creating or distributing modified versions requires written permission. See [LICENSE](LICENSE).
