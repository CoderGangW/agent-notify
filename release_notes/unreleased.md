<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Reliable relaunch after in-app updates (macOS)**: updating now reloads the launchd job (bootout + bootstrap) instead of kickstart. The daemon's clean exit used to satisfy the job's `KeepAlive SuccessfulExit=false` semaphore, so newer macOS ignored the kickstart and the app stayed dead — or came back unmanaged via `open`, losing its daemon log. The hook's dead-daemon revive uses the same reload, so a quit or crashed daemon comes back under launchd management again.

<details><summary>🇰🇷 한국어</summary>

- **앱 내 업데이트 후 재시작 안정화 (macOS)**: 업데이트 시 kickstart 대신 launchd job을 리로드(bootout + bootstrap)합니다. 데몬의 정상 종료가 job의 `KeepAlive SuccessfulExit=false` 세마포어를 충족시켜 최신 macOS에서 kickstart가 무시됐고, 앱이 죽은 채로 남거나 `open`으로 launchd 관리 밖에서(로그 유실) 되살아나는 문제가 있었습니다. hook의 죽은 데몬 부활도 같은 리로드를 사용해 종료·크래시 후에도 launchd 관리로 복귀합니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **应用内更新后可靠重启 (macOS)**：更新时改为重载 launchd job（bootout + bootstrap）而非 kickstart。守护进程的正常退出会满足 job 的 `KeepAlive SuccessfulExit=false` 信号量，导致较新的 macOS 忽略 kickstart — 应用要么保持死亡，要么通过 `open` 以非托管方式复活（丢失日志）。钩子的死进程复活也使用同样的重载，退出或崩溃后都会回到 launchd 托管。

</details>
