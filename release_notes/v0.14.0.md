<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Session list window: the new list button on each agent tab opens a full-size window with every session that agent left on disk (Claude Code: CLI, VS Code, Desktop, SDK and orphaned leftovers; Codex, Gemini, Antigravity CLI + IDE, opencode, Cursor). opencode and Cursor keep sessions in SQLite, which needs the `sqlite3` command (built into macOS). Filter by running/empty/old/large, source or project, search, multi-select, and delete a session with its checkpoints and prompt history — to the trash by default, or permanently. Running sessions, protected sessions and excluded folders are never deleted; resume a session in the terminal straight from the list.
- Notifications now come only from the agent-notify app itself: the terminal-notifier / AppleScript fallbacks are gone, and while the daemon isn't running hooks stay silent (the daemon is restarted for the next event). On macOS, install.sh now installs agent-notify.app into /Applications (a bare binary can't show notifications).
- AI summaries removed: completion notifications show an excerpt of the session's last reply instead of a Haiku one-liner — no more `claude -p` calls or token use per notification.
- Onboarding tour: after the dashboard steps it continues in the session list window (sidebar, list, details, search, bulk cleanup, delete, settings — on sample sessions), and Skip/Get started there closes the window and brings the dashboard back. The tour also works on a fresh install with no agents picked yet, its sample data follows the UI language, the sub-tab you had open is restored afterwards, and the expand/search step now covers the new list button.
- New app icon: a redesigned bell for the app, dashboard and tray, rendered crisper at every size.

<details><summary>🇰🇷 한국어</summary>

- 세션 목록 창: 각 에이전트 탭의 목록 버튼을 누르면 그 에이전트가 디스크에 남긴 모든 세션을 큰 창으로 보여줍니다 (Claude Code: CLI·VS Code·Desktop·SDK·고아 데이터, Codex, Gemini, Antigravity CLI·IDE, opencode, Cursor). opencode·Cursor는 세션을 SQLite에 저장해 `sqlite3` 명령이 필요합니다(macOS 기본 탑재). 실행 중·빈 세션·오래된·큰 세션, 출처·프로젝트별 필터와 검색, 다중 선택을 지원하고, 세션을 체크포인트·프롬프트 히스토리까지 함께 삭제합니다 — 기본은 휴지통, 원하면 영구 삭제. 실행 중이거나 보호된 세션, 제외 폴더의 세션은 지워지지 않으며, 목록에서 바로 터미널로 세션을 이어갈 수 있습니다.
- 알림은 이제 agent-notify 앱에서만 보냅니다: terminal-notifier·AppleScript 폴백을 없앴고, 데몬이 꺼져 있으면 훅은 알림 없이 다음 이벤트를 위해 데몬만 다시 띄웁니다. macOS에서 install.sh는 이제 agent-notify.app을 /Applications에 설치합니다 (단독 바이너리는 알림을 띄울 수 없음).
- AI 요약 제거: 완료 알림 본문은 Haiku 한 줄 요약 대신 세션 마지막 응답 발췌를 보여줍니다 — 알림마다 `claude -p` 호출·토큰 사용이 없어집니다.
- 온보딩 투어: 대시보드 단계가 끝나면 세션 목록 창으로 이어집니다 (사이드바·목록·상세·검색·일괄 정리·삭제·설정 — 예시 세션으로). 거기서 건너뛰기/시작하기를 누르면 창이 닫히고 대시보드로 돌아옵니다. 에이전트를 아직 고르지 않은 새 설치에서도 투어가 제대로 보이고, 예시 데이터가 UI 언어를 따르며, 투어 후 보던 하위 탭이 복원되고, 확장·검색 단계에 새 목록 버튼 설명이 추가됐습니다.
- 새 앱 아이콘: 앱·대시보드·트레이 아이콘을 새로 디자인했고, 모든 크기에서 더 선명하게 렌더링됩니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- 会话列表窗口：点击各代理标签页的列表按钮，即可在独立大窗口中查看该代理留在磁盘上的所有会话（Claude Code：CLI、VS Code、Desktop、SDK 及孤立数据；Codex、Gemini、Antigravity CLI 与 IDE、opencode、Cursor）。opencode 与 Cursor 将会话存于 SQLite，需要 `sqlite3` 命令（macOS 自带）。支持按运行中/空/旧/大会话、来源或项目筛选、搜索与多选，删除时连同检查点和提示历史一并清理 — 默认移到回收站，也可永久删除。运行中、受保护或位于排除文件夹的会话不会被删除；还可直接从列表在终端中继续会话。
- 通知现在只由 agent-notify 应用本身发送：移除了 terminal-notifier / AppleScript 后备方案，守护进程未运行时钩子不再弹通知（只会重启守护进程以接收下一个事件）。macOS 上 install.sh 现在会将 agent-notify.app 安装到 /Applications（独立二进制无法显示通知）。
- 移除 AI 摘要：完成通知改为显示会话最后一条回复的摘录，而不是 Haiku 一句话摘要 — 不再为每条通知调用 `claude -p` 或消耗 token。
- 新手导览：仪表盘步骤结束后会继续进入会话列表窗口（侧栏、列表、详情、搜索、批量清理、删除、设置 — 使用示例会话），在那里点跳过/开始使用会关闭该窗口并回到仪表盘。尚未选择代理的全新安装也能正常显示导览，示例数据跟随界面语言，导览结束后恢复原先打开的子标签，展开/搜索步骤也加入了新的列表按钮说明。
- 全新应用图标：重新设计了应用、仪表盘和托盘的铃铛图标，各尺寸下都更清晰。

</details>
