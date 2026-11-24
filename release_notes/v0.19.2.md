<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Session cleaner: no more doubled Claude sessions**: with a second Claude config folder (e.g. `~/.claude-r2` for another account in VSCode) that links its `projects/` back to `~/.claude`, every session was listed twice. Shared folders are now scanned once, and a session running under either account still counts as running, so it stays protected from deletion.

<details><summary>🇰🇷 한국어</summary>

- **세션 정리 창: Claude 세션 중복 표시 수정**: 두 번째 Claude 설정 폴더(예: VSCode에서 다른 계정용 `~/.claude-r2`)가 `projects/`를 `~/.claude`로 링크해 두면 모든 세션이 두 번씩 보이던 문제. 공유 폴더는 한 번만 읽고, 어느 계정에서 실행 중이든 실행 중으로 인식해 삭제 보호도 그대로 유지됩니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **会话清理窗口：Claude 会话不再重复显示**：当第二个 Claude 配置文件夹（例如在 VSCode 中用于另一账户的 `~/.claude-r2`）将 `projects/` 链接回 `~/.claude` 时，每个会话都会显示两次。现在共享文件夹只扫描一次，任一账户下运行中的会话都会被识别为运行中，删除保护保持不变。

</details>
