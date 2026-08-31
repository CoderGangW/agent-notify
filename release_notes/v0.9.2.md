<!-- Draft for the next release. Fill all three languages — English is
     the main body, Korean/Chinese are collapsed.
     release.sh archives this as v<version>.md on release and resets it. -->

- Fixed: plan-limit gauges could jump around or duplicate as the usage API changed which buckets it returned. The order is now pinned (session → weekly all → per-model weekly), gauges only move when actually out of place (no restarted bar animations), and buckets the API stops returning are removed.

<details><summary>🇰🇷 한국어</summary>

- 수정: usage API가 반환하는 버킷이 바뀔 때 플랜 한도 게이지가 순서가 뒤바뀌거나 중복되던 문제. 이제 순서가 고정되고(세션 → 주간 전체 → 모델별 주간), 실제로 위치가 바뀔 때만 이동해 바 애니메이션이 다시 시작되지 않으며, API가 더 이상 반환하지 않는 버킷은 제거됩니다.

</details>

<details><summary>🇨🇳 简体中文</summary>

- 修复：当用量 API 返回的额度桶发生变化时，套餐额度仪表可能错位或重复。现在顺序固定（会话 → 每周全部 → 每模型每周），仅在实际位置变化时才移动（不会重启进度条动画），API 不再返回的桶会被移除。

</details>
