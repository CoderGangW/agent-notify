<div align="center">

<img src="../assets/logo.png" width="96" alt="agent-notify 徽标"/>

<h1>agent-notify</h1>

<p><b>不再错过任何一个完成的 Claude Code 会话。</b></p>

<p>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest"><img src="https://img.shields.io/github/v/release/CoderGangW/agent-notify?color=8a2be2" alt="latest release"/></a>
  <img src="https://img.shields.io/badge/platforms-macOS%20%7C%20Windows%20%7C%20Linux-blue" alt="platforms"/>
  <img src="https://img.shields.io/badge/Go-Wails%20v3-00ADD8?logo=go&logoColor=white" alt="Go + Wails"/>
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-Source--Available%20ND-orange" alt="license"/></a>
</p>

<p>
  🇺🇸 <a href="../README.md">English</a>&nbsp;&nbsp;·&nbsp;&nbsp;🇰🇷 <a href="README.ko.md">한국어</a>&nbsp;&nbsp;·&nbsp;&nbsp;🇨🇳 <b>简体中文</b>
</p>

</div>

跨平台（macOS / Windows / Linux）托盘应用：同时运行多个 Claude Code 会话时，告诉你**刚刚完成的是哪一个** — 并附带会话、令牌用量与套餐限额仪表盘。

## 功能

**通知**
- ✅ 任务完成（Stop）/ 🔔 需要输入（Notification）时发送原生系统通知
- 标题 = **会话标题**（含 VSCode 扩展生成的标题），正文 = **一句话 AI 摘要**（`claude -p --model haiku`，使用现有认证 · 失败时回退为最后回复的摘录）
- macOS 上点击通知（需 `terminal-notifier`）**聚焦会话所在窗口** — 自动识别 IDE（VSCode / Cursor / Windsurf）或终端

**仪表盘窗口** — 点击托盘图标
- 最近会话事件：状态、标题、AI 摘要，一键聚焦回会话的 IDE/终端
- **令牌用量** — 本地汇总 `~/.claude/projects` 转录（今日 / 近 7 天 / 按模型），去重，里程表式数字动画
- **套餐限额仪表** — 与 `/usage` 命令相同的 5 小时 / 每周使用率，附重置倒计时
- 界面语言：English / 한국어 / 简体中文（自动检测，可在窗口内切换）

**同样支持 Codex**
- `agent-notify install-codex` 一条命令，通过 [Codex CLI](https://github.com/openai/codex) 的 `notify` 钩子接入同样的通知

## 工作原理

```
Claude Code ──(Stop / Notification hook)──▶ agent-notify hook ─┐
Codex CLI  ──(notify)──▶ agent-notify codex-hook ──────────────┤ POST localhost:49517
                                                                ▼
                                            托盘守护进程 ──▶ 系统通知
                                                 │
                                                 └──▶ 仪表盘窗口（会话 · 用量 · 限额）
```

守护进程未运行时，钩子会直接发送系统通知作为回退 — 只装钩子也能收到通知。

## 安装

一行命令 — 下载二进制、注册 Claude Code 钩子、设置开机自启：

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.sh | sh
```

```powershell
# Windows
irm https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.ps1 | iex
```

已安装 Go：

```sh
go install github.com/CoderGangW/agent-notify@latest
agent-notify install   # 注册钩子 + 自启 + 启动守护进程
```

macOS 也可从发布页下载 **agent-notify.app**（已签名、通用二进制）— 双击即可：首次运行会启动守护进程并**自动安装钩子和登录自启**，无需终端。

## 命令

| 命令 | 说明 |
|---|---|
| `agent-notify` | 运行托盘守护进程（默认） |
| `agent-notify install` | 复制二进制到固定路径、注册 Stop/Notification 钩子、开机自启、启动守护进程 |
| `agent-notify install-codex` | 在 `~/.codex/config.toml` 注册 Codex CLI `notify` 钩子 |
| `agent-notify uninstall` | 移除钩子与自启 |
| `agent-notify stats` | 调试：输出窗口所示的用量 + 限额 JSON |
| `agent-notify hook` / `codex-hook` | 由 Claude Code / Codex 调用的钩子端点（勿手动运行） |
| `agent-notify peek <transcript.jsonl> [session-id]` | 调试：查看会话标题/摘要来源 |

## 平台说明

- **macOS**：强烈建议 `brew install terminal-notifier` 以支持点击聚焦。窗口 UI 为原生（Wails v3 / WebKit）。本地构建用 `build/release-macos.sh` 打包签名（bundle id 固定为 `com.codergangw.claude-notify`）。
- **Linux**：窗口 UI 需要发行版的 `libgtk-3` + `libwebkit2gtk-4.1`；托盘需要 `libayatana-appindicator`，通知需要 `notify-send`。缺少它们时仅钩子通知仍可用。安装时注册应用菜单启动器和图标。
- **Windows**：toast 通知，无额外依赖。exe 运行不弹控制台窗口。
- **AI 摘要**：PATH 中有 `claude` CLI 时自动启用。设 `CLAUDE_NOTIFY_NO_AI=1` 可禁用。
- **套餐限额**：通过 `/usage` 命令所用的同一非官方端点、以现有 Claude Code 认证只读查询（绝不刷新或修改令牌）。即使失效也不影响应用其余功能。

## 自定义图标

图标由 `tools/genicon` 从矢量几何生成（托盘 PNG、macOS 包 `.icns`、Windows 多尺寸 `.ico`），构建时嵌入：

```sh
go run ./tools/genicon assets && go build
```

## 路线图

- [x] 点击通知/事件聚焦会话的 IDE 或终端
- [x] 仪表盘窗口：会话、令牌用量、套餐限额
- [x] Codex CLI 支持
- [ ] 聚焦 VSCode 扩展中的特定会话面板（依赖扩展提供深链接 API）
- [ ] Webhook（ntfy / Slack / Telegram / Discord）— 离开电脑时推送到手机
- [ ] 实时会话状态（PreToolUse/PostToolUse：运行中的工具、耗时）
- [ ] 收集远程机器的事件

## 许可证

源码可见许可：可自由使用与再分发未修改副本（含商业用途），修改及分发修改版需书面许可。见 [LICENSE](../LICENSE)。
