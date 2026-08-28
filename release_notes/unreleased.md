<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Fixed (Windows): a PowerShell console flashed every second while the app ran — the system-locale probe spawned a visible console on each dashboard poll. The locale is now cached and every helper process runs with its console hidden (registry setup, update verification, AI summary too).
- Fixed (Windows): the tray showed a blank icon — the icon was handed to Windows in the wrong format. The tray now shows the app icon.

<details><summary>🇰🇷 한국어</summary>

- 수정 (Windows): 앱 실행 중 PowerShell 콘솔이 1초마다 번쩍이던 문제 — 시스템 로케일 조회가 대시보드 폴링마다 콘솔 창을 띄웠습니다. 이제 로케일을 캐시하고 모든 헬퍼 프로세스(레지스트리 등록, 업데이트 검증, AI 요약 포함)를 콘솔 숨김으로 실행합니다.
- 수정 (Windows): 트레이 아이콘이 비어 보이던 문제 — 아이콘을 잘못된 포맷으로 전달하고 있었습니다. 이제 앱 아이콘이 정상 표시됩니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- 修复（Windows）：应用运行时 PowerShell 控制台每秒闪烁 — 系统区域设置探测在每次仪表盘轮询时都会弹出控制台窗口。现在区域设置已缓存，且所有辅助进程（注册表设置、更新校验、AI 摘要）均以隐藏控制台方式运行。
- 修复（Windows）：托盘图标显示为空白 — 图标以错误的格式传递给了系统。现在托盘会正常显示应用图标。

</details>
