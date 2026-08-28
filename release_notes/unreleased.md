<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- New agents: Antigravity CLI (`agy`), Gemini CLI, opencode, and Cursor CLI join Claude Code and Codex — each gets its own tab with completion/attention notifications and live session tracking (as far as each CLI's hook system allows). Gemini CLI stays supported for API-key/Vertex/enterprise auth (Google ended personal-account access on 2026-06-18 in favor of Antigravity). Tabs for agents you haven't installed tuck behind a "+" button.
- Guided setup per agent: if a CLI isn't installed the tab shows the install command; if it's installed but not logged in, one click opens the login in your terminal; registering the notify hook is one click too.
- Per-model plan limits: the usage endpoint's new `limits` array is now parsed, so model-scoped weekly buckets (e.g. "Weekly · Fable") show up alongside the session and weekly-all gauges — same data the Claude desktop tray shows.
- Tray right-click menu: app info (version, last-updated date), an in-place update check that turns into a one-click install when a release is found, restart, and quit — the dashboard stays on left click.
- Antigravity model-quota card: per-model gauges from `agy`'s local credentials, with a note when Claude and GPT share one pooled quota.
- Expandable window with search: the expand button grows the window (natively animated) for long lists, and a search field filters sessions and events by title, content, or project — with quick chips for "Unread" and "Tool running".
- Notification modes: the bell now cycles On → Alerts only → Quiet → Silent, controlling OS notifications and unread badges separately (also in settings).
- The welcome checklist now checks macOS notification and Automation permissions with one-click Allow buttons.

<details><summary>🇰🇷 한국어</summary>

- 에이전트 추가: Claude Code·Codex에 이어 Antigravity CLI(`agy`), Gemini CLI, opencode, Cursor CLI를 지원합니다 — 에이전트별 탭에서 완료/입력요청 알림과 실시간 세션 추적을 제공합니다 (각 CLI 훅 시스템이 허용하는 범위까지). Gemini CLI는 API 키·Vertex·엔터프라이즈 인증으로 계속 지원됩니다 (개인 계정은 2026-06-18부로 Google이 Antigravity로 이관). 미설치 에이전트 탭은 "+" 버튼 뒤로 접힙니다.
- 에이전트별 셋업 가이드: CLI가 없으면 설치 명령을 보여주고, 설치됐지만 로그인 전이면 클릭 한 번으로 터미널 로그인을 열어주며, 알림 훅 등록도 클릭 한 번입니다.
- 모델별 플랜 한도: usage 엔드포인트의 새 `limits` 배열을 파싱해 모델 범위 주간 버킷(예: "주간 · Fable")이 세션·주간 전체 게이지와 함께 표시됩니다 — Claude 데스크톱 트레이와 같은 데이터입니다.
- 트레이 우클릭 메뉴: 앱 정보(버전·마지막 업데이트 날짜), 확인 후 원클릭 설치로 이어지는 업데이트 확인, 재시작, 종료 — 대시보드는 기존처럼 좌클릭입니다.
- Antigravity 모델 쿼터 카드: `agy` 로컬 자격증명으로 모델별 게이지를 표시하고, Claude·GPT가 쿼터를 공유하면 안내를 함께 보여줍니다.
- 창 확장 + 검색: 확장 버튼으로 창이 (네이티브 애니메이션으로) 커지고, 검색창에서 제목·내용·프로젝트로 세션과 이벤트를 필터링합니다 — "안 읽음", "도구 실행 중" 퀵 칩 포함.
- 알림 모드: 종 버튼이 켜짐 → 알림만 → 조용히 → 무음 순서로 순환하며 OS 알림과 안 읽음 뱃지를 각각 제어합니다 (설정에서도 변경 가능).
- 웰컴 체크리스트가 macOS 알림·자동화 권한을 확인하고 원클릭 허용 버튼을 제공합니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- 新增代理：在 Claude Code、Codex 之外支持 Antigravity CLI（`agy`）、Gemini CLI、opencode 和 Cursor CLI — 每个代理有独立标签页，提供完成/需要输入通知与实时会话跟踪（以各 CLI 钩子系统支持的范围为准）。Gemini CLI 继续支持 API 密钥/Vertex/企业认证（个人账户已于 2026-06-18 被 Google 迁移至 Antigravity）。未安装的代理标签收纳在"+"按钮后。
- 按代理的引导设置：未安装 CLI 时显示安装命令；已安装未登录时一键在终端打开登录；注册通知钩子也只需一键。
- 按模型的套餐用量：解析 usage 端点新的 `limits` 数组，模型级每周额度（如 "每周 · Fable"）与会话、每周全部一同显示 — 与 Claude 桌面托盘同源数据。
- 托盘右键菜单：应用信息（版本、最近更新日期）、检查更新（发现新版本后一键安装）、重启、退出 — 仪表盘仍为左键打开。
- Antigravity 模型配额卡片：通过 `agy` 本地凭据显示每个模型的配额仪表，当 Claude 与 GPT 共享配额池时会附带说明。
- 可展开窗口与搜索：展开按钮以原生动画放大窗口，搜索框可按标题、内容或项目筛选会话与事件 — 并提供"未读"、"工具运行中"快捷筛选。
- 通知模式：铃铛按钮按 开启 → 仅提醒 → 安静 → 静音 循环，分别控制系统通知与未读徽章（也可在设置中更改）。
- 欢迎清单现在会检查 macOS 通知与自动化权限，并提供一键允许按钮。

</details>
