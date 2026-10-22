<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Version picker in the tray menu**: right-click → "Install another version" lists recent releases — one click installs any of them, so a misbehaving release can be rolled back on the spot. Same verified pipeline as updates (download → run → version check → swap).
- **Fixed: the app really relaunches after an update now.** launchd kills the daemon's whole process group on exit, which silently killed the relaunch helper before it could restart the app — updates applied but the app stayed closed. The helper (and the restart button's replacement daemon) now runs in its own session and survives.

<details><summary>🇰🇷 한국어</summary>

- **트레이 메뉴 버전 선택기**: 우클릭 → "다른 버전 설치"에서 최근 릴리즈 목록을 보고 클릭 한 번으로 설치 — 문제 있는 릴리즈를 그 자리에서 되돌릴 수 있습니다. 업데이트와 같은 검증 파이프라인(다운로드 → 실행 → 버전 확인 → 교체)을 사용합니다.
- **수정: 업데이트 후 앱이 이제 정말 다시 켜집니다.** launchd가 데몬 종료 시 프로세스 그룹 전체를 종료해, 재실행 헬퍼가 앱을 켜기도 전에 조용히 죽고 있었습니다 — 업데이트는 되는데 앱은 꺼진 채였던 원인. 이제 헬퍼(그리고 재시작 버튼의 대체 데몬)가 자체 세션에서 실행되어 살아남습니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **托盘菜单版本选择器**：右键 → "安装其他版本" 列出最近的发布版本，一键安装任意版本 — 出问题的版本可当场回滚。与更新使用同一套校验流程（下载 → 运行 → 版本校验 → 替换）。
- **修复：更新后应用现在真的会重新启动。** launchd 在守护进程退出时会终止整个进程组，导致重启辅助进程在拉起应用前就被悄悄杀掉 — 这就是更新成功但应用保持关闭的原因。现在辅助进程（以及重启按钮的替代守护进程）在独立会话中运行，不再被波及。

</details>
