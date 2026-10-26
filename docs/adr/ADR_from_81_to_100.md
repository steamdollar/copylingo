# CopyLingo 의사결정 기록 (ADR)

## ADR-081: 학습 로직에 언어별 분기를 두지 않는다

- 날짜: 2026-10-26
- 상태: 승인됨
- 배경:
  - 레벨 순서·인접 레벨·레벨 통계 정렬이 일본어 전용으로 짜여 있었다. service에 `japaneseProficiencyLevels`·`jlptLevelRank`와 `if language != "ja"`·`if user.Language == "ja"` 분기가 있었다. 일본어가 아닌 언어는 오류 없이 다른 경로(그 레벨만 범위, 정렬 없음)를 탔다.
  - 2026-10-26 수정(ADR-079 정정)에서 레벨 순서를 언어 key map `proficiencyLevels`로 옮기고, `levelRank`·`neighborLevels`·`sessionLevelsFor`가 이 map만 읽게 바꿨다. 이 방식을 이후 모든 작업의 규칙으로 고정한다.
- 결정:
  - 로직 코드(service·bot·scheduler·repository·miniapp)는 학습 언어나 레벨 이름으로 분기하지 않는다. 언어별 사실은 언어를 key로 하는 data에 두고, 로직은 언어를 인자로 받아 조회한다.
  - data에 없는 언어·레벨은 오류나 예외 분기가 아니라 중립 기본값으로 처리한다(그 레벨만 범위, 순위 맨 뒤).
  - 일부 언어에만 있는 기능(가나 손글씨, 한자 해설)은 언어 코드가 아니라 문제 유형이나 data 유무로 켠다.
  - 원칙은 AGENTS.md §4 원칙 4에, 구현 규칙은 CONVENTIONS.md "Learning-language neutrality"에 둔다.
- 버린 대안:
  - 언어가 늘 때마다 분기 추가: 언어마다 로직 경로가 갈라져 테스트·검토 범위가 언어 수만큼 늘어난다.
  - 언어별 strategy interface 구현체: 지금 언어 간 차이는 data(레벨 순서·콘텐츠)뿐이라 과하다. 편성 규칙 자체가 언어마다 달라질 때 다시 검토한다.
- 결과:
  - 언어 추가는 data 추가(레벨 순서 배열, seed 파일)로 끝나고, 같은 편성 규칙이 그대로 적용된다.
  - 레벨·언어 로직 테스트는 data에 없는 언어·레벨 사례를 포함한다(`TestNeighborLevels`의 `en`/`B1`, `ja`/`N0`).
  - 단일 언어 외부 출처 adapter(`internal/pipeline/fetcher_nhk.go`)의 상수와 CLI 기본값(`cmd/admin/generate_listening_audio`의 `-language ja`)은 로직 분기가 아니므로 허용한다.
