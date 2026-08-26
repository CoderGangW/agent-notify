# claude-notify

Claude Code 세션 여러 개를 돌릴 때 **어느 작업이 끝났는지** 바로 알려주는 크로스플랫폼(macOS / Windows / Linux) 트레이 데몬.

터미널을 일일이 돌아다니며 확인할 필요 없이:

- ✅ 작업 완료(Stop) / 🔔 입력 필요(Notification) 시 **OS 네이티브 알림**
- 알림 제목 = **세션 제목**, 내용 = **Claude의 작업 요약** (transcript에서 마지막 응답 추출) — 어느 세션에서 뭘 했는지 즉시 식별
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

```sh
go install github.com/CoderGangW/claude-notify@latest

# Claude Code hook 자동 등록 (~/.claude/settings.json 수정, 기존 설정은 백업 후 병합)
claude-notify install

# 트레이 데몬 실행
claude-notify
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
| `claude-notify install` | `~/.claude/settings.json`에 Stop/Notification hook 등록 |
| `claude-notify uninstall` | 등록한 hook 제거 |
| `claude-notify hook` | Claude Code가 호출하는 hook 엔드포인트 (직접 실행할 일 없음) |

## 아이콘 교체

트레이 아이콘은 빌드 시 임베드됨. `assets/icon.png`(Windows/Linux, 컬러), `assets/icon_mac.png`(macOS 메뉴바, 단색 템플릿)를 원하는 32×32 PNG로 교체 후 `go build`.

## 플랫폼 참고

- **macOS**: 알림은 `osascript` 경유라 발신 앱이 "Script Editor"로 표시되고 알림 아이콘 커스텀 불가 (UNUserNotificationCenter 기반 .app 번들은 로드맵). 최초 알림 시 알림 허용 필요할 수 있음. 로그인 시 자동 실행하려면 시스템 설정 → 로그인 항목에 바이너리 추가.
- **Linux**: 트레이에 `libayatana-appindicator`, 알림에 `notify-send`(libnotify) 필요.
- **Windows**: 토스트 알림 사용. 별도 의존성 없음.

## 로드맵

- [ ] 알림 클릭 시 해당 터미널/IDE 포커스
- [ ] localhost 대시보드 (진행 중 세션 상태 한눈에)
- [ ] 원격 머신 세션 이벤트 수집

## License

MIT
