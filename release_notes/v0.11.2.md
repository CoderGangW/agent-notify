<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Codex on Windows: hook setup through notification, end to end.** The Codex desktop app now wraps a user's `notify` command instead of replacing it (`--previous-notify`, openai/codex#28404). agent-notify recognizes that wrapper as "hooked", leaves it alone, and never relays back into it — the previous relay logic could have recursed helper → agent-notify → helper forever. A relay marker in the environment guards the loop even if the app changes shape again. Uninstall strips only our entry from the wrapper.
- Codex events now carry the turn's `thread-id` and `cwd` from the notify payload, so a Windows toast click focuses the right session/window and the event list groups Codex turns correctly.
- Codex desktop app windows are now matched by the Windows focus logic.

<details><summary>🇰🇷 한국어</summary>

- **Windows Codex: 훅 설정부터 알림까지 전 구간 정비.** Codex 데스크톱 앱이 사용자의 `notify` 명령을 교체 대신 `--previous-notify`로 감싸는 동작(openai/codex#28404)을 인식해 "등록됨"으로 처리하고 손대지 않으며, 그 래퍼로 다시 릴레이하지 않습니다 — 기존 릴레이 로직은 helper → agent-notify → helper 무한 재귀 가능성이 있었습니다. 환경변수 마커로 앱 동작이 또 바뀌어도 루프를 차단합니다. 제거 시 래퍼에서 우리 항목만 걷어냅니다.
- Codex 이벤트에 notify payload의 `thread-id`와 `cwd`를 실어, Windows 토스트 클릭이 올바른 세션/창을 포커스하고 이벤트 목록이 Codex 턴을 정확히 묶습니다.
- Windows 포커스 로직이 Codex 데스크톱 앱 창도 매칭합니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **Windows 上的 Codex：从 hook 设置到通知全链路修复。** Codex 桌面应用现在会用 `--previous-notify` 包装用户的 `notify` 命令而非替换（openai/codex#28404）。agent-notify 会将该包装识别为“已注册”、不再改写，也不会向它回转发——此前的转发逻辑可能导致 helper → agent-notify → helper 无限递归。环境变量标记可在应用行为再次变化时阻断循环。卸载时只从包装中移除我们的条目。
- Codex 事件现在携带 notify payload 中的 `thread-id` 与 `cwd`，Windows toast 点击可聚焦正确的会话/窗口，事件列表也能正确归组 Codex 轮次。
- Windows 聚焦逻辑现已匹配 Codex 桌面应用窗口。

</details>
