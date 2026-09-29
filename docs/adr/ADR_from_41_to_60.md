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

## ADR-045: Scheduled Session backlog를 합산 3개까지 허용한다

> `schedule.max_unfinished_sessions` 설정 방식은 ADR-053에서 제거했다. 사용자별 미완료 세션 3개 상한과 재알림 규칙은 유지한다.

- **날짜**: 2026-08-30
- **상태**: 채택됨
- **맥락**:
  - ADR-042는 Quiz/Study 전체에서 미완료 세션이 하나라도 있으면 새 scheduled session을 만들지 않고 기존 세션만 재알림한다.
  - 무한 backlog는 방지하지만, 세션 하나를 놓친 경우 이후 Quiz/Study content가 전혀 노출되지 않는 차단 효과가 크다.
- **결정**:
  - `pending`/`in_progress` Quiz·Study를 합산한 사용자별 scheduled backlog 상한을 `schedule.max_unfinished_sessions` 설정으로 관리하고 기본값을 3으로 둔다.
  - scheduled cron 실행 시 미완료 세션 수가 상한보다 적으면 해당 cron 종류의 새 세션을 생성·발송한다.
  - 상한에 도달하면 새 세션을 만들지 않고 ADR-042의 우선순위(`in_progress` 우선, 같은 상태에서 오래된 순)로 기존 세션 하나를 재알림한다.
  - `/study` 등 수동 생성 세션도 미완료 수에 포함하지만, 수동 요청 자체를 차단하지는 않는다. 자동 `expired` 전환은 도입하지 않는다.
- **결과 / 트레이드오프**:
  - 사용자는 일시적으로 세션을 놓쳐도 최대 3개의 서로 다른 scheduled content를 받으며, backlog은 유한하게 유지된다.
  - 상한 도달 전에는 기존 backlog 재알림 대신 새 세션이 전송되므로, 오래된 세션의 즉시 소비를 강제하지 않는다.
  - 수동 생성이 상한을 넘길 수 있고 조회 후 생성 사이의 동시성 race가 남는 제약은 유지한다.

## ADR-046: Study·Quiz는 현재 JLPT level과 인접 level을 함께 편성한다

> Quiz의 레벨별 편성 우선순위는 [ADR-050](ADR-050_current_level_quiz_focus.md)에서 보완한다. 인접 레벨 후보 범위는 부족분 보충에 유지한다.

- **날짜**: 2026-09-02
- **상태**: 채택됨
- **맥락**:
  - 사용자는 현재 level만 반복하기보다 바로 아래·위 난이도의 Material과 Question을 한 session에서 함께 학습하기로 했다.
  - 기존 Study·Quiz 조회는 `proficiency_level`을 단일 값으로 exact 비교해 인접 level의 신규·복습 item을 모두 누락했다.
  - `N1`·`N2` 같은 label은 문자열 정렬로 순서를 판단할 수 없고, Question `type`/render 동작과 `item_type` taxonomy는 이번 level scope와 독립적이다.
- **결정**:
  - 일본어 JLPT level 순서를 하나의 ordered slice로 정의하고, 현재 level의 index에서 바로 아래·위 한 단계만 session scope로 계산한다. 따라서 N4는 `[N5, N4, N3]`, N5는 경계 밖을 제외한 `[N5, N4]`가 된다.
  - authored content는 level-specific Go symbol이나 `switch` 대신 `LevelCatalog` registry로 묶는다. seeder와 material builder는 registry를 순회하며, 새 level은 dataset manifest entry로 등록한다.
  - Study의 신규·due Material과 Quiz의 신규·due Question 모두 같은 인접-level scope를 사용한다.
  - level별 고정 비율이나 우선순위는 두지 않고, 기존 category·difficulty·SRS 정렬과 후보 부족 fallback을 유지한다.
  - 일본어 이외 언어와 정의되지 않은 level은 ordering을 추측하지 않고 exact scope로 fallback한다. 사용자 level 값, content dataset, DB schema와 audio scheduler는 변경하지 않는다.
- **결과 / 트레이드오프**:
  - N4 사용자는 N5·N4·N3의 신규 콘텐츠와 due 복습을 함께 받고, N5 사용자는 N5·N4를 함께 받는다.
  - repository query는 단일 level 대신 최대 3개의 level array를 사용하며, SRS progress와 catalog 데이터의 migration은 필요 없다.
  - level별 분기와 중복 identifier가 없어져 새 catalog 추가가 기존 seeder 조립 코드를 수정하지 않지만, dataset 파일·생성 방식·기존 key compatibility는 manifest에서 명시해야 한다.
  - 향후 다른 언어 또는 proficiency 체계에 인접 범위를 적용하려면 해당 체계의 순서를 별도로 명시해야 하며, 미정 label을 lexical 비교로 처리하지 않는다.

## ADR-047: Study 시간대별 분량과 유형별 신규·복습 할당

- **날짜**: 2026-09-13
- **상태**: 채택됨
- Study를 아침 20개·저녁 24개로 편성하고 유형별 신규량을 보장한다. 현재 레벨의 신규 Material을 우선하며, 복습 부족 시 신규량을 임의로 늘리지 않는다.
- 상세: [ADR-047_study_morning_evening_mix.md](ADR-047_study_morning_evening_mix.md)

## ADR-048: 사용자별 맞춤 푸시 스케줄링 및 30분 폴링 엔진

- **날짜**: 2026-09-14
- **상태**: 채택됨
- 30분 이산 슬롯(`morning_study`, `morning_quiz`, `evening_study`, `evening_quiz`), 다중 타임존 partial index, Go 동시성 Worker Pool(4개)+Rate Limiter(25 msg/s), Redis 멱등성 락 및 텔레그램 설정 UI.
- 상세: [ADR-048_personalized_push_scheduling.md](ADR-048_personalized_push_scheduling.md)

## ADR-049: Study 복습 후보 부족분은 신규 단어로 보충한다

- 날짜: 2026-09-19
- 상태: 승인됨
- 배경: 9월 18일 Study가 아침 19/20개, 저녁 16/24개로 생성됐다. 신규 단어 재고는 충분하지만 ADR-047이 신규 할당을 상한으로 제한해 due 복습 부족분을 채우지 못했다. 사용자는 신규 학습량 증가를 감수하고 목표 수량을 유지하는 방향을 승인했다.
- 결정:
  - 기존 유형별 신규·복습 할당과 due 보충을 먼저 수행한다.
  - 그래도 총량이 부족하면 아직 선택하지 않은 신규 Vocabulary를 현재 레벨 우선으로 추가한다. 인접 레벨 범위와 난이도·무작위 정렬은 기존 신규 선택 정책을 따른다.
  - 신규 Grammar·Reading 할당은 늘리지 않고, 미래 복습을 당기거나 미완료 Study에 편성된 Material을 재사용하지 않는다.
  - 아침 20개·저녁 24개 및 수동 `/study n`의 요청 수량까지 보충하며, 허용된 due·신규 단어 후보가 모두 부족할 때만 짧은 세션을 반환한다.
  - ADR-047의 신규 상한 중 Vocabulary만 이 보충 단계에서 완화한다. 기존 세션과 SRS 이력은 변경하지 않는다.
- 결과: 복습이 적은 날 신규 단어 수와 학습 부담이 늘어날 수 있다. 예를 들어 9월 18일은 신규 단어를 아침 1개·저녁 8개 추가하면 목표 수량을 채운다. 기존 PostgreSQL 선택 쿼리 안에서 보충하므로 사용자별 전체 후보를 앱으로 가져오거나 별도 설정·스키마를 추가하지 않는다.

## ADR-050: Quiz는 현재 레벨과 최근 Study 자료를 우선 편성한다

- 날짜: 2026-09-19
- 상태: 승인됨
- 정규 Quiz에서 현재 레벨 최소 80%를 목표로 편성하고 독해·청해를 예약한다. 신규 문제는 최근 Study 자료를 우선하면서 자료별로 분산하고, 추가 복습도 현재 레벨 due부터 선정한다.
- 상세: [ADR-050_current_level_quiz_focus.md](ADR-050_current_level_quiz_focus.md)

## ADR-051: 사용자별 자료 유지 복습·제외와 공통 출제 기한

- 날짜: 2026-09-25
- 상태: 승인됨
- 사용자별 설정을 progress와 분리하고 `next_check_at`을 새 Study·연결 Quiz 후보에 공통 적용한다. 기존 세션은 유지하며 정답·오답에 따라 유지 간격을 조정한다.
- 상세: [ADR-051_user_material_preferences.md](ADR-051_user_material_preferences.md)

## ADR-052: 사용자별 세션 발송은 단일 30분 cron으로 구동한다

- 날짜: 2026-09-26
- 상태: 승인됨
- 사용자별 슬롯 시각은 DB에 두고, 서버는 매시 `:00`·`:30`에 발송 대상을 조회한다. 기존 전역 시각별 발송 및 자동 콘텐츠 수집 cron 등록을 제거한다.
- 상세: [ADR-052_single_push_cron.md](ADR-052_single_push_cron.md)

## ADR-053: 미완료 세션 상한을 발송 규칙으로 고정한다

- 날짜: 2026-09-26
- 상태: 승인됨
- 배경: ADR-045의 `schedule.max_unfinished_sessions`는 현재 `ScheduleConfig`의 유일한 값이며, 기본값 3과 허용 상한 3이 같아 설정으로는 상한을 낮추는 일만 가능하다.
- 결정: 서버 설정과 시작 시 범위 검증을 제거한다. 사용자마다 미완료 세션 수를 따로 조회하고, 발송 시 공통 상한 3개에 도달하면 기존 세션을 재알림하는 ADR-045 규칙은 유지한다.
- 결과: 발송 정책을 바꾸려면 코드를 수정해야 한다. 사용자별 상한 조정이 실제 요구사항이 되면 사용자 설정과 그 입력 경계에서 다룬다.

## ADR-054: TTS 활성화 설정을 제거한다

- 날짜: 2026-09-26
- 상태: 승인됨
- 배경: 청해 음성 생성은 기본으로 활성화되어 있고, `tts.enabled`는 서비스 생성과 관리 명령에서만 확인한다.
- 결정: `tts.enabled` 설정과 활성화 분기를 제거한다. API 키가 있으면 청해 음성 서비스를 구성한다. API 키가 없는 환경에서는 기존처럼 서비스를 구성하지 않고 관리 명령은 실행을 중단한다.
- 결과: TTS를 환경변수로 끌 수 없으며, 모델·음성 설정은 계속 사용할 수 있다.

## ADR-055: LLM과 TTS 설정을 하나의 타입으로 관리한다

- 날짜: 2026-09-26
- 상태: 승인됨
- 배경: TTS는 LLM과 API 키를 공유하며, 별도 `TTSConfig`에는 모델과 음성 이름만 남아 있다.
- 결정: TTS 모델과 음성 이름을 `LLMConfig`로 옮긴다. 설정 키는 `llm.tts_model`, `llm.tts_voice_name`이며 환경변수는 각각 `COPYLINGO_LLM_TTS_MODEL`, `COPYLINGO_LLM_TTS_VOICE_NAME`이다.
- 결과: 설정 타입과 YAML 구역 하나를 줄인다. 채팅은 OpenAI 호환 API, TTS는 Gemini native API를 사용하며 호출 방식은 유지한다. 이전 `COPYLINGO_TTS_MODEL`, `COPYLINGO_TTS_VOICE_NAME`은 더 이상 읽지 않는다.

## ADR-056: 새 청해 대화 음성은 A/B 두 화자로 합성한다

- 날짜: 2026-09-26
- 상태: 승인됨
- 배경: N4 청해 대화의 여러 발화가 한 목소리로 합성되어 화자 전환을 알아듣기 어렵다. 현재 TTS 모델은 두 화자 출력을 지원한다.
- 결정: `tts_voice_name`을 A의 목소리로 유지하고 B용 `tts_voice_name_b`를 추가한다. `「발화」「발화」` 형식의 스크립트는 A/B를 번갈아 배정해 다중 화자 요청으로 합성한다. 다른 스크립트는 단일 화자로 합성하며, 화자 라벨은 문제 화면에 표시하지 않는다.
- 결과: 앞으로 생성되는 음성의 캐시 키에 두 목소리를 포함한다. 이미 생성된 음성의 DB 경로와 Telegram 캐시는 재생성하지 않는다.

## ADR-057: 현재 서버 시작 경로에서는 서비스만 조립 결과로 전달한다

> 콘텐츠 수집 파이프라인의 저장 경계는 ADR-058에서 서비스 경유로 정리했다.

- 날짜: 2026-09-26
- 상태: 승인됨
- 배경: `initApp`이 서비스 구성에 필요한 저장소 묶음을 `run`에도 반환했지만, 유일한 후속 사용처는 자동 실행이 중단된 콘텐츠 수집 파이프라인 초기화였다.
- 결정: 저장소는 `initApp` 안에서 서비스 조립에만 사용하고 반환하지 않는다. 자동 수집 파이프라인은 시작 시 만들지 않으며, 관련 생성·실행 코드는 재도입을 위해 유지한다.
- 결과: 현재 발송·HTTP 경로는 서비스를 사용한다. 향후 콘텐츠 수집을 다시 연결할 때 서비스와 오케스트레이터를 그 실행 경로에서 명시적으로 구성해야 한다.

## ADR-058: 콘텐츠 수집의 저장 규칙은 서비스가 소유한다

- 날짜: 2026-09-26
- 상태: 승인됨
- 배경: 남겨둔 `pipeline.ContentSaver`가 중복 URL 확인과 DB 저장을 직접 수행해 파이프라인이 저장소에 접근하고 있었다.
- 결정: 해당 규칙을 `service.ContentService`로 옮기고, 파이프라인의 `Saver` 인터페이스에는 콘텐츠 서비스를 연결한다. 저장소는 서비스 구성 시에만 전달한다.
- 결과: 자동 콘텐츠 수집은 계속 비활성 상태다. 향후 재연결 시 수집 파이프라인은 서비스를 통해 저장하며 기존의 중복 건너뛰기와 개별 실패 집계 동작을 유지한다.

## ADR-059: 기능별 호출 경계와 서버 초기화 구조 단순화

- 날짜: 2026-09-26
- 상태: 설계 방향 승인, Redis 경계 1단계 완료 (ADR-060); 2026-09-30 서비스 2계층·생성자·하위 계층 규칙 보강, 2·3단계는 세분화 순서 A~E로 진행 중
- Redis 저장 구현을 모으고 Quiz·Study의 처리 책임과 호출 범위를 정리한다. 서버 객체 생성·주입은 `cmd/server`에서 한 번 연결하고 실행·종료 흐름과 구분한다.
- 상세: [ADR-059_architecture_simplification.md](ADR-059_architecture_simplification.md)

## ADR-060: Redis 저장 구현과 소비자 기능 계약을 분리한다

- 날짜: 2026-09-26
- 상태: 승인됨, 구현 완료
- 배경: 서비스·Bot·Mini App·Scheduler가 Redis 명령과 키·직렬화를 직접 다뤄 같은 저장 계약을 여러 패키지에서 알아야 했다. ADR-059의 1단계와 기존 Redis 경계 TODO를 실행한다.
- 결정:
  - `internal/redisstore`가 Quiz·Study JSON working set, 입력·초안·메시지 참조·복구 상태, 발송 claim의 Redis 구현을 소유한다. 키·값 형식·TTL과 `GetDel`·`SetNX` 사용을 유지한다.
  - 서비스는 타입이 있는 세션 저장 계약으로 읽기·저장·삭제를 요청한다. 세션 ID·버전 검증과 상태 전이는 서비스에 남긴다. 저장 구현의 누락·손상·의존성 오류는 공통 상태 오류로 전달하고 기존 서비스 오류로 변환한다.
  - Bot에는 입력·어순 초안·메시지·복구·시간 기록 계약을 각각 주입한다. Mini App에는 메시지 조회 계약만, Scheduler에는 발송 claim 계약만 주입한다. 문자열 저장 포맷 대신 타입이 있는 값을 주고받는다.
  - `cmd/server`에서 같은 Redis 연결로 각 저장 구현을 생성한다. 연결 생성·종료와 health Ping은 서버에 남긴다. `service.NewServices`의 저장소·외부 클라이언트 조립과 서버 초기화 구조 전체 변경은 후속 단계로 남긴다.
- 결과: 저장 형식은 `redisstore`에서 확인하고, 소비자는 사용하는 기능만 알게 된다. 기존 Redis 데이터를 그대로 읽으며 별도 마이그레이션·초기화는 하지 않는다. malformed JSON 삭제와 오류 전달, one-shot 입력 소비, 발송 오류 시 fail-open 및 기존 claim 유지 정책을 보존한다.
- 검증: 전체 `make test` 통과, 소비자 4개 패키지의 raw Redis·키 참조 0건, 앱 재시작 후 `/health` 정상. 전용 DB 환경변수가 없는 PostgreSQL 통합 테스트 6개는 건너뛰었다.
- 작업 기록: [2609262045_redis_access_boundary.md](../workthrough/2609/2609262045_redis_access_boundary.md)
- 후속 정리 (2026-09-27): 같은 패키지에서 외부용 세션 API와 공통 JSON 저장 구현, 입력 상태·어순 초안·Mini App 상태를 역할별 파일로 나눴다. 키·TTL·동작·생성자와 주입 구조는 유지하며 새로운 패키지나 객체는 추가하지 않는다.
- 저장 타입 단순화 (2026-09-27): 상태 타입과 키만 다른 `QuizSessions`·`StudySessions` 포장 타입 및 전달 메서드를 제거한다. 공통 `SessionStore[T]`가 소비자의 `Load/Save/Delete` 계약을 직접 구현하고, 기존 이름의 생성 함수는 각 상태 타입과 키를 지정한다. 학습 규칙·상태 모델·Redis 저장 형식은 구분을 유지한다.
- 세션 저장소 생성 조건 (2026-09-27): Redis 클라이언트 생성·연결 확인은 서버의 `initRedis`에서 보장한다. 실패하면 기존 `initInfra` → `run` → `main` 오류 처리로 서버 시작을 중단한다. 세션 저장소의 생성자와 `Load/Save/Delete`는 정상 주입을 전제로 하며 별도 nil 검사를 하지 않는다. `NewQuizSessions`·`NewStudySessions`가 기존 `sessionRedis` 인터페이스를 직접 받아 운영과 테스트에서 같은 생성자를 사용하고, 테스트용 비공개 생성자는 제거한다. 실제 Redis 명령 실패는 error로 전달하며 `SessionStore`의 zero value는 지원하지 않는다.
- 세션 종류를 드러내는 이름 (2026-09-27): Quiz 전용 진행 상태·서비스·저장소와 관련 필드·지역 변수·오류에는 `QuizActiveSession`을 사용해 기존 `StudyActiveSession`과 구분한다. Quiz 완료 결과도 `QuizSessionResult`·`QuizSessionWrongAnswer`로 명시한다. 관련 소스·테스트 파일명과 호출부·문서 참조를 함께 정리하며, Redis 키·JSON 필드·상태 버전·DB 스키마는 이름 변경 대상이 아니다.
