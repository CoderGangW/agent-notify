<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Windows: the installed binary is always `agent-notify.exe`.** A browser-downloaded release asset (`agent-notify-windows-amd64 (1).exe`) used to be installed under that exact name, so every hook — Claude settings, the Codex `notify` wrapper — pointed at a copy that in-app updates (which write `agent-notify.exe`) never touched. Install now copies to the canonical name, removes stale release-asset copies, and the daemon repoints a Codex desktop wrapper that still names the old copy.

<details><summary>🇰🇷 한국어</summary>

- **Windows: 설치 바이너리 이름을 항상 `agent-notify.exe`로 고정.** 브라우저로 받은 릴리즈 파일(`agent-notify-windows-amd64 (1).exe`)이 그 이름 그대로 설치돼, Claude 설정과 Codex `notify` 래퍼 등 모든 훅이 인앱 업데이트(`agent-notify.exe`에 기록)가 절대 건드리지 않는 사본을 가리키던 문제 수정. 이제 표준 이름으로 복사하고 오래된 릴리즈 파일명 사본을 정리하며, 데몬이 옛 사본을 가리키는 Codex 데스크톱 래퍼도 자동으로 새 경로로 바꿉니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **Windows：安装的可执行文件始终为 `agent-notify.exe`。** 通过浏览器下载的发布文件（`agent-notify-windows-amd64 (1).exe`）过去会以原名安装，导致所有 hook——Claude 设置、Codex `notify` 包装——都指向应用内更新（写入 `agent-notify.exe`）永远不会触及的副本。现在安装会复制为规范名称、清理旧的发布文件名副本，守护进程也会自动把仍指向旧副本的 Codex 桌面包装改为新路径。

</details>
