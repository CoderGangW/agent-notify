# claude-notify

[English](../README.md) | **한국어** | [简体中文](README.zh-CN.md)

Claude Code 세션 여러 개를 돌릴 때 **어느 작업이 끝났는지** 바로 알려주는 크로스플랫폼(macOS / Windows / Linux) 트레이 데몬.

터미널을 일일이 돌아다니며 확인할 필요 없이:

- ✅ 작업 완료(Stop) / 🔔 입력 필요(Notification) 시 **OS 네이티브 알림**
- 알림 제목 = **세션 제목** (VSCode 확장이 생성한 제목 포함), 부제 = **프로젝트 경로**, 내용 = **AI 한 줄 요약** (`claude -p --model haiku`, 기존 인증 그대로 사용 · 실패 시 마지막 응답 발췌로 폴백)
- macOS에서 `terminal-notifier` 설치 시 **알림 클릭하면 해당 세션의 IDE로 포커스** (VSCode/Cursor/Windsurf 자동 감지)
- 트레이 메뉴에 최근 이벤트 목록, 클릭하면 해당 프로젝트 폴더 열기
- macOS 메뉴바에 완료된 작업 수 뱃지

## 동작 원리

```
Claude Code ──(Stop / Notification hook)──▶ claude-notify hook
                                                │  POST localhost:49517
                                                ▼
                                        트레이 데몬 ──▶ OS 알림 + 이벤트 목록
```

데몬이 꺼져 있으면 hook이 OS 알림을 직접 보내는 폴백으로 동작하므로, 데몬 없이 hook만 등록해도 알림 자체는 작동한다.

## 설치

한 줄 설치 — 바이너리 다운로드 + Claude Code hook 등록 + 로그인 자동 시작까지 전부:

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/CoderGangW/claude-notify/main/install.sh | sh
```

```powershell
# Windows
irm https://raw.githubusercontent.com/CoderGangW/claude-notify/main/install.ps1 | iex
```

Go 있으면:

```sh
go install github.com/CoderGangW/claude-notify@latest
claude-notify install   # hook 등록 + 자동 시작 등록 + 데몬 시작
```

소스 빌드:

```sh
git clone https://github.com/CoderGangW/claude-notify
cd claude-notify
go build -o claude-notify .
```

## 명령어

| 명령 | 설명 |
|---|---|
| `claude-notify` | 트레이 데몬 실행 (기본) |
| `claude-notify install` | Stop/Notification hook 등록 + 로그인 자동 시작 등록(LaunchAgent / XDG autostart / 레지스트리) + 데몬 시작 |
| `claude-notify uninstall` | hook과 자동 시작 등록 제거 |
| `claude-notify hook` | Claude Code가 호출하는 hook 엔드포인트 (직접 실행할 일 없음) |
| `claude-notify peek <transcript.jsonl> [session-id]` | 해당 세션에서 뽑히는 제목/요약 소스 디버그 출력 |

## 아이콘 교체

트레이 아이콘은 빌드 시 임베드됨. `assets/icon.png`(Windows/Linux, 컬러), `assets/icon_mac.png`(macOS 메뉴바, 단색 템플릿)를 원하는 32×32 PNG로 교체 후 `go build`.

## 플랫폼 참고

- **macOS**: `brew install terminal-notifier` 강력 추천 — 클릭 시 IDE 포커스 + 경로 부제 지원. 최초 알림 시 시스템 설정 → 알림에서 terminal-notifier 허용 필요. 없으면 `osascript` 폴백 (발신 앱 "Script Editor"로 표시, 클릭 동작 없음). 알림 발신자명은 terminal-notifier로 표시됨 (자체 발신자명은 .app 번들 필요 — 로드맵).
- **AI 요약**: `claude` CLI가 PATH에 있으면 자동 사용 (요약 시간만큼 알림이 몇 초 늦게 옴). 끄려면 환경변수 `CLAUDE_NOTIFY_NO_AI=1`. 요약용 하위 세션의 hook 재귀는 `CLAUDE_NOTIFY_SUPPRESS` 가드로 차단됨.
- **Linux**: 트레이에 `libayatana-appindicator`, 알림에 `notify-send`(libnotify) 필요.
- **Windows**: 토스트 알림 사용. 별도 의존성 없음.

## 로드맵

- [x] 알림 클릭 시 해당 IDE 포커스 (macOS + terminal-notifier)
- [ ] 자체 .app 번들 + UNUserNotificationCenter — 알림 발신자명/아이콘을 claude-notify로
- [ ] localhost 대시보드 (진행 중 세션 상태 한눈에)
- [ ] 원격 머신 세션 이벤트 수집

## License

MIT
