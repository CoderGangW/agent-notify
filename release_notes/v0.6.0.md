<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

### Native macOS notifications

- Notifications are now posted by the app itself (UNUserNotificationCenter): they carry the app's own icon and need **no extra tools** — `terminal-notifier` is only a fallback when the daemon is down.
- Clicking a notification focuses the **exact window** the session ran in, same as clicking a row in the dashboard — IDE, terminal tab, or multiplexer pane.

### Ghostty support

- Click-to-focus now lands on the **exact Ghostty terminal** the session ran in (captured via Ghostty's AppleScript API at prompt time, with a working-directory fallback).

### Live session detail

- Each running tool shows a **per-tool icon** (shell, file edit, read, web, search, agent) plus a one-line hint of what it's doing — the command, file name, or search pattern.
- The "working" state got a sparkles glyph, and the tour's icon legend was updated to match.

<details><summary>🇰🇷 한국어</summary>

### 네이티브 macOS 알림

- 이제 앱이 직접 알림을 보냅니다(UNUserNotificationCenter). 앱 고유 아이콘이 표시되고 **추가 도구가 필요 없습니다** — `terminal-notifier`는 데몬이 꺼져 있을 때의 폴백으로만 쓰입니다.
- 알림을 클릭하면 대시보드 행 클릭과 동일하게 세션이 실행됐던 **정확한 창**(IDE, 터미널 탭, 멀티플렉서 페인)으로 이동합니다.

### Ghostty 지원

- 클릭 포커스가 세션이 실행된 **정확한 Ghostty 터미널**로 이동합니다(프롬프트 입력 시점에 AppleScript API로 캡처, 작업 디렉터리 폴백 포함).

### 실시간 세션 상세

- 실행 중인 도구마다 **도구별 아이콘**(셸, 파일 편집, 읽기, 웹, 검색, 에이전트)과 무엇을 하는지 한 줄 힌트(명령어, 파일명, 검색 패턴)가 표시됩니다.
- "작업 중" 상태에 반짝임 아이콘이 적용됐고, 투어의 아이콘 설명도 업데이트됐습니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

### 原生 macOS 通知

- 现在由应用自身发送通知（UNUserNotificationCenter）：显示应用自己的图标，**无需额外工具** — `terminal-notifier` 仅在守护进程未运行时作为回退。
- 点击通知会聚焦会话运行时所在的**确切窗口**（IDE、终端标签页或多路复用器窗格），与点击仪表盘行为一致。

### Ghostty 支持

- 点击聚焦现在会跳转到会话所在的**确切 Ghostty 终端**（在提交提示词时通过 AppleScript API 捕获，并以工作目录作为回退）。

### 实时会话详情

- 每个运行中的工具都会显示**专属图标**（终端、文件编辑、读取、网络、搜索、代理）以及一行提示（命令、文件名或搜索模式）。
- "工作中"状态改用闪光图标，教程中的图标说明也已同步更新。

</details>
