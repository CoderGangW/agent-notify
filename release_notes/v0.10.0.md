<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Click-to-focus for VSCode workspaces**: a session inside a multi-root `.code-workspace` (or in a subfolder of an open folder) now focuses the window that already shows it instead of spawning a new VSCode window. The daemon reads the IDE's live window list and hands `open` the window's real root — the `.code-workspace` file, or the top folder.

<details><summary>🇰🇷 한국어</summary>

- **VSCode 워크스페이스 클릭-포커스**: 멀티루트 `.code-workspace`(또는 열린 폴더의 하위 경로) 안에서 도는 세션을 클릭하면 새 VSCode 창이 뜨는 대신 이미 그 프로젝트를 보여주는 창이 포커스됩니다. 데몬이 IDE의 실시간 창 목록을 읽어 창의 실제 루트(`.code-workspace` 파일 또는 최상위 폴더)를 `open`에 넘깁니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **VSCode 工作区点击聚焦**：在多根 `.code-workspace`（或已打开文件夹的子目录）中运行的会话，点击时会聚焦已显示该项目的窗口，而不再新开一个 VSCode 窗口。守护进程读取 IDE 的实时窗口列表，把窗口的真实根（`.code-workspace` 文件或顶层文件夹）交给 `open`。

</details>
