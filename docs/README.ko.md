<div align="center">

<img src="../assets/logo.png" width="96" alt="agent-notify 로고"/>

<h1>agent-notify</h1>

<p><b>끝난 코딩 에이전트 세션, 다시는 놓치지 마세요 — Claude Code · Codex · Antigravity · Gemini · opencode · Cursor.</b></p>

<p>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest"><img src="https://img.shields.io/github/v/release/CoderGangW/agent-notify?color=8a2be2" alt="latest release"/></a>
  <img src="https://img.shields.io/badge/platforms-macOS%20%7C%20Windows%20%7C%20Linux-blue" alt="platforms"/>
  <img src="https://img.shields.io/badge/Go-Wails%20v3-00ADD8?logo=go&logoColor=white" alt="Go + Wails"/>
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-Source--Available%20ND-orange" alt="license"/></a>
</p>

<p>
  🇺🇸 <a href="../README.md">English</a>&nbsp;&nbsp;·&nbsp;&nbsp;🇰🇷 <b>한국어</b>&nbsp;&nbsp;·&nbsp;&nbsp;🇨🇳 <a href="README.zh-CN.md">简体中文</a>
</p>

<p>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest/download/agent-notify-macos-universal.app.zip"><img src="https://img.shields.io/badge/macOS-Download-111111?style=for-the-badge&logo=apple&logoColor=white" alt="Download for macOS"/></a>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest/download/agent-notify-windows-amd64.exe"><img src="https://img.shields.io/badge/Windows-Download-0a66c2?style=for-the-badge" alt="Download for Windows"/></a>
  <a href="https://github.com/CoderGangW/agent-notify/releases/latest/download/agent-notify-linux-amd64"><img src="https://img.shields.io/badge/Linux-Download-f4a028?style=for-the-badge&logo=linux&logoColor=white" alt="Download for Linux"/></a>
</p>

<img src="../assets/main_screen_kr.png" width="360" alt="agent-notify 대시보드"/>

</div>

코딩 에이전트를 여러 개 동시에 돌리다 보면 어느 게 방금 끝났는지, 어느 게 내 입력을 기다리는지, 플랜을 얼마나 썼는지 놓치기 쉽습니다. **agent-notify**는 macOS · Windows · Linux용 작은 트레이 앱으로 이 셋을 해결합니다 — 세션이 끝나거나 입력이 필요한 순간 알려주고, 클릭 한 번으로 그 세션이 돌던 창으로 돌아가게 해주며, 세션·토큰 사용량·플랜 한도를 대시보드 하나에 모아 보여줍니다.

## 이런 걸 해줍니다

**끝났는지, 멈췄는지 바로 알기**
- ✅ 작업이 끝나면 네이티브 OS 알림, 🔔 에이전트가 입력이나 권한 승인을 기다리면 알림
- *어느* 세션인지 알림만 봐도 압니다: 제목 = 세션 제목(VSCode 익스텐션이 붙인 제목 포함), 본문 = 무엇을 했는지 한 줄 AI 요약(실패 시 마지막 응답 발췌)
- 알림 모드 4단계 — **켬**, **알림만**, **조용히**(배지만, 배너 없음), **무음**

**바로 그 창으로 돌아가기**
- 알림·이벤트·활성 세션을 클릭하면 세션이 돌던 바로 그 창이 앞으로 옵니다 — IDE 워크스페이스(VSCode / Cursor / Windsurf), 터미널 탭, 멀티플렉서 페인
- 멀티루트 `.code-workspace`와 하위 폴더 세션도 새 창을 띄우지 않고 이미 열려 있는 창을 포커스

**모든 에이전트를 한 대시보드에서** — 트레이 아이콘 클릭
- 고른 에이전트마다 탭 하나; 탭마다 설정 체크리스트(설치됨 · 로그인됨 · 훅 연결됨)와 원클릭 해결 버튼
- **활성 세션**과 실시간 상태 — 작업 중 · 입력 대기 · 유휴
- **세션 이벤트** 피드 — 읽지 않음 배지, 검색, 필터
- **사용량 통계** — 시간별 / 일별 / 월별 토큰 차트(스크롤 확대, 드래그 이동), 모델별
- **플랜 한도 게이지** — `/usage`가 보여주는 5시간·주간 사용률과 리셋 카운트다운
- English / 한국어 / 简体中文, 라이트 / 다크 / 시스템 테마, 로그인 시 자동 시작, 새 버전 알림과 원클릭 설치

## 지원 에이전트

| 에이전트 | 작업 완료 | 입력 필요 | 실시간 세션 상태 | 사용량·한도 | 연결 명령 |
|---|:-:|:-:|:-:|---|---|
| **Claude Code** | ✅ | ✅ | ✅ | 토큰 사용량 + 플랜 한도 | `agent-notify install` |
| **Codex CLI** | ✅ | — | — | — | `agent-notify install-codex` |
| **Gemini CLI** (베타) | ✅ | ✅ | ✅ | — | `agent-notify install-gemini` |
| **Antigravity CLI** (베타) | ✅ | — | ✅ | 모델 쿼터 게이지 | `agent-notify install-antigravity` |
| **opencode** (베타) | ✅ | ✅ (권한 요청) | ✅ | 토큰·비용 사용량 | `agent-notify install-opencode` |
| **Cursor CLI** (베타) | ✅ | — | ✅ | — | `agent-notify install-cursor` |

연결 명령은 한 번만 실행하면 됩니다 — 각 에이전트 자체의 훅/플러그인 설정에 agent-notify를 등록하며, `agent-notify uninstall`로 되돌릴 수 있습니다. 대시보드의 설정 체크리스트가 대신 실행해 주므로 터미널을 열 일은 거의 없습니다.

## 클릭-포커스 지원 범위

| 세션이 돌던 곳 | 포커스 대상 | 비고 |
|---|---|---|
| VSCode / Cursor / Windsurf | 정확한 워크스페이스 창 | 멀티루트 워크스페이스·하위 폴더 지원 |
| tmux | 정확한 페인 | 윈도우·페인·클라이언트 전환 |
| WezTerm | 정확한 페인 | |
| iTerm2 | 정확한 세션 | |
| kitty | 윈도우 | `allow_remote_control` 필요 |
| GNU screen | 윈도우 | |
| Zellij | 페인 (베스트 에포트) | `focus-pane-with-id` 지원 버전 필요 |
| [cmux](https://cmux.com) | 워크스페이스 + 페인 | Settings → Automation → socket 접근을 password/allowAll로 |
| 그 외 터미널 | 앱 창 | |

플랫폼별 정밀도: **macOS** — 위 전부. **Windows** — 세션의 창을 앞으로 가져옴. **Linux** — 세션의 프로젝트 폴더를 엶.

## 설치

한 줄이면 앱 다운로드, Claude Code 연결, 로그인 시 자동 시작까지 끝납니다:

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.sh | sh
```

```powershell
# Windows
irm https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.ps1 | iex
```

**macOS, 터미널 없이:** 위 버튼으로 **agent-notify.app**(서명됨, 유니버설)을 받아 열기만 하세요 — 첫 실행에서 Claude Code 연결과 로그인 자동 시작을 스스로 등록하고, 필요한 알림·폴더 접근 권한을 요청합니다.

Go가 있다면: `go install github.com/CoderGangW/agent-notify@latest && agent-notify install`.

다른 에이전트는 대시보드의 에이전트 선택에서 추가하거나, 위 표의 `install-*` 명령으로 연결합니다. `agent-notify uninstall`은 모든 훅과 자동 시작 항목을 제거합니다.

## 플랫폼 참고

- **macOS** — 앱 자체 아이콘의 네이티브 알림, 별도 도구 불필요. 첫 실행 시 알림 권한과 데스크탑/문서/다운로드 폴더 접근 권한을 요청합니다(폴더 접근이 있어야 클릭 시 정확한 IDE 창을 포커스할 수 있습니다).
- **Windows** — 토스트 알림, 추가 의존성 없음, 콘솔창 없음.
- **Linux** — 알림은 `notify-send`; 대시보드 창은 `libgtk-3`와 `libwebkit2gtk-4.1`, 트레이 아이콘은 `libayatana-appindicator`가 필요합니다. 없어도 훅 기반 알림은 동작합니다.
- **AI 요약**은 이미 로그인된 `claude` CLI를 사용합니다(Haiku, 알림당 토큰 소량). 설정에서 끄거나 `CLAUDE_NOTIFY_NO_AI=1`로 비활성화할 수 있습니다.

## 개인정보

모든 것은 내 컴퓨터 안에 있습니다. 토큰 사용량은 로컬 트랜스크립트에서 계산하고, 플랜 한도는 Claude Code가 이미 저장해 둔 자격증명으로 읽기 전용 조회합니다 — 갱신·수정·업로드하지 않습니다. 네트워크 요청은 보이는 것뿐입니다: 한도 조회, 선택적 AI 요약, 새 버전 확인.

## 로드맵

- [x] 알림/이벤트 클릭으로 세션의 IDE 또는 터미널 포커스
- [x] 대시보드: 세션, 토큰 사용량, 플랜 한도
- [x] Codex · Gemini · Antigravity · opencode · Cursor
- [ ] VSCode 익스텐션의 특정 세션 패널까지 포커스 (익스텐션 딥링크 API 필요)
- [ ] Webhook (ntfy / Slack / Telegram / Discord) — 자리 비웠을 때 폰 알림
- [ ] 원격 머신 이벤트 수집

## 릴리즈 노트

버전별 변경사항은 [release_notes/](../release_notes/)와 [릴리즈 페이지](https://github.com/CoderGangW/agent-notify/releases)에서 볼 수 있습니다.

## 라이선스

소스 공개형 라이선스: 무수정 사용·재배포는 자유(상업적 사용 포함)이나, 수정 및 수정본 배포는 서면 허가 필요. [LICENSE](../LICENSE) 참고.
