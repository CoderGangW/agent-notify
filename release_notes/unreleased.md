<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **First-run permission pipeline (macOS)**: the bundled app now asks for everything it needs up front — the notification banner and the Desktop/Documents/Downloads file-access dialogs fire on first launch, so click-to-focus works without a surprise prompt later.
- **File access on the welcome checklist**: a new row shows whether the app can read your project folders (the thing that lets a click focus the exact IDE window instead of opening a new one). Allow fires the folder dialogs; if access is still missing it opens the Full Disk Access pane.
- **Live checklist**: while the welcome screen is open, permission changes made in System Settings tick the checklist within a couple of seconds — no restart or button press needed.
- **Smarter VSCode focus fallback**: when the daemon can't read a session's project path (no file-access grant yet), clicking now just brings the IDE forward instead of spawning an empty new window.

<details><summary>🇰🇷 한국어</summary>

- **첫 실행 권한 파이프라인 (macOS)**: 번들 앱이 첫 실행 때 필요한 권한을 한 번에 요청합니다 — 알림 배너와 데스크탑/문서/다운로드 파일 접근 대화상자가 바로 떠서, 클릭-포커스가 나중에 갑작스런 프롬프트 없이 동작합니다.
- **웰컴 체크리스트에 파일 접근 추가**: 프로젝트 폴더를 읽을 수 있는지(클릭 시 새 창 대신 정확한 IDE 창을 포커스하게 해주는 권한) 보여주는 행이 생겼습니다. 허용을 누르면 폴더 대화상자가 뜨고, 그래도 부족하면 전체 디스크 접근 설정을 열어줍니다.
- **라이브 체크리스트**: 웰컴 화면이 열려 있는 동안 시스템 설정에서 바꾼 권한이 몇 초 안에 체크리스트에 반영됩니다 — 재시작이나 버튼 조작이 필요 없습니다.
- **VSCode 포커스 폴백 개선**: 세션의 프로젝트 경로를 읽을 수 없으면(파일 접근 미허용) 클릭 시 빈 새 창을 띄우는 대신 IDE만 앞으로 가져옵니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **首次启动权限流程 (macOS)**：捆绑应用现在会在首次启动时一次性请求所需权限 — 通知横幅与桌面/文稿/下载文件访问对话框立即弹出，点击聚焦功能不会在之后突然弹窗。
- **欢迎清单新增文件访问**：新增一行显示应用能否读取你的项目文件夹（正是它让点击聚焦到已打开项目的 IDE 窗口而不是新开一个）。点「允许」会弹出文件夹对话框；若仍不足则打开完全磁盘访问设置。
- **实时清单**：欢迎界面打开期间，在系统设置中更改的权限会在几秒内反映到清单上 — 无需重启或按钮操作。
- **VSCode 聚焦回退改进**：当守护进程无法读取会话的项目路径（尚未授权文件访问）时，点击只会把 IDE 带到前台，而不是弹出一个空白新窗口。

</details>
