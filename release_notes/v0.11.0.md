<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Codex desktop app now works with one hook registration.** The Codex desktop app claims the single `notify` slot in `~/.codex/config.toml` for its own binary, so registering our hook used to fail silently. Install now saves the app's command and relays every event to it — desktop-app callbacks and agent-notify notifications both stay live. If an app update rewrites the config, the daemon detects it and re-wires the hook automatically; uninstall restores the original `notify` entry.
- Fixed Codex hook install writing `notify` below a `[projects]` table section, where Codex ignores it — the entry now always lands top-level.

<details><summary>🇰🇷 한국어</summary>

- **Codex 데스크톱 앱도 훅 등록 한 번이면 됩니다.** Codex 데스크톱 앱이 `~/.codex/config.toml`의 단일 `notify` 슬롯을 자체 바이너리로 선점해 훅 등록이 조용히 실패하던 문제 해결. 이제 설치 시 앱의 명령을 백업하고 모든 이벤트를 릴레이해 데스크톱 앱 콜백과 agent-notify 알림이 함께 동작합니다. 앱 업데이트로 설정이 덮어써지면 데몬이 감지해 자동 복구하고, 제거 시 원래 `notify` 항목을 복원합니다.
- Codex 훅 설치 시 `notify`가 `[projects]` 테이블 아래에 기록돼 Codex가 무시하던 버그 수정 — 이제 항상 최상위에 기록됩니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **Codex 桌面应用现在只需注册一次 hook。** Codex 桌面应用会用自己的程序占用 `~/.codex/config.toml` 中唯一的 `notify` 槽位，导致 hook 注册静默失败。现在安装时会备份该命令并转发所有事件——桌面应用回调与 agent-notify 通知同时生效。应用更新覆盖配置时守护进程会自动检测并恢复；卸载时还原原有 `notify` 条目。
- 修复 Codex hook 安装时把 `notify` 写到 `[projects]` 表之下导致 Codex 忽略的问题——现在始终写入顶层。

</details>
