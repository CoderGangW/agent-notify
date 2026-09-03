<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Fixed (Windows): the Codex desktop app's helper window strobed open/closed endlessly. The running app watches `config.toml` and reasserts its notify command, and our hook repair took it back every 15 seconds — each rewrite made the app bounce its helper. Repair now backs off (at most every 10 minutes, giving up after 3 rounds while the app is actively fighting), and relayed app commands run with their console hidden.

<details><summary>🇰🇷 한국어</summary>

- 수정 (Windows): Codex 데스크톱 앱 헬퍼 창이 끝없이 켜졌다 꺼졌다 반복하던 문제. 실행 중인 앱이 `config.toml`을 감시하며 자기 notify 명령을 되돌리는데, 우리 훅 복구가 15초마다 슬롯을 다시 가져가면서 매번 앱이 헬퍼를 재시작했습니다. 이제 복구가 백오프하고(최대 10분에 한 번, 앱이 계속 되뺏으면 3회 후 중단), 릴레이되는 앱 명령은 콘솔 숨김으로 실행됩니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- 修复（Windows）：Codex 桌面应用的辅助窗口无限闪烁开关。运行中的应用会监视 `config.toml` 并重新写回自己的 notify 命令，而我们的钩子修复每 15 秒就把槽位夺回来 — 每次改写都让应用重启辅助程序。现在修复采用退避策略（最多每 10 分钟一次，应用持续争夺时 3 轮后放弃），转发的应用命令也以隐藏控制台方式运行。

</details>
