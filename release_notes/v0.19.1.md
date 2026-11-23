<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- **Correct plan name**: the plan chip now reads `planDisplayName` from `~/.claude.json`, so an Ultra subscription shows as "Ultra" instead of the keychain's coarser "Max".
- **Account email no longer hidden by errors**: a usage-API failure used to replace the signed-in email on the limits card. The email comes from a different source, so it stays put; the error shows beside it when gauges are present, or in the card body when they aren't.
- **Data folder renamed**: `~/.claude-notify` is now `~/.agent-notify`. It migrates automatically on first launch after updating — settings, stats and logs come along — and the old path stays as a link to the new one.

<details><summary>🇰🇷 한국어</summary>

- **플랜 이름 정확히 표시**: 플랜 칩이 `~/.claude.json`의 `planDisplayName`을 읽습니다. Ultra 구독이 키체인의 뭉뚱그린 "Max" 대신 "Ultra"로 표시됩니다.
- **에러가 계정 이메일을 가리지 않음**: 사용량 API 실패 시 플랜 한도 카드의 로그인 이메일이 에러 메시지로 덮이던 문제. 이메일은 출처가 달라 그대로 유지되고, 에러는 게이지가 있으면 옆에, 없으면 카드 본문에 표시됩니다.
- **데이터 폴더 이름 변경**: `~/.claude-notify`가 `~/.agent-notify`로 바뀌었습니다. 업데이트 후 첫 실행 때 설정·통계·로그가 자동으로 옮겨지고, 옛 경로는 새 폴더를 가리키는 링크로 남습니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- **套餐名称显示正确**：套餐标签现在读取 `~/.claude.json` 的 `planDisplayName`，Ultra 订阅会显示为「Ultra」，而不是钥匙串笼统的「Max」。
- **错误不再遮挡账户邮箱**：用量 API 失败时，限额卡片上已登录的邮箱会被错误信息替换。邮箱来自另一数据源，因此会保留；错误在有仪表时显示在旁边，没有时显示在卡片正文。
- **数据文件夹更名**：`~/.claude-notify` 改为 `~/.agent-notify`。更新后首次启动时自动迁移（设置、统计和日志一并迁移），旧路径保留为指向新文件夹的链接。

</details>
