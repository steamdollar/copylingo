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
  - **2026-08-25 보정**: 재알림된 `in_progress` 세션 진입은 새 시작이 아니라 resume으로 취급한다. Quiz/Study 모두 Redis working set을 우선하고, Redis miss일 때만 DB에서 복구하며, 첫 미답변 문제/미학습 카드부터 다시 표시한다.
  - 이미 처리된 Telegram callback은 오류 문구를 추가 전송하지 않고 현재 첫 미답변 문제로 self-heal한다. 모든 문제가 답변된 상태라면 기존 완료 경로를 사용한다.
- **결과 / 트레이드오프**:
  - Scheduled session이 무한히 쌓이지 않고 사용자가 진행 중이거나 오래 기다린 세션부터 소비하게 된다.
  - 사용자가 미완료 세션을 끝내지 않으면 새 학습 콘텐츠는 노출되지 않고 같은 세션 알림이 반복된다.
  - 완료 전 진행 상태의 SSOT는 Redis working set이며 DB는 완료 시 flush된다. 따라서 Redis TTL 만료 시에는 DB에 없는 부분 진행 상태를 복구할 수 없다는 기존 제약은 유지된다.
  - 조회 후 생성 사이의 동시성 경쟁을 완전히 차단하는 distributed lock 또는 DB 제약은 별도 확장성 과제로 남는다.

## ADR-043: Word Order는 Telegram tap-to-build와 Redis draft로 구현한다

- **날짜**: 2026-08-24
- **상태**: 채택됨
- **맥락**:
  - `QuestionWordOrder` type과 `SkillSentenceComposition` taxonomy는 있지만 seed·renderer·answer path가 없어 실제로는 출제되지 않았다.
  - Telegram inline keyboard에서 drag-and-drop은 지원되지 않으며, 이 문제만을 위한 Mini App은 별도 web UI·auth·submit endpoint를 요구한다.
  - 문장 조립 중간 상태는 최종 채점 결과가 아니므로 DB 영속 대상이 아니다.
- **결정**:
  - 사용자는 Telegram inline keyboard의 문장 조각을 순서대로 tap하고, `되돌리기`·`초기화`·`제출` action으로 조립한다. 반복되는 조각은 text가 아닌 option index로 식별한다.
  - 조각 표시 순서는 session·question ID로 안정적으로 shuffle하여 callback 사이에 유지한다.
  - 선택한 index 목록은 question별 별도 Redis draft key에 active session과 같은 TTL로 저장하고, 완료·세션 종료 시 삭제한다. DB `session_questions.user_answer`에는 제출한 최종 문장만 저장한다.
  - `questions.options` JSONB에는 문장 조각 배열, `correct_answer`에는 정규 완성 문장을 저장한다. 일본어 MVP는 조각을 공백 없이 join해 기존 exact-match grader에 전달한다.
  - 초기 데이터는 기존 N5 grammar material에 연결된 static seed로 구성하고 stable `question_key`를 사용한다.
- **결과 / 트레이드오프**:
  - 기존 Question catalog·SessionBuilder·exact grader·SRS를 재사용하므로 DB migration과 복습 정책 변경이 없다.
  - 조각 tap마다 작은 Redis write가 발생하지만, 전체 Active Session blob을 다시 저장하지 않아 write amplification을 제한한다.
  - Drag UX와 다국어 delimiter·복수 정답 지원은 현재 일본어 Telegram MVP 범위에서 제외한다.

## ADR-044: Go toolchain과 container builder를 1.27.0으로 올린다

- **날짜**: 2026-08-24
- **상태**: 채택됨
- **맥락**:
  - Project의 `go.mod`는 Go 1.25.5, Docker builder는 `golang:1.25-alpine`에 고정돼 있어 local과 container의 patch version이 일치하지 않았다.
  - Go 1.27.0은 2026-08-19 정식 release됐고 Go 1 compatibility를 유지한다.
- **결정**:
  - `go.mod` minimum Go version을 `1.27.0`, Docker builder image를 `golang:1.27.0-alpine`로 올려 local·CI·container build 기준을 같은 patch version으로 맞춘다.
  - Upgrade와 dependency version 변경을 분리하고, Go 1.27 신규 language·standard-library API는 이 작업에서 도입하지 않는다.
  - Go 1.27.0으로 `go mod tidy`, `make test`, binary build, container rebuild, health check를 모두 통과해야 upgrade를 완료한다.
- **결과 / 트레이드오프**:
  - 최신 supported toolchain의 runtime·compiler·standard-library 개선을 사용하고 local/container 재현성을 높인다.
  - `.0` release의 초기 regression 가능성은 있지만 full test·container smoke로 현재 application contract를 검증하고, 문제 시 두 version pin을 같이 revert한다.
