# CopyLingo 의사결정 기록 (ADR)

## ADR-021: Application Log는 Context 기반 JSONL Structured Logging으로 기록

- **날짜**: 2026-06-01
- **상태**: 채택됨
- **맥락**:
  - 기존 로그는 Standard Library `log.Printf` 문자열이 여러 경계 계층에 흩어져 있다.
  - Telegram Update, Mini App HTTP 요청, Scheduler job에서 발생한 하위 로그를 하나의 상호작용 단위로 추적하기 어렵다.
  - 현재 운영 단계에서는 중앙 로그 수집기보다 로컬에서 직접 조회 가능한 일별 파일이 우선 필요하다.
- **결정**:
  - Standard Library `log/slog` JSON Handler를 사용한다. 외부 logger Dependency는 추가하지 않는다.
  - 로그는 stdout과 `logs/copylingo-YYYY-MM-DD.jsonl`에 동시에 기록한다.
  - 파일명과 JSON timestamp는 기본적으로 `Asia/Seoul` 기준이며, 30일이 지난 규약 파일은 자동 삭제한다.
  - HTTP 요청, Telegram Update, Scheduler job 진입점에서 `interaction_id`를 생성하고 `context.Context`로 하위 레이어에 전달한다.
  - 파일 sink 장애는 stderr 경고 후 stdout-only로 degrade한다. Application Log 보존 실패 때문에 서비스 전체를 중단하지 않는다.
  - 숫자 식별자는 기록할 수 있지만 token, Telegram `init_data`, 사용자 답안 원문, stroke 좌표는 기록하지 않는다.
  - 파일 로그는 장애 분석용이며 DB 상태나 Audit Log의 SSOT로 사용하지 않는다.
- **장점**:
  - 외부 Dependency 없이 건별 correlation과 JSON 기반 조회가 가능하다.
  - stdout을 유지하므로 Docker logging driver 또는 향후 중앙 collector로 전환하기 쉽다.
  - 파일 sink 장애와 서비스 가용성을 분리한다.
- **단점 / 트레이드오프**:
  - 단일 서버 파일은 수평 확장 환경에서 통합 조회가 어렵다.
  - 파일 cleanup과 rotation 책임이 애플리케이션에 추가된다.
  - 일부 기존 `log.Printf`는 점진 전환 기간 동안 `legacy.log` event로 남는다.
- **대안**:
  - Uber `zap`: 고빈도 logging 성능은 우수하지만 현재 로그량에서 외부 Dependency 비용 대비 실익이 작아 기각.
  - stdout-only + Docker logging driver: 운영은 단순하지만 로컬에서 일별 파일을 직접 조회하려는 요구를 충족하지 못해 기각.
  - DB Audit Log: 상태 변경 이력 보존에는 적합하지만 이번 요구는 장애 분석용 Application Log이므로 별도 범위로 분리.

---

## ADR-022: Study Material은 Question과 분리된 SSOT로 관리

- **날짜**: 2026-06-02
- **상태**: 채택됨 (Task 1 완료, Session 연결은 후속 Task)
- **맥락**:
  - 기존 앱은 `questions`를 직접 Seed하고 출제하므로 사용자가 Quiz 전에 학습 개념을 익히는 흐름이 없다.
  - 같은 단어도 객관식, 입력, 손글씨 Question으로 중복 표현된다.
  - `contents`는 NHK 등 외부 수집 원문이며, 가나와 단어처럼 코드로 Seed하는 학습 단위와 lifecycle이 다르다.
- **결정**:
  - `materials` 테이블을 Study Module의 학습 단위 SSOT로 추가한다.
  - `materials.content_id`는 nullable FK로 둔다. 코드 Seeder 기반 Material은 외부 원문 없이 존재할 수 있다.
  - 카드 형태 차이는 `payload JSONB`로 수용하고, 공통 검색 조건은 `category`, `language`, `proficiency_level`, `difficulty` 컬럼으로 유지한다.
  - `material_key`는 `{language}:{domain}:{stable_slug}` 형식의 Business Key이며 UNIQUE Constraint로 Idempotent Upsert를 보장한다.
  - Kana Slug는 Romaji 중복을 피하기 위해 Unicode code point를 사용한다. 예: `ja:kana:u3042`.
  - Vocabulary Slug는 Level을 제외한 안정 ID를 사용한다. 예: `ja:vocab:word_024`.
  - JA Seed는 `cmd/ja/seeder`가 Material 생성 후 Question 생성을 한 흐름에서 수행한다.
  - 초기 Material Seed 범위는 Vocabulary로 제한한다. Kana, Grammar, Sentence Material은 Study UX 도입 순서에 맞춰 후속 추가한다.
  - 기존 Quiz Question과 SRS 구조는 Task 1에서 변경하지 않는다.
- **장점**:
  - Study 개념과 Quiz 표현을 분리하여 향후 하나의 Material에 여러 Question 유형을 연결할 수 있다.
  - Seeder 재실행으로 Material이 중복되지 않는다.
  - `contents` 수집기 상태와 무관하게 기초 학습 Material을 운영할 수 있다.
- **단점 / 트레이드오프**:
  - Study Session 연결 전까지 `materials`는 Quiz 흐름에서 직접 사용되지 않는다.
  - `payload JSONB` 구조는 Category별 Consumer가 명시적으로 해석해야 한다.
  - 초기 Vocabulary Catalog는 기존 Question Seeder 데이터와 일부 중복된다. 두 Seeder의 lifecycle 격리를 우선하고, 중복 변경 비용이 커질 때 공용 Catalog를 재검토한다.
- **대안**:
  - `contents` 재사용: 외부 원문과 기초 학습 개념의 lifecycle이 섞여 기각.
  - `questions` 재사용: Quiz 표현 중복 때문에 Study 진도 SSOT로 부적합하여 기각.
  - `material_key` 없이 SERIAL PK만 사용: Seeder Idempotency를 보장하기 어려워 기각.

---

## ADR-023: Daily Session은 Vocabulary 슬롯을 최소 1/3 예약

- **날짜**: 2026-06-03
- **상태**: 채택됨
- **맥락**:
  - 기존 Random Slot Relay는 `kana`, `handwriting`, `vocabulary`, `grammar` 순서로 새 문항 슬롯을 랜덤 배분한다.
  - 앞 카테고리가 슬롯을 먼저 가져가고, 최종 fallback도 낮은 `difficulty`를 우선하므로 Vocabulary 노출량이 지나치게 낮아질 수 있다.
  - Kana 복습을 유지하면서도 실제 단어 학습 비중을 세션 전체의 `1/3 ~ 1/2` 이상으로 높일 필요가 있다.
- **결정**:
  - Morning, Evening Daily Session은 총 문제 수의 `ceil(1/3)` 슬롯을 신규 Vocabulary Question에 먼저 예약한다.
  - 예약 Vocabulary 재고가 부족하면 기존 Random Slot Relay와 전체 fallback이 빈 슬롯을 채운다.
  - 총 문제 수를 유지하기 위해 review 상한은 예약 Vocabulary 슬롯을 제외한 수로 제한한다.
  - Morning Session은 기존 `review 6 + new 9` 구성을 유지한다.
  - Evening Session은 `review 8 + new 2`에서 최대 `review 6 + vocabulary 4` 중심 구성으로 변경한다.
  - Review 전용 Session은 Vocabulary 예약 정책을 적용하지 않는다.
- **장점**:
  - Daily Session마다 Vocabulary 최소 노출량을 예측할 수 있다.
  - Vocabulary 재고 부족 시 세션 자체가 비지 않고 기존 카테고리로 degrade한다.
  - Repository Interface와 Schema를 변경하지 않는다.
- **단점 / 트레이드오프**:
  - Evening Session에서 한 번에 처리하는 SRS due review 수가 최대 8개에서 6개로 줄어든다.
  - 신규 Vocabulary 재고가 부족하면 최소 1/3 비율은 보장되지 않는다.
  - 사용자별 학습 목적에 따른 조합 선택은 아직 지원하지 않는다.
- **대안**:
  - 새 문항 슬롯의 절반만 Vocabulary로 예약: Evening Session 전체의 1/3을 보장하지 못해 기각.
  - Relay 순서만 Vocabulary 우선으로 변경: 최소 비율을 명시적으로 보장하지 못해 기각.
  - 기존 Random Slot Relay 유지: Kana 편중 문제가 지속되어 기각.

---

## ADR-024: Study Session은 sessions 공통 Parent와 session_materials Child로 관리

- **날짜**: 2026-06-06
- **상태**: 채택됨
- **맥락**:
  - Study Module은 `materials`를 학습 카드 SSOT로 추가했지만, 실제 사용자 session lifecycle에는 아직 연결되지 않았다.
  - Quiz Session과 Study Session은 payload child가 다르지만, 사용자에게 Push되고 시작/완료되는 lifecycle은 같다.
  - 기존 `sessions`에는 `user_id`, `type`, `status`, `started_at`, `completed_at`, `created_at` 등 공통 lifecycle 컬럼이 이미 있다.
- **결정**:
  - 별도 `study_sessions` parent table을 만들지 않고 기존 `sessions`를 Quiz/Study 공통 parent로 유지한다.
  - `sessions.mode VARCHAR(20) NOT NULL`를 추가해 interaction model을 `quiz`와 `study`로 구분한다.
  - `mode` 값은 DB default에 기대지 않고 application layer의 `model.SessionMode` enum으로 명시적으로 세팅하고 검증한다.
  - Study child table로 `session_materials`를 추가한다.
  - Material 반복 학습 상태는 `user_material_progress`에 user별 SRS state로 저장한다.
  - `session_materials.session_id`는 `sessions(id) ON DELETE CASCADE`를 사용한다. Session이 삭제되면 child progress row는 독립 의미가 없다.
  - `session_materials.material_id`는 `materials(id)` FK를 두되 CASCADE를 사용하지 않는다. Material은 catalog SSOT라 session 삭제 lifecycle에 종속되지 않는다.
  - Study 진행 상태는 진행 중에는 Redis Working Set에 저장하고, 완료 시 DB에 한 번에 flush한다.
  - Quiz `ActiveSessionService`와 Study `StudyActiveSessionService`는 domain service를 분리하되, Redis get/save/delete 공통 로직은 generic `workingSetStore[T]`로 공유한다.
  - Study card 이동 callback은 Redis의 `StudyActiveSessionState`만 갱신한다. `session_materials.studied_at`, `sessions.status`, `user_material_progress`는 finish 시 transaction으로 반영한다.
  - Redis miss 또는 재시작 복구 시에는 DB의 `sessions`와 `session_materials`에서 Study Working Set을 재구성한다.
  - DB flush는 이미 completed 된 session이면 material/progress 갱신을 건너뛰어 중복 finish callback에도 progress를 재증가시키지 않는다.
  - 정오 Study Session 생성은 due Material(`next_review_at <= NOW()`)을 우선하고, 부족한 슬롯은 신규 Material(`progress row 없음`)로 채운다.
  - 정오 Scheduler는 사용자 `language`, `proficiency_level`에 맞는 Material로 `mode='study'`, `type='study'` session을 생성하고 Telegram으로 Push한다.
- **장점**:
  - Session lifecycle, Scheduler, Telegram push 관점을 공통 parent로 재사용할 수 있다.
  - Quiz child(`session_questions`)와 Study child(`session_materials`)가 분리되어 payload별 책임이 명확하다.
  - `sessions.mode`로 기존 Quiz flow가 Study session을 잘못 ActiveSession으로 로드하는 문제를 차단할 수 있다.
  - Study card마다 DB write를 하지 않으므로 callback 진행 중 DB 부하와 write amplification을 줄일 수 있다.
  - Redis Working Set 공통 로직을 공유해 Quiz/Study의 cache lifecycle 오류 처리와 corrupt state 삭제 정책을 일관되게 유지할 수 있다.
  - 완료 시 transaction flush로 `sessions`, `session_materials`, `user_material_progress`의 상태 전이를 한 경계에 묶을 수 있다.
  - 기존 Question SRS는 `questions` row 자체에 state가 있어 user별 SRS가 아니다. Material SRS는 처음부터 user별 progress table로 분리해 다중 사용자 확장성을 확보한다.
- **단점 / 트레이드오프**:
  - `sessions.total_questions`, `correct_count` 명칭은 Study에는 어색하다. 현재는 Breaking Change를 피하고 Study의 material count를 `total_questions`에 저장한다.
  - Study와 Quiz의 완료 의미가 달라 향후 analytics에서 mode별 aggregation 분기가 필요하다.
  - 하나의 parent table에 여러 mode가 공존하므로 repository query는 mode 조건을 명시해야 한다.
  - Redis Working Set이 유실되면 마지막 DB flush 이전 card 진행 상태는 복구되지 않는다. 현재는 DB 상태 기준으로 미학습 카드부터 재개한다.
  - 완료 직전까지 DB에는 card별 `studied_at`이 반영되지 않으므로 실시간 학습 진행률 analytics는 Redis 또는 별도 event stream을 봐야 한다.
  - `user_material_progress`는 Material용 SRS를 먼저 도입하므로, 향후 Question SRS도 `user_question_progress`로 분리하는 후속 설계가 필요하다.
- **대안**:
  - 별도 `study_sessions` parent table: Study 도메인 컬럼명은 깔끔하지만 status/start/complete/push/history 로직이 중복되고 전체 학습 timeline 조회가 UNION 중심이 되어 기각.
  - `materials`를 Telegram 메시지로만 Push: 구현은 빠르지만 session 기록, idempotency, 완료 추적이 없어 기각.
  - `materials` table에 SRS 컬럼 추가: Question SRS와 비슷하지만 user별 반복 상태를 표현할 수 없어 기각.
  - `session_materials` 이력만으로 least-seen/oldest-seen 정렬: schema 추가 없이 반복 노출은 가능하지만 interval/ease factor 기반 Review Scheduling이 없어 기각.
  - Study card마다 DB write: 구현은 단순하지만 card 수와 사용자 수가 늘 때 callback path에서 N write가 발생해 기각.
  - Quiz `ActiveSessionService`를 Study에도 직접 재사용: Redis lifecycle은 공유할 수 있지만 answer scheduling, correct count, question payload 의미가 달라 domain coupling이 커져 기각.
  - `session_materials`에 FK를 두지 않음: sharding/MSA 환경에서는 선택될 수 있으나 현재 단일 Postgres SSOT에서는 orphan row 리스크가 더 커 기각.

## ADR-025: 코드 스타일은 golangci-lint v2로 강제하고 라인 폭 정리는 commit-time에 자동화

- **날짜**: 2026-06-12
- **상태**: 채택됨
- **맥락**:
  - `make lint`는 golangci-lint를 호출했지만 `.golangci.yml` 설정도 바이너리도 없어 사실상 동작하지 않았다.
  - 라인이 옆으로 과하게 퍼지는 것을 막고 싶지만, gofmt/goimports/gopls는 라인 길이 줄바꿈을 하지 않는다.
  - 포매팅을 위해 매번 수동 명령을 실행하는 것은 누락되기 쉽다. "코드만 작성하면 자동 정리"가 요구사항이었다.
- **결정**:
  - golangci-lint **v2** 스키마(`.golangci.yml`)를 SSOT로 두고 `linters`(진단)와 `formatters`(자동 적용)를 분리한다.
  - 라인 길이 기준은 **120자**로 통일한다. `golines` 포매터가 120자 초과 라인을 자동 줄바꿈하고, golines가 못 줄이는 잔여분(긴 문자열 리터럴 등)은 `lll` 린터가 보고만 한다.
  - 자동화 시점은 **save-time이 아니라 commit-time**으로 한다. git pre-commit hook(`scripts/git-hooks/pre-commit`)이 staged `.go`를 `golangci-lint fmt` 후 재-stage한다. hook은 레포에 커밋하고 `make hooks`(`core.hooksPath`)로 활성화한다.
  - 린터는 실무 표준 세트(standard + revive, gocritic, gocyclo, misspell, errorlint, bodyclose, unconvert, nakedret, nolintlint, lll)를 적용한다. 테스트 파일은 길이/복잡도/에러체크 룰을 완화한다.
- **장점**:
  - 라인 폭 정리가 사람 손과 무관하게 commit마다 일관 적용된다.
  - hook과 설정이 레포에 있어 에디터·OS 독립적이고 재현 가능하다 (실무 CI/팀 환경 가정).
  - `errorlint`가 프로젝트의 `%w` 에러 래핑 규약(CLAUDE.md §5)을 정적으로 보강한다.
  - 포매팅을 gopls의 save-time 동작에 묶지 않아 golines 같은 비-gopls 포매터도 안정적으로 적용된다.
- **단점 / 트레이드오프**:
  - `core.hooksPath`는 로컬 git 설정이라 새 클론 환경마다 `make hooks` 1회가 필요하다.
  - 포맷이 commit 시점에만 적용되므로 작성 중 에디터에서는 긴 라인이 그대로 보인다(save-time보다 피드백이 늦음).
  - hook은 staged 파일을 working-tree에서 포맷 후 add하므로, 같은 파일의 unstaged 변경이 함께 stage될 수 있다(부분 커밋 시 주의).
- **대안**:
  - VSCode save-time 포맷(formatTool→golines / Run-on-Save 확장): 피드백은 즉각적이나 설정이 개인 PC에 종속되고 에디터 의존성이 커 기각.
  - `lll`만 적용(보고 전용): 위반을 알려주지만 자동 수정이 없어 "치면 자동" 요구를 충족하지 못해 기각.
  - pre-commit/lefthook 프레임워크: 기능은 풍부하나 외부 도구 의존성이 늘어 단순 git hook + Makefile로 충분하다 판단해 기각.

---

## ADR-026: Question Type은 풀이 방식으로 유지하고 Item Type으로 JLPT Taxonomy를 분리

- **날짜**: 2026-06-13
- **상태**: 채택됨
- **맥락**:
  - `questions.type`은 기존 데이터와 코드에서 `multiple_choice`, `fill_blank`, `subjective`, `kana_handwriting`처럼 Telegram rendering 및 grading mode로 사용된다.
  - JLPT N1까지 확장하려면 `kanji_reading`, `vocab_usage`, `text_grammar`, `reading_integrated`, `listening_key_point` 같은 공식 문항 taxonomy도 별도로 추적해야 한다.
  - `questions.type`에 JLPT taxonomy를 직접 넣으면 `SessionFlow.renderByType`, `GraderService`, `HandwritingService`의 hot path와 기존 seed data가 깨진다.
- **결정**:
  - `questions.type`은 하위호환을 위해 풀이/채점 방식(answer/rendering mode)으로 유지한다.
  - `questions.item_type VARCHAR(64)`를 nullable column으로 추가해 실제 측정 skill taxonomy를 기록한다.
  - Go model에는 `model.Skill` enum을 추가한다.
  - 초기 taxonomy는 현재 앱 내부 학습 유형(`kana_reading`, `kana_recall`, `kana_handwriting`, `vocab_meaning`, `vocab_recall`, `vocab_handwriting`)과 JLPT official-style 유형을 함께 포함한다.
  - 기존 DB row는 현재 `type + category` 조합으로 의미가 명확한 6개 조합만 backfill한다. 의미가 불명확한 row는 향후에도 nullable 상태를 허용한다.
- **장점**:
  - 기존 Telegram rendering, grading, handwriting flow를 변경하지 않고 JLPT taxonomy를 추가할 수 있다.
  - 향후 세션 배분은 broad `category`, 풀이 방식은 `type`, 세부 측정 skill은 `item_type`으로 각각 분리해 볼 수 있다.
  - N1 문제 생성기에서 official skill별 prompt/template/평가 통계를 만들 수 있다.
- **단점 / 트레이드오프**:
  - `type`, `category`, `item_type` 세 축을 구분해야 하므로 model 설명과 generator discipline이 중요해진다.
  - `item_type`을 nullable로 시작하므로 analytics에서 `NULL` guard가 필요하다.
  - 현재 SRS는 question row 자체에 묶여 있어 skill별 사용자별 숙련도 추적은 별도 `user_question_progress` 설계가 필요하다.
- **대안**:
  - `questions.type` 하나에 모든 taxonomy를 통합: schema는 단순하지만 기존 rendering/grading 의미와 충돌해 기각.
  - `questions.type`을 JLPT taxonomy로 재정의하고 기존 데이터를 backfill: 현재 `multiple_choice/kana` 같은 row에서 taxonomy를 완전히 복원할 수 없어 기각.
  - `category`만 세분화: broad analytics 축을 잃고 vocabulary/grammar/reading 같은 상위 도메인 필터링이 흐려져 기각.

---

## ADR-027: Task 4는 Seed Source 정리 후 Question-Material FK를 재생성

- **날짜**: 2026-06-15
- **상태**: 채택됨
- **맥락**:
  - Study Module Task 1~3에서 `materials`, `session_materials`, `user_material_progress`, Study Session Push가 구현됐다.
  - 다음 단계는 Study Material과 Quiz Question의 관계를 연결하는 것이다.
  - 기존에는 Vocabulary Material catalog와 Vocabulary Question catalog가 서로 다른 seeder에 분리되어 있었다. Material은 500개, Question은 100개 기준이라 장기적으로 drift가 생길 수 있었다.
  - Backfill command로 기존 row를 보정할 수는 있지만, seed source 자체가 계속 갈라져 있으면 같은 문제가 반복된다.
- **결정**:
  - `questions.material_id INT NULL REFERENCES materials(id) ON DELETE SET NULL`을 추가한다.
  - `questions.material_id`에는 partial index를 둔다.
  - Seed Question 식별을 위해 `questions.question_key VARCHAR(255) UNIQUE`를 추가한다.
  - JA seed catalog는 `cmd/ja`로 통합한다.
  - `cmd/ja`는 Kana map, N5 Vocabulary 500개, Kana/Vocabulary Material builder, stable `material_key` helper의 SSOT다.
  - JA Seeder는 Kana Material과 Vocabulary Material을 모두 upsert한 뒤 `material_key`로 Material을 조회한다.
  - JA Seeder는 기존 Kana/Vocabulary Question 생성 유형을 유지하고 생성 Question에 `material_id`와 stable `question_key`를 저장한다.
  - JA Seeder는 deterministic generation과 `ON CONFLICT (question_key) DO UPDATE`로 재실행 멱등성을 보장한다.
  - 기존 Backfill command는 제거한다.
  - 로컬 재생성은 `users`를 보존하고 학습 데이터 테이블을 reset한 뒤 Material, Question, pending Study Session을 다시 생성한다.
  - `user_question_progress` 도입, Quiz SRS user별 분리, Study 완료 Material의 Evening Quiz 편성 반영은 후속 Task로 분리한다.
  - 기존 `questions` SRS 컬럼과 `SessionBuilderService` 편성 정책은 이번 Task에서 변경하지 않는다.
- **장점**:
  - Study Material과 Quiz Question의 추적 관계를 seed 시점부터 보장한다.
  - Material/Question catalog drift를 줄인다.
  - Backfill match 조건에 의존하지 않는다.
  - 기존 Quiz/Study Session hot path와 SRS 동작을 건드리지 않아 회귀 리스크가 낮다.
  - 후속 Study 기반 Quiz 편성 작업에서 사용할 명확한 FK 기반이 생긴다.
- **단점 / 트레이드오프**:
  - 현재 Quiz SRS가 `questions` row에 저장되는 다중 사용자 한계는 남는다.
  - `material_id`만 연결해도 Study 완료 Material이 자동으로 Quiz에 출제되지는 않는다.
  - 로컬 reset은 기존 sessions/progress/contents/tips를 삭제하므로 운영 데이터에는 그대로 적용할 수 없다.
  - Question Seeder는 아직 idempotent upsert가 아니라 reset 없는 재실행 시 중복 row를 만들 수 있다.
- **대안**:
  - `user_question_progress`까지 즉시 도입: 장기 구조는 더 정확하지만 SRSService, Grader, SessionBuilder 영향 범위가 커져 이번 Task에서는 기각.
  - `question_materials` join table 도입: 다중 Material 문항 확장성은 있으나 현재 Vocabulary 1문항-1Material 관계에는 과해 기각.
  - Backfill command 유지: 기존 row 보정에는 유용하지만 seed source drift를 근본적으로 해결하지 못해 기각.
  - Do nothing: Study와 Quiz 사이의 추적 기반이 계속 없어져 기각.

---

## ADR-028: Telegram `/llm` 질문은 owner user ID 배열과 Tip Candidate로 수집

- **날짜**: 2026-06-21
- **상태**: 채택됨
- **맥락**:
  - Telegram에서 즉석으로 LLM에게 언어 학습 질문을 하고 싶다.
  - 질문/답변은 이후 유사 레벨 사용자에게 재사용 가능한 tip 후보가 될 수 있다.
  - 단, 현재 기능은 개인 owner 워크플로우용이며 임의 사용자에게 LLM 비용과 저장 경로를 열면 abuse surface가 생긴다.
- **결정**:
  - `/llm` command로 Redis 기반 1회성 LLM mode를 활성화한다.
  - 다음 plain text 1개만 LLM 질문으로 라우팅하고, 처리 후 pending key를 삭제한다.
  - 접근 제어는 코드 내 배열 `config.LLMAllowedTelegramUserIDs`로 한다. 배열에 포함되지 않은 user ID면 LLM 호출과 DB write를 하지 않는다.
  - 질문/답변은 `tip_candidates` 테이블에 저장한다. 저장 필드는 `user_id`, `username`, `language`, `proficiency_level`, `question`, `answer`, `source_model`, `created_at`이다.
  - `tips`와 분리해 raw candidate와 노출 가능한 curated tip의 lifecycle을 분리한다.
  - pass-through service/repository는 만들지 않는다. LLM 호출은 `external.LLMClient`를 감싼 `LLMService`, candidate 저장은 기존 Tip domain의 `TipService`/`TipRepository`가 담당한다.
- **장점**:
  - LLM 호출 전 owner user ID 배열 membership check로 비용/abuse를 차단한다.
  - user의 `language`, `proficiency_level`을 함께 저장해 향후 비슷한 레벨 tip 추천/큐레이션에 사용할 수 있다.
  - Redis 1회성 state라 대화 mode가 오래 남아 일반 답변/세션 답변을 계속 가로채지 않는다.
  - `tips` 테이블 오염 없이 후보 데이터를 따로 쌓을 수 있다.
- **단점 / 트레이드오프**:
  - 허용 user 변경 시 코드 변경이 필요하다. 대신 설정 파싱/환경변수 관리 코드는 없다.
  - Redis pending state가 TTL 전에 유실되면 사용자는 `/llm`을 다시 입력해야 한다.
  - 후보 저장 실패는 사용자 답변 제공을 막지 않고 로그만 남긴다. candidate 수집 완전성은 로그/DB 모니터링이 필요하다.
- **대안**:
  - env allowlist: 설정 기반으로 더 유연하지만, 현재 단일 owner 기능에는 config/parsing/test 코드가 과해 기각.
  - `tips`에 바로 저장: raw 질문/답변과 curated tip이 섞여 Mini App 노출 품질을 해쳐 기각.
  - DB flag 기반 권한: 다중 admin 관리에는 좋지만 현재 범위에서는 schema/운영 절차가 커져 기각.

---

## ADR-029: Quiz 결과 메시지에 문제 원문 보존 + 문제 컨텍스트 LLM 질문 버튼

- **날짜**: 2026-06-28
- **상태**: 채택됨
- **맥락**:
  - Quiz 답변 시 원본 문제 메시지를 `editMessage`로 덮어써 결과(정답/해설)를 보여주는데, 결과 텍스트에 문제 원문(`prompt`)이 빠져 있어 사용자가 "무슨 문제였는지"를 다시 볼 수 없다.
  - 그 자리에서 바로 LLM에게 그 문제를 물어보고 싶지만, 기존 `/llm`(ADR-028)은 quiz context를 몰라 owner가 문제 원문을 직접 재입력해야 한다.
  - 모든 답변마다 자동 LLM 호출은 비용/UX상 과하다 — "필요할 때만" 호출돼야 한다.
- **결정**:
  - 결과 메시지(`processAnswerText`)에 문제 원문 `📝 {prompt}`를 정답/오답 공통으로 prepend한다. 해설은 이미 `question.Explanation`으로 표시 중이므로 별도 저장/변경 없음.
  - 결과 메시지에 `🤖 이 문제 질문` inline button을 추가한다. callback은 기존 `q:` prefix의 sub-action `q:{sessionID}:ask:{questionID}`(`config.FormatQuestionAskLLM`)을 쓴다 (`next` sub-action과 동일 패턴).
  - 버튼은 ADR-028과 동일하게 `config.LLMAllowedTelegramUserIDs` owner에게만 렌더하고, callback 처리 시에도 재검증한다 (비용/abuse gate).
  - 버튼을 누르면 기존 `/llm` pending 메커니즘을 재사용하되, `UserLLMPendingRedisKey` 값에 `q:{sessionID}:{questionID}` 컨텍스트 토큰을 저장한다 (plain `/llm`은 기존처럼 `"1"`). 다음 plain text 1개를 Redis active session에서 그 문제(원문/정답/해설/사용자 답)로 로드해 LLM 프롬프트에 실어 답한다.
  - `external.LLMClient` 인터페이스/`AnswerLearningQuestion` 시그니처는 바꾸지 않는다. 컨텍스트 프롬프트는 bot 레이어에서 조립해 기존 메서드로 전달한다 (mock/test 영향 0).
- **장점**:
  - 원문 소실 문제를 한 줄로 해결, 해설은 이미 있던 것을 그대로 활용.
  - "이어지는 flow" 버튼은 누를 때만 LLM을 호출 → "필요할 때만"을 정확히 충족.
  - 기존 `llm_pending` 라우팅·owner gate·tip candidate 수집을 재사용해 새 machinery 최소(콜백 sub-action 1종 + pending 값 확장).
- **단점 / 트레이드오프**:
  - pending 값이 `"1"` 외에 `q:sid:qid` 형식을 가지면서 값 파싱 분기가 생긴다 (단순 prefix 검사로 처리).
  - active session이 Redis에서 만료되면(pending TTL 10분 내라 드묾) context 로드가 실패 → context 없이 일반 답변으로 graceful fallback.
  - 문제별 LLM 질문도 owner gate에 의존하므로, 다중 사용자 확장 시 per-user rate-limit/비용 통제가 별도로 필요하다 (현 범위 밖, 향후 과제).
- **대안**:
  - 새 callback prefix(`llmq:`) 도입: handler dispatch/로깅 분기를 추가해야 해 기각, 기존 `q:` sub-action 재사용이 더 일관적.
  - `LLMClient`에 `AnswerQuestionInContext` 전용 메서드 추가: 의미는 더 명확하나 인터페이스 변경으로 mock/test가 광범위하게 깨져 현 범위엔 과해 기각.
  - editMessage 폐기(문제 메시지 보존 후 결과를 새 메시지로): 채팅 정리를 위한 edit-in-place 설계(SessionFlow editMessageID, ADR 기록)를 뒤집어야 해 기각.

## ADR-030: Grammar 예문 읽기는 정적 catalog에 미리 계산된 전문 かな 필드로 보강

- **날짜**: 2026-06-30
- **상태**: 채택됨
- **맥락**:
  - Grammar Study Material의 예문(`example`: 私は学生です。)·cloze(`cloze_prompt`)은 한자 포함인데 읽기(furigana)가 없어 N5 학습자가 문장을 못 읽는다. Vocabulary는 이미 `kana` 필드가 `읽기:`로 렌더돼 동일 문제가 없다.
  - Telegram Bot API HTML 허용 태그에 `<ruby>`가 없어 한자 위 첨자형 진짜 후리가나는 불가 — 별도 줄이나 인라인 괄호만 가능.
  - 한자 읽기는 정적 catalog(grammar 80 / vocab_context cloze)과 동적 AI 콘텐츠(Phase 2.4 아티클)라는 성격이 다른 두 모집단에 걸쳐 있다.
- **결정**:
  - Grammar 예문 읽기를 **전문 ひらがな(가타카나·문장부호는 원형 유지) 단일 필드 `example_reading`** 으로 표현한다. 인라인 괄호 furigana(B안)·런타임 형태소 분석(C안)을 기각하고 vocab의 `읽기:` 줄과 동일한 별도 줄 패턴을 따른다.
  - 정적 catalog는 **읽기를 시드 타임에 미리 계산해 `Material.Payload`(JSONB)에 박는다.** 별도 DB 칼럼/마이그레이션 없이 catalog JSON(`n5_grammar.json`) → `GrammarPoint`/`GrammarMaterialPayload` 필드 추가로 흐른다. 렌더는 `renderGrammarPayload`에서 예문 바로 아래 `읽기:` 줄을 비-bold로 추가한다.
  - 읽기 생성은 LLM 1회 배치(80문장, 외부 위임 임계 50k 토큰 미만 → 본 세션 처리)이되 오독은 틀린 읽기를 가르치므로 검증 후 확정. 데이터 무결성은 `datasets_test`의 필수 필드 검사에 `ExampleReading`을 추가해 80개 전수 강제.
  - **범위 한정**: 이번엔 grammar 예문(study material)만. quiz `cloze_prompt`·vocab_context cloze 읽기는 별도 과제로 남긴다.
- **장점**:
  - 수만 유저 가정(§4)에서 정답: 고정 catalog는 렌더마다 분석기를 돌리는 대신 1회 계산 후 싸게 서빙. 무거운 형태소 사전 의존성(kagome 수십 MB) 회피.
  - vocab `읽기:`와 동일 UX·렌더 패턴으로 일관성, 최소 변경(struct 3곳 + 렌더 1줄 + JSON 80필드).
- **단점 / 트레이드오프**:
  - 전문 かな 줄은 한자↔かな 정렬 정보를 잃는다(어느 한자가 어느 읽기인지 비명시). N5 단문에선 허용 가능, 정렬이 중요해지면 인라인 furigana로 승급 여지.
  - 읽기는 수기/LLM 생성이라 오독 리스크가 상존 → 무결성 테스트는 존재만 강제하고 정확성은 보장 못 함(리뷰 의존).
- **대안 / 향후**:
  - 동적 AI 콘텐츠(아티클)는 본 결정과 분리: 런타임 furigana가 필요하므로 LLM 생성 파이프라인이 읽기를 함께 출력하게 하는 별도 설계가 맞다(현 범위 밖).
  - quiz cloze·vocab_context cloze 읽기 보강은 동일 `example_reading` 패턴 확장으로 후속 처리.

## ADR-031 · ADR-032 → 별도 파일

청해(Listening) 음성 생성·저장·전송 아키텍처는 결정 범위가 커서 range 파일에 인라인하지 않고 별도 파일로 분리했다.

- **ADR-031**: 청해 TTS = Gemini 2.5 native TTS 사전 생성 (OpenAI-compat 미지원 → native `generateContent`, PCM→OGG transcode)
- **ADR-032**: 청해 음성 = S3 호환 object storage(content-addressed dedup) + Telegram `file_id` 재전송 (MinIO 로컬 / S3-서울 prod, R2 기각)

→ [ADR-031_032_listening_audio_pipeline.md](ADR-031_032_listening_audio_pipeline.md)

---

## ADR-033: Vocabulary Kanji Recall은 FillBlank Skill로 추가하고 세션당 3개로 제한

- **날짜**: 2026-07-18
- **상태**: 채택됨
- **맥락**:
  - N5 Vocabulary Material은 `kana`, `kanji`, `meaning_ko`를 이미 가지지만, Quiz는 뜻→かな recall만 요구해 한자 표기를 직접 회상하는 문항이 없다.
  - `questions.type`은 rendering/grading mode, `item_type`은 측정 skill이라는 ADR-026 계약을 유지해야 한다.
  - 한자 직접 입력을 한 세션에 많이 노출하면 오답 밀집으로 학습 UX가 악화되므로 세션당 상한이 필요하다.
- **결정**:
  - 새 rendering/grading `QuestionType`은 추가하지 않고 exact-match text input인 `fill_blank`를 재사용한다.
  - 새 skill `vocab_kanji_recall`을 추가하고 category는 기존 Material과 같은 `vocabulary`로 유지한다.
  - 생성 대상은 `kanji != kana`이면서 Unicode Han 문자를 1개 이상 포함한 Vocabulary catalog row로 한정한다. Prompt에 한국어 뜻과 かな 읽기를 함께 보여 동음이의어 모호성을 줄이고, 정답은 catalog `kanji`로 한다.
  - Question key는 `ja:vocab:<dataset_id>:kanji_recall`로 고정하고, 기존 `UpsertSeedBatch` conflict update로 재실행 멱등성을 보장한다.
  - `vocab_kanji_recall`은 morning/evening/review 세션 전체에서 최대 3개다. Repository query가 남은 budget만큼만 candidate를 eligible로 만들어 non-kanji로 backfill하고, Session Builder admission gate가 최종 invariant를 다시 검증한다.
- **장점**:
  - 기존 Material SSOT·text input·exact grader·seed upsert를 재사용해 schema와 bot flow 변경 없이 학습 축을 확장한다.
  - Query-side budget이 `LIMIT` 앞에 적용되므로 많은 overdue 한자 문항이 non-kanji review를 밀어내는 starvation을 피한다.
  - Skill 단위 cap이라 기존 vocabulary 최소 1/3 예약 정책과 broad category analytics를 유지한다.
- **단점 / 트레이드오프**:
  - Repository·SRS·Session Builder 계약에 kanji budget parameter가 추가된다. 현재는 하나의 UX 상한만 있어 generic policy abstraction은 도입하지 않는다.
  - Exact match는 이체자·복수 표기·Unicode normalization을 허용하지 않는다. 현재는 catalog의 단일 SSOT 정답을 학습하는 문항이라 수용한다.
- **대안**:
  - 새 `QuestionType`: 풀이 방식은 기존 `fill_blank`와 동일해 ADR-026의 축 분리를 깨므로 기각.
  - `kanji_reading` skill 재사용: 한자→읽기와 뜻/읽기→한자는 측정 방향이 달라 analytics를 오염시키므로 기각.
  - Session Builder에서만 사후 filtering: repository `LIMIT` 결과가 한자 문항으로 채워지면 세션 부족과 overdue starvation이 발생해 기각.

---

## ADR-034: 초기 Listening 콘텐츠는 Question-only Quiz Seed로 검증하고 Material-first Study는 후속 결정으로 둔다

- **날짜**: 2026-07-20
- **상태**: 채택됨
- **맥락**:
  - ADR-031/032로 Gemini native TTS→S3/MinIO→Telegram voice 파이프라인은 구현됐지만 실제 listening 문항이 0건이라 live e2e가 미검증이었다.
  - Listening을 Study Material SSOT로 먼저 모델링하면 학습→Quiz 연결은 자연스럽지만, `materials` audio metadata와 Study renderer/lifecycle까지 새로 설계해야 해 샘플 5문항 검증 범위를 크게 넘는다.
- **결정**:
  - 초기 5문항은 `material_id = NULL`인 Question-only original N5 comprehension MCQ로 seed한다. 기출/교재를 복제하지 않고 가격·시간·장소·행동·순서 정보를 다룬다.
  - `type=listening`, `category=listening`, `audio_script`, 4개 options, exact-match `correct_answer`, listening skill을 정적 JSON에서 멱등 `question_key`로 upsert한다.
  - 이번 단계의 완료 기준은 5개 TTS 생성·object path 저장과 Telegram voice/MCQ/채점/`audio_file_id` cache의 live smoke다.
  - Listening Study Material SSOT와 Quiz `material_id` 연결은 이번 결정에 포함하지 않는다. 해당 기능을 추진할 때 audio asset 소유권과 Study 공개 UX를 별도 Case A로 결정한다.
- **장점**:
  - 기존 audio/render/grader pipeline을 그대로 검증해 신규 schema나 hot-path 변경 없이 end-to-end 공백을 닫는다.
  - original seed라 저작권 위험이 없고, JSON integrity test로 필수 필드·skill·난이도·4지선다·정답 포함을 고정한다.
- **단점 / 트레이드오프**:
  - 현재 5문항은 Study 이력 우선순위를 받지 못하며 Quiz에서 바로 출제된다.
  - 향후 Material-first 도입 시 content SSOT와 audio metadata 위치를 다시 정하고 seed를 연결해야 한다.
- **대안**:
  - Material-first를 즉시 구현: 장기 학습 흐름은 좋지만 이번 smoke에 Study domain/schema/renderer 변경까지 결합해 기각.
  - 기존 pipeline 단위 테스트만 유지: 실제 Gemini/ffmpeg/MinIO/Telegram 경계를 검증하지 못해 기각.
