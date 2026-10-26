<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Windows: clicking a notification now actually focuses the session's window — the toast's callback was routed to a path the CLI could never reach, so clicks did nothing since v0.8.0.
- Security: the local port hooks talk to now refuses requests from web browsers, so a web page can no longer post fake notifications or pop the window open.
- Windows — Claude Code: on PCs without Git for Windows, our hooks ran through PowerShell and failed to parse; since Claude Code 2.1.299 that blocked every tool call. Hooks are now registered in exec form (no shell) and existing installs migrate automatically.
- Windows — Gemini, Cursor, Antigravity: hook commands broke on user profiles with a space (C:\Users\John Smith). Gemini/Cursor now get a PowerShell-safe command and Antigravity a cmd-safe path; existing entries are repaired automatically. Hook payloads with a UTF-8 BOM (Windows PowerShell 5.1) are now read correctly.
- Windows — Cursor: the prompt hook is no longer registered, since Cursor launched from Git Bash treated its crash as "block" and refused every prompt.
- Windows — Antigravity: the quota meter now reads agy's token from Credential Manager, and error stops are announced again.
- Windows — setup: a downloaded exe now registers start-at-login on first run, `install` starts the daemon right away, re-installing over a running copy works, and in-app updates restart into the new version.
- Windows — fewer stray console windows, agents installed after login are detected without restarting the app, install hints show the Windows installers, and an old Claude Desktop alias no longer opens the desktop app.
- The daemon moved off port 49517, which sits in the range Windows' Hyper-V/WSL2/Docker reserve at random — on those PCs the app sometimes couldn't start.
- Claude Code: the done notification now shows the turn's actual final reply — the transcript is written asynchronously and sometimes still held the previous one.
- Codex: a BOM-prefixed config.toml no longer gets a duplicate notify line, CODEX_HOME is honored, and a previous notify command given by name keeps being relayed.
- opencode: notifications carry the session title (which also lets a Windows click find the terminal), and on Windows a folder you excluded in the session list now also protects opencode sessions stored with forward slashes.

<details><summary>🇰🇷 한국어</summary>

- Windows: 알림을 클릭하면 이제 실제로 세션 창이 포커스됩니다 — 토스트 콜백이 CLI가 닿을 수 없는 경로로 가고 있어 v0.8.0부터 클릭해도 아무 일도 없었습니다.
- 보안: 훅이 쓰는 로컬 포트가 이제 웹 브라우저 요청을 거부합니다. 웹페이지가 가짜 알림을 띄우거나 창을 여는 일이 더 이상 불가능합니다.
- Windows — Claude Code: Git for Windows가 없는 PC에서 훅이 PowerShell로 실행되며 파싱 오류가 났고, Claude Code 2.1.299부터는 이 때문에 모든 도구 호출이 차단됐습니다. 이제 셸 없이 실행되는 exec 형식으로 등록하며 기존 설치는 자동으로 전환됩니다.
- Windows — Gemini·Cursor·Antigravity: 사용자 폴더 이름에 공백이 있으면(C:\Users\John Smith) 훅이 실패했습니다. Gemini·Cursor는 PowerShell용, Antigravity는 cmd용 안전한 명령으로 바꾸고 기존 항목은 자동 복구합니다. Windows PowerShell 5.1이 붙이는 BOM이 있어도 훅 데이터를 제대로 읽습니다.
- Windows — Cursor: Git Bash에서 실행한 Cursor가 훅 오류를 "차단"으로 처리해 모든 프롬프트를 거부하던 문제로, 프롬프트 훅을 더 이상 등록하지 않습니다.
- Windows — Antigravity: 쿼터 미터가 자격 증명 관리자에서 agy 토큰을 읽고, 오류로 멈춘 경우도 다시 알립니다.
- Windows — 설치: 내려받은 exe를 처음 실행하면 로그인 시 자동 시작이 등록되고, `install` 직후 데몬이 바로 뜨며, 실행 중인 상태에서 재설치해도 되고, 앱 내 업데이트 후 새 버전으로 재시작합니다.
- Windows — 콘솔 창 깜빡임 제거, 앱 실행 후 설치한 에이전트도 재시작 없이 감지, 설치 안내에 Windows용 설치 명령 표시, 옛 Claude Desktop 별칭이 데스크톱 앱을 띄우던 문제 수정.
- 데몬 포트를 49517에서 옮겼습니다. 이 포트는 Windows의 Hyper-V·WSL2·Docker가 무작위로 예약하는 범위라, 해당 PC에서 앱이 가끔 시작되지 못했습니다.
- Claude Code: 완료 알림에 이번 턴의 실제 마지막 답변이 표시됩니다 — 트랜스크립트가 비동기로 기록돼 가끔 이전 답변이 나왔습니다.
- Codex: BOM이 있는 config.toml에 notify 줄이 중복으로 들어가던 문제 수정, CODEX_HOME 반영, 이름으로 지정된 기존 notify 명령도 계속 전달합니다.
- opencode: 알림에 세션 제목이 표시됩니다(Windows에서 클릭 시 터미널을 찾는 데도 쓰임). Windows에서 세션 목록의 제외 폴더가 슬래시(/)로 저장된 opencode 세션도 보호합니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- Windows：点击通知现在会真正聚焦会话所在窗口 — 此前 toast 回调走向 CLI 无法访问的路径，自 v0.8.0 起点击毫无反应。
- 安全：钩子使用的本地端口现在会拒绝来自网页浏览器的请求，网页无法再伪造通知或弹出窗口。
- Windows — Claude Code：在未安装 Git for Windows 的电脑上，钩子经 PowerShell 运行并解析失败；自 Claude Code 2.1.299 起这会阻止所有工具调用。现改为以无 shell 的 exec 形式注册，现有安装会自动迁移。
- Windows — Gemini、Cursor、Antigravity：用户目录名含空格（C:\Users\John Smith）时钩子会失败。Gemini/Cursor 改用 PowerShell 安全的命令，Antigravity 改用 cmd 安全的路径，现有条目自动修复；带 UTF-8 BOM 的钩子数据（Windows PowerShell 5.1）也能正确读取。
- Windows — Cursor：不再注册提交提示钩子 — 从 Git Bash 启动的 Cursor 会把其崩溃视为"阻止"，从而拒绝所有提示。
- Windows — Antigravity：额度表现在从凭据管理器读取 agy 令牌，出错停止时也会再次提醒。
- Windows — 安装：下载的 exe 首次运行即注册开机自启，`install` 后守护进程立即启动，可覆盖正在运行的版本重新安装，应用内更新后会重启到新版本。
- Windows — 减少多余的控制台窗口，登录后安装的代理无需重启应用即可识别，安装提示显示 Windows 安装命令，旧版 Claude Desktop 别名不再打开桌面应用。
- 守护进程端口已从 49517 迁移 — 该端口位于 Windows 的 Hyper-V/WSL2/Docker 随机保留的范围内，在这些电脑上应用有时无法启动。
- Claude Code：完成通知现在显示本轮真正的最后回复 — 转录文件异步写入，有时仍显示上一条回复。
- Codex：带 BOM 的 config.toml 不再被重复写入 notify 行，支持 CODEX_HOME，按名称指定的原有 notify 命令也会继续转发。
- opencode：通知会显示会话标题（Windows 上点击时也借此找到终端）；在 Windows 上，会话列表中排除的文件夹现在也能保护以正斜杠存储的 opencode 会话。

</details>
