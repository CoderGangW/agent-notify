# claude-notify

[English](../README.md) | [한국어](README.ko.md) | **简体中文**

跨平台（macOS / Windows / Linux）托盘守护程序：同时运行多个 Claude Code 会话时，**立刻告诉你哪个任务完成了**。

不用再挨个终端去确认：

- ✅ 任务完成（Stop）/ 🔔 需要输入（Notification）时发送**系统原生通知**
- 通知标题 = **会话标题**（包括 VSCode 扩展生成的标题），副标题 = **项目路径**，正文 = **AI 一句话总结**（`claude -p --model haiku`，直接使用你现有的认证 · 失败时回退为最后回复的摘录）
- macOS 上安装 `terminal-notifier` 后，**点击通知即可聚焦对应会话的 IDE**（自动识别 VSCode / Cursor / Windsurf）
- 托盘菜单显示最近事件，点击可打开对应项目文件夹
- macOS 菜单栏显示已完成任务数角标

## 工作原理

```
Claude Code ──(Stop / Notification hook)──▶ claude-notify hook
                                                │  POST localhost:49517
                                                ▼
                                          托盘守护程序 ──▶ 系统通知 + 事件列表
```

守护程序未运行时，hook 会回退为直接发送系统通知，因此只注册 hook 也能收到通知。

## 安装

一行命令 — 下载二进制、注册 Claude Code hook、设置开机自启：

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/CoderGangW/claude-notify/main/install.sh | sh
```

```powershell
# Windows
irm https://raw.githubusercontent.com/CoderGangW/claude-notify/main/install.ps1 | iex
```

已安装 Go：

```sh
go install github.com/CoderGangW/claude-notify@latest
claude-notify install   # 注册 hook + 自启 + 启动守护程序
```

源码构建：

```sh
git clone https://github.com/CoderGangW/claude-notify
cd claude-notify
go build -o claude-notify .
```

## 命令

| 命令 | 说明 |
|---|---|
| `claude-notify` | 运行托盘守护程序（默认） |
| `claude-notify install` | 注册 Stop/Notification hook + 登录自启（LaunchAgent / XDG autostart / 注册表）+ 启动守护程序 |
| `claude-notify uninstall` | 移除 hook 和自启注册 |
| `claude-notify hook` | 由 Claude Code 调用的 hook 入口（无需手动执行） |
| `claude-notify peek <transcript.jsonl> [session-id]` | 调试：打印该会话解析出的标题/摘要来源 |

## 自定义图标

托盘图标在构建时嵌入。将 `assets/icon.png`（Windows/Linux，彩色）和 `assets/icon_mac.png`（macOS 菜单栏，单色模板）替换为任意 32×32 PNG 后执行 `go build`。

## 平台说明

- **macOS**：强烈建议 `brew install terminal-notifier` — 支持点击聚焦 IDE 和路径副标题。首次使用需在 系统设置 → 通知 中允许 terminal-notifier。未安装时回退为 `osascript`（通知显示为 "Script Editor"，无点击动作）。通知发送者显示为 terminal-notifier（自定义发送者名称需要 .app 包 — 见路线图）。
- **AI 总结**：`claude` CLI 在 PATH 中时自动启用（通知会因生成总结晚到几秒）。设置环境变量 `CLAUDE_NOTIFY_NO_AI=1` 可关闭。总结子会话的 hook 递归由 `CLAUDE_NOTIFY_SUPPRESS` 防护阻断。
- **Linux**：托盘需要 `libayatana-appindicator`，通知需要 `notify-send`（libnotify）。
- **Windows**：使用 toast 通知，无额外依赖。

## 路线图

- [x] 点击通知聚焦对应 IDE（macOS + terminal-notifier）
- [ ] 自有 .app 包 + UNUserNotificationCenter — 通知发送者名称/图标显示为 claude-notify
- [ ] localhost 仪表盘（一眼看清所有运行中会话）
- [ ] 收集远程机器的会话事件

## License

MIT
