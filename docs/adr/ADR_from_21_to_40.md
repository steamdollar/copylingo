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
  - Material Seed는 `cmd/ja/material_seeder`가 독립적으로 수행한다. 기존 Kana/Vocab Question Seeder는 Material을 변경하지 않는다.
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
