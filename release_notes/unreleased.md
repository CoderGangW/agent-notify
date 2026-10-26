<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Codex: on Codex 0.129+ the integration moves from the single `notify` slot to Codex hooks — long turns now notify on Windows too (the old argv payload hit Windows' command-line limit), approval requests raise an "input needed" alert, and the notify slot is handed back to the Codex desktop app. Our hook is recorded as trusted when you install it; older Codex keeps using notify.
- opencode: supports OpenCode 2 — one plugin now loads in both opencode 1.x and 2.x, follows 2.x's new events and usage tables, and also works in the Node-based builds.
- Cursor: on Windows, Claude Code's prompt and tool hooks are registered as HTTP hooks, which Cursor CLI doesn't import — launched from Git Bash, Cursor turned their failure into blocking every prompt and tool call. They also skip a process launch per tool call.

<details><summary>🇰🇷 한국어</summary>

- Codex: Codex 0.129 이상에서는 notify 슬롯 대신 Codex hooks로 연동합니다 — Windows에서도 긴 턴의 알림이 오고(기존 방식은 Windows 명령줄 길이 한계에 걸렸습니다), 승인 요청 시 "입력 필요" 알림이 뜨며, notify 슬롯은 Codex 데스크톱 앱에 돌려줍니다. 설치할 때 우리 훅을 신뢰 목록에 함께 기록하며, 이전 버전 Codex는 계속 notify를 씁니다.
- opencode: OpenCode 2를 지원합니다 — 하나의 플러그인이 opencode 1.x와 2.x 모두에서 로드되고, 2.x의 새 이벤트와 사용량 테이블을 따르며, Node 기반 빌드에서도 동작합니다.
- Cursor: Windows에서 Claude Code의 프롬프트·도구 훅을 Cursor CLI가 가져가지 않는 HTTP 훅으로 등록합니다 — Git Bash에서 실행한 Cursor가 이 훅들의 실패를 모든 프롬프트·도구 호출 차단으로 처리했습니다. 도구 호출마다 프로세스를 띄우지 않아 더 가볍습니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- Codex：在 Codex 0.129 及以上版本改用 Codex hooks 而非 notify 槽位 — Windows 上长对话轮次也能收到通知（旧方式会触及 Windows 命令行长度上限），批准请求会触发"需要输入"提醒，notify 槽位交还给 Codex 桌面应用。安装时会一并将我们的钩子记录为受信任；旧版 Codex 继续使用 notify。
- opencode：支持 OpenCode 2 — 同一插件可在 opencode 1.x 与 2.x 中加载，适配 2.x 的新事件与用量表，也可在基于 Node 的构建中运行。
- Cursor：在 Windows 上将 Claude Code 的提示与工具钩子注册为 Cursor CLI 不会导入的 HTTP 钩子 — 从 Git Bash 启动的 Cursor 曾把这些钩子的失败当作阻止，拒绝所有提示与工具调用。同时每次工具调用不再启动新进程。

</details>
