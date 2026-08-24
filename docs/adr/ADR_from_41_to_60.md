# CopyLingo 의사결정 기록 (ADR)

## ADR-041: Daily Quiz Session을 2문항 확장하고 Listening 1자리를 예약한다

- **날짜**: 2026-08-05
- **상태**: 채택됨
- **맥락**:
  - Listening Question 50개와 audio가 모두 준비됐지만, 신규 문항은 고정 순서 Random Slot Relay의 다섯 번째 category였다.
  - 기존 Evening 10문항은 due review 6개와 신규 Vocabulary 4개가 모든 자리를 채워 Listening이 출제될 수 없었다.
  - 기존 Morning 15문항도 review 6개와 Vocabulary 5개 이후 최대 4자리만 relay에 남아, 선행 category가 Listening 자리를 대부분 소진했다.
- **결정**:
  - Morning Session은 15개에서 17개, Evening Session은 10개에서 12개로 늘린다.
  - 두 Daily Session 모두 audio-ready unseen Listening을 최대 1개 먼저 예약한다.
  - Listening 후보가 없거나 조회에 실패하면 빈자리는 기존 Random Slot Relay가 일반 category로 채운다.
  - Vocabulary 최소 1/3 예약은 유지한다. 이에 따라 Morning은 review 최대 6 + Vocabulary 6 + Listening 1, Evening은 review 최대 7 + Vocabulary 4 + Listening 1을 우선 편성하고 남는 자리는 relay로 채운다.
  - SRS Review Session의 총량과 편성 정책은 변경하지 않는다.
- **결과 / 트레이드오프**:
  - 신규 Listening 노출이 category random allocation에만 의존하지 않는다.
  - Daily 학습량은 하루 최대 4문항 증가한다. Listening inventory를 모두 소진한 뒤에는 예약 시도가 빈 조회 1회를 추가하지만 기존 relay가 자리를 회수한다.

## ADR-042: Scheduled Session은 미완료 backlog를 재알림한다

- **날짜**: 2026-08-22
- **상태**: 채택됨
- **맥락**:
  - Morning/Evening Quiz와 정오/오후 Study cron은 사용자에게 `pending` 또는 `in_progress` 세션이 있어도 새 세션을 생성해 backlog를 누적했다.
  - 별도 사전 build job은 현재 등록되어 있지 않으며, 각 push cron이 세션 생성과 Telegram 발송을 연속 수행한다.
- **결정**:
  - Scheduled cron은 사용자별 Quiz/Study 전체에서 미완료 세션을 먼저 조회한다.
  - `in_progress`를 우선하고, 같은 상태에서는 가장 오래된 세션을 선택한다. 미완료 세션이 있으면 새 세션을 생성하지 않고 해당 mode에 맞는 Telegram 알림을 다시 보낸다.
  - 미완료 세션이 없을 때만 기존처럼 cron 종류에 맞는 새 세션을 생성하고 발송한다.
  - `/study` 등 사용자가 직접 요청하는 세션 생성은 제한하지 않는다. `expired` 전환과 기존 backlog 일괄 정리는 이번 결정에 포함하지 않는다.
- **결과 / 트레이드오프**:
  - Scheduled session이 무한히 쌓이지 않고 사용자가 진행 중이거나 오래 기다린 세션부터 소비하게 된다.
  - 사용자가 미완료 세션을 끝내지 않으면 새 학습 콘텐츠는 노출되지 않고 같은 세션 알림이 반복된다.
  - 조회 후 생성 사이의 동시성 경쟁을 완전히 차단하는 distributed lock 또는 DB 제약은 별도 확장성 과제로 남는다.
