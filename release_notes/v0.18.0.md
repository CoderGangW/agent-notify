<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Fixed a second menu bar icon blinking on and off when agent-notify was launched twice at login (LaunchAgent + an "Open at Login" app entry): a duplicate now exits immediately, before it ever shows a tray icon.
- macOS: the "Start at login" toggle now mirrors System Settings — switching agent-notify off under Login Items → Allow in the Background shows the toggle off, and turning it back on in the app takes you to that switch. The settings row says where the entry lives and opens that pane.
- macOS in-app updates now install the release's signed app bundle whole instead of patching the binary inside and re-signing it ad-hoc. The app keeps its signing identity across updates, so macOS permissions stay attached to it and Login Items no longer lists it under "Unknown Developer".
- macOS: the tray menu no longer comes out too short and cuts off its last row when it opens mid-animation.

<details><summary>🇰🇷 한국어</summary>

- 로그인 시 agent-notify가 두 번 실행되면(LaunchAgent + "로그인 시 열기" 앱 항목) 메뉴 막대 아이콘이 하나 더 생겼다 사라지기를 반복하던 문제 수정: 중복 실행은 트레이 아이콘을 띄우기 전에 바로 종료됩니다.
- macOS: "로그인 시 자동 시작" 토글이 시스템 설정과 일치합니다 — 로그인 항목 → 백그라운드에서 허용에서 끄면 토글도 꺼진 상태로 보이고, 앱에서 다시 켜면 해당 스위치 화면으로 안내합니다. 설정 행에 등록 위치 안내와 바로 열기 버튼이 생겼습니다.
- macOS 인앱 업데이트가 번들 안의 바이너리만 바꾸고 ad-hoc으로 다시 서명하던 방식 대신, 릴리즈의 서명된 앱 번들을 통째로 설치합니다. 업데이트 후에도 서명 신원이 유지되어 macOS 권한이 그대로 이어지고, 로그인 항목에 "Unknown Developer"로 표시되지 않습니다.
- macOS: 트레이 메뉴가 열리는 애니메이션 도중 크기가 짧게 잡혀 마지막 줄이 잘리던 문제를 고쳤습니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- 修复登录时 agent-notify 被启动两次（LaunchAgent + "登录时打开"应用项）导致菜单栏多出一个图标反复闪现的问题：重复实例会在显示托盘图标前立即退出。
- macOS："登录时自动启动"开关与系统设置保持一致 — 在 登录项 → 允许在后台 中关闭后开关同步显示为关闭，在应用中重新开启时会引导至该开关。设置行注明了条目位置并可一键打开该面板。
- macOS 应用内更新改为整体安装发行版中已签名的应用包，不再只替换包内二进制并进行 ad-hoc 重签名。更新后签名身份保持不变，macOS 权限得以延续，登录项中也不再显示为 "Unknown Developer"。
- macOS：修复托盘菜单在打开动画过程中尺寸偏小、最后一行被截断的问题。

</details>
