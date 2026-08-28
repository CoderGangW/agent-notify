<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- opencode tab now shows token usage and recent sessions: the notify plugin aggregates the local `opencode.db` (per-day/per-model tokens, cost, latest sessions) and the window renders a usage card just like Claude's — cached, so it still shows after opencode exits
- Windows: clicking a notification now focuses the window the session ran in, like on macOS — toasts activate an `agent-notify:` URL protocol and the daemon raises the session's window via win32
- The tray right-click menu is now the app's own themed menu on every platform (native menu item icons stopped rendering on recent macOS) — same look in light/dark, localized, live update status

<details><summary>🇰🇷 한국어</summary>

- opencode 탭에 토큰 사용량과 최근 세션 표시: 알림 플러그인이 로컬 `opencode.db`를 집계(일별/모델별 토큰, 비용, 최근 세션)해 Claude 탭과 같은 사용량 카드로 렌더링 — 캐시되므로 opencode 종료 후에도 유지
- Windows: 알림 클릭 시 macOS처럼 세션이 실행됐던 창으로 포커스 이동 — 토스트가 `agent-notify:` URL 프로토콜을 활성화하고 데몬이 win32로 해당 창을 올립니다
- 트레이 우클릭 메뉴를 모든 플랫폼에서 앱 자체 테마 메뉴로 교체 (최신 macOS에서 네이티브 메뉴 아이콘이 렌더링되지 않는 문제) — 라이트/다크 동일한 디자인, 현지화, 실시간 업데이트 상태 표시

</details>

<details><summary>🇨🇳 简体中文</summary>

- opencode 标签页新增令牌用量与最近会话：通知插件汇总本地 `opencode.db`（按日/按模型令牌、费用、最近会话），以与 Claude 相同的用量卡片展示 — 数据有缓存，opencode 退出后仍可查看
- Windows：点击通知现在会像 macOS 一样聚焦会话所在的窗口 — 通知激活 `agent-notify:` URL 协议，守护进程通过 win32 提升对应窗口
- 托盘右键菜单在所有平台改为应用自绘的主题菜单（新版 macOS 上原生菜单图标无法渲染）— 明暗主题外观一致、已本地化、实时显示更新状态

</details>
