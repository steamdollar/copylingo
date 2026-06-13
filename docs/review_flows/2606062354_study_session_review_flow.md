# Study Session Review Flow

## Scope

- 정오 Study Session 생성, Vocabulary Material 선택, Telegram push, Redis Working Set 기반 card 진행, 완료 시 DB flush 흐름을 한 번에 리뷰한다.
- Quiz Session과 Study Session이 `sessions.mode`로 분리되고, Redis lifecycle은 `workingSetStore[T]`로 일반화된 지점을 함께 확인한다.
- 최신 정책인 Vocabulary Material 8개, Kana 단일 문자 제외, card 이동 시 DB write 제거, 완료 시 transaction flush를 확인한다.

## Route Summary

| Category | Purpose |
|---|---|
| Trigger & Config | 정오 Study push가 어떤 설정과 Scheduler job에서 시작되는지 확인한다. |
| Schema & Models | Study Session의 DB contract와 Redis Working Set state shape를 확인한다. |
| Dependency Wiring | Repository/Service/Bot graph에 생성 경로와 active runtime 경로가 모두 연결되는지 확인한다. |
| Creation Path | Material 선택, `sessions` 생성, `session_materials` 생성까지의 DB write 경로를 확인한다. |
| Working Set Generalization | Quiz/Study가 Redis lifecycle을 공유하되 domain service는 분리되는지 확인한다. |
| Active Runtime Path | Telegram callback이 Redis state를 start/next/finish 순서로 갱신하는지 확인한다. |
| Flush Persistence | 완료 시 `sessions`, `session_materials`, `user_material_progress`를 transaction으로 반영하는지 확인한다. |
| Boundary & Rendering | Telegram message, callback format, Material rendering edge case를 확인한다. |
| Tests | query, service, Redis state, bot callback 테스트가 핵심 경로를 잡는지 확인한다. |

## Review Order

### 1. Trigger & Config

1. [config.yaml:38](../../config.yaml) — 정오 Study push cron이 운영 설정에서 `0 12 * * *`로 노출되는지 확인한다.
2. [internal/config/config.go:76](../../internal/config/config.go) — `ScheduleConfig`가 `StudyPushCron`을 application config로 받는지 확인한다.
3. [internal/config/config.go:139](../../internal/config/config.go) — config default에도 정오 Study push cron이 같은 값으로 잡히는지 확인한다.
4. [internal/scheduler/scheduler.go:83](../../internal/scheduler/scheduler.go) — Scheduler가 `study_push` job을 등록하고 `buildAndPushStudySessions`로 연결하는지 확인한다.
5. [internal/scheduler/scheduler.go:253](../../internal/scheduler/scheduler.go) — job 실행 시 전체 user를 순회하며 Study Session 생성과 Telegram push를 수행하는지 확인한다.

### 2. Schema & Models

1. [migrations/001_init.sql:56](../../migrations/001_init.sql) — `user_material_progress`가 user별 Material SRS state를 저장하는지 확인한다.
2. [migrations/001_init.sql:108](../../migrations/001_init.sql) — `sessions.mode`가 DB default 없이 application-owned enum으로 저장되는지 확인한다.
3. [migrations/001_init.sql:126](../../migrations/001_init.sql) — `session_materials`가 session별 ordered Material과 `studied_at`을 저장하는 child table인지 확인한다.
4. [internal/model/session.go:16](../../internal/model/session.go) — `SessionModeQuiz`와 `SessionModeStudy`가 interaction model을 분리하는 기준인지 확인한다.
5. [internal/model/session.go:63](../../internal/model/session.go) — `SessionMaterial`와 `StudySessionMaterial`가 DB child row와 Material snapshot을 어떻게 묶는지 확인한다.
6. [internal/model/study_active_session.go:7](../../internal/model/study_active_session.go) — Redis에 저장되는 `StudyActiveSessionState`가 session, ordered items, studied count, initial studied set을 포함하는지 확인한다.
7. [internal/model/study_active_session.go:75](../../internal/model/study_active_session.go) — DB flush 대상 Material이 Working Set 로드 이후 새로 studied 된 항목만 반환되는지 확인한다.

### 3. Dependency Wiring

1. [internal/repository/repositories.go:10](../../internal/repository/repositories.go) — `SessionMaterialRepository`와 `StudyActiveSessionRepository`가 각각 생성/active 경로로 등록되는지 확인한다.
2. [internal/service/services.go:11](../../internal/service/services.go) — `StudySessionService`와 `StudyActiveSessionService`가 별도 service로 유지되는지 확인한다.
3. [internal/service/services.go:38](../../internal/service/services.go) — 생성 service는 Material/Session/SessionMaterial repo를 받고 active service는 StudyActiveSession/Session repo와 Redis를 받는지 확인한다.
4. [internal/config/constants.go:62](../../internal/config/constants.go) — Study callback data format이 `start`, `next`, `finish` action을 모두 표현하는지 확인한다.
5. [internal/config/constants.go:84](../../internal/config/constants.go) — Study Redis Working Set key가 Quiz key와 별도 namespace인지 확인한다.
6. [internal/bot/handler.go:68](../../internal/bot/handler.go) — Bot 생성 시 기존 Quiz flow와 별도로 `StudyFlow`가 초기화되는지 확인한다.

### 4. Creation Path

1. [docs/study_module_plan.md:53](../../docs/study_module_plan.md) — Task 3 정책이 Study Session 생성과 Redis Working Set 진행 방식으로 기록되어 있는지 확인한다.
2. [internal/service/study_session.go:10](../../internal/service/study_session.go) — Study Session당 Material 개수가 8개로 고정되어 있는지 확인한다.
3. [internal/repository/material_repo.go:20](../../internal/repository/material_repo.go) — Material 후보가 user language/level, `vocabulary` category, due/new 조건, pending/in_progress 중복 제외 조건으로 선택되는지 확인한다.
4. [internal/service/study_session.go:43](../../internal/service/study_session.go) — `BuildStudySession`이 Material 조회 후 없으면 session을 만들지 않는지 확인한다.
5. [internal/service/study_session.go:53](../../internal/service/study_session.go) — 생성되는 parent session이 `type='study'`, `mode='study'`, `status='pending'`인지 확인한다.
6. [internal/repository/session_repo.go:20](../../internal/repository/session_repo.go) — `CreateSession`이 `SessionMode` validation 후 explicit `mode` insert를 수행하는지 확인한다.
7. [internal/repository/session_material_repo.go:22](../../internal/repository/session_material_repo.go) — 생성 직후 ordered `session_materials`만 batch insert하고 진행 상태 write는 하지 않는지 확인한다.

### 5. Working Set Generalization

1. [internal/service/working_set.go:13](../../internal/service/working_set.go) — Redis dependency interface가 공통 Working Set에 필요한 최소 연산만 요구하는지 확인한다.
2. [internal/service/working_set.go:49](../../internal/service/working_set.go) — Redis miss, corrupt JSON, validator failure 시 오류와 delete 정책이 일관적인지 확인한다.
3. [internal/service/working_set.go:78](../../internal/service/working_set.go) — JSON marshal과 TTL 저장이 Quiz/Study에서 공통으로 쓰이는지 확인한다.
4. [internal/service/active_session.go:47](../../internal/service/active_session.go) — 기존 Quiz `ActiveSessionService`가 `workingSetStore[ActiveSessionState]`를 쓰되 answer/SRS domain behavior는 유지하는지 확인한다.
5. [internal/service/study_active_session.go:41](../../internal/service/study_active_session.go) — Study `StudyActiveSessionService`가 같은 store를 `StudyActiveSessionState` 타입으로 사용하는지 확인한다.
6. [internal/service/study_active_session.go:207](../../internal/service/study_active_session.go) — Study Working Set key가 `StudySessionWorkingSetRedisKey`로 분리되는지 확인한다.

### 6. Active Runtime Path

1. [internal/bot/handler.go:298](../../internal/bot/handler.go) — Telegram callback dispatch가 `study:` prefix를 `StudyFlow`로 넘기는지 확인한다.
2. [internal/bot/study_flow.go:38](../../internal/bot/study_flow.go) — Study callback parser가 `study:{session_id}:start|next|finish`를 action별로 분기하는지 확인한다.
3. [internal/service/study_active_session.go:63](../../internal/service/study_active_session.go) — start 시 DB load, owner/mode 검증, pending session start, Redis save 순서를 확인한다.
4. [internal/service/study_active_session.go:101](../../internal/service/study_active_session.go) — DB load 후 Redis state version, current index, initially studied set을 재구성하는지 확인한다.
5. [internal/service/study_active_session.go:135](../../internal/service/study_active_session.go) — Redis miss 복구 시 owner/mode 검증 뒤 Working Set을 저장하는지 확인한다.
6. [internal/service/study_active_session.go:156](../../internal/service/study_active_session.go) — `MarkStudied`가 DB write 없이 Redis state만 갱신하는지 확인한다.
7. [internal/model/study_active_session.go:63](../../internal/model/study_active_session.go) — model method가 이미 studied 된 card를 중복 mark하지 않는지 확인한다.
8. [internal/service/study_active_session.go:172](../../internal/service/study_active_session.go) — `Complete`가 모든 card studied 여부를 검증한 뒤 flush와 Redis delete를 수행하는지 확인한다.

### 7. Flush Persistence

1. [internal/repository/study_active_session_repo.go:23](../../internal/repository/study_active_session_repo.go) — Redis Working Set 복구용 DB load가 `sessions`, ordered `session_materials`, Material payload를 단일 joined query로 가져오는지 확인한다.
2. [internal/repository/study_active_session_repo.go:74](../../internal/repository/study_active_session_repo.go) — 완료 flush가 하나의 DB transaction으로 감싸져 있는지 확인한다.
3. [internal/repository/study_active_session_repo.go:87](../../internal/repository/study_active_session_repo.go) — 이미 completed 된 session이면 material/progress flush를 건너뛰는지 확인한다.
4. [internal/repository/study_active_session_repo.go:132](../../internal/repository/study_active_session_repo.go) — parent session 완료 처리 조건이 idempotent한지 확인한다.
5. [internal/repository/study_active_session_repo.go:149](../../internal/repository/study_active_session_repo.go) — `session_materials.studied_at` batch update가 기존 값을 덮어쓰지 않는지 확인한다.
6. [internal/repository/study_active_session_repo.go:183](../../internal/repository/study_active_session_repo.go) — 신규 학습 Material만 `user_material_progress` SRS upsert 대상이 되는지 확인한다.

### 8. Boundary & Rendering

1. [internal/bot/handler.go:106](../../internal/bot/handler.go) — Scheduler boundary에서 호출하는 `PushStudySession` entrypoint를 확인한다.
2. [internal/bot/study_flow.go:28](../../internal/bot/study_flow.go) — 정오 push 메시지와 start button callback data를 확인한다.
3. [internal/bot/study_flow.go:81](../../internal/bot/study_flow.go) — start callback이 Active Session service를 통해 첫 unstudied Material을 표시하는지 확인한다.
4. [internal/bot/study_flow.go:107](../../internal/bot/study_flow.go) — next callback이 현재 Material을 Redis에서 studied 처리하고 다음 order를 표시하는지 확인한다.
5. [internal/bot/study_flow.go:123](../../internal/bot/study_flow.go) — finish callback이 마지막 Material mark 후 session complete flush로 이어지는지 확인한다.
6. [internal/bot/study_flow.go:150](../../internal/bot/study_flow.go) — Material 표시 로직이 Redis state item, last button, empty/out-of-range edge case를 처리하는지 확인한다.
7. [internal/bot/study_flow.go:231](../../internal/bot/study_flow.go) — Vocabulary payload를 Telegram HTML 메시지로 렌더링하는 방식을 확인한다.

### 9. Tests

1. [internal/repository/material_repo_test.go:51](../../internal/repository/material_repo_test.go) — Study Session query가 `vocabulary` category filter와 placeholder 순서를 유지하는지 확인한다.
2. [internal/service/study_session_test.go:41](../../internal/service/study_session_test.go) — 생성 service 테스트가 8개 limit, `mode='study'`, ordered `session_materials`를 검증하는지 확인한다.
3. [internal/service/study_session_test.go:94](../../internal/service/study_session_test.go) — Material 후보가 없을 때 session을 만들지 않는지 확인한다.
4. [internal/service/study_active_session_test.go:37](../../internal/service/study_active_session_test.go) — start가 DB load, pending start, Redis save를 검증하는지 확인한다.
5. [internal/service/study_active_session_test.go:68](../../internal/service/study_active_session_test.go) — `MarkStudied`가 Redis state만 갱신하고 DB flush를 하지 않는지 확인한다.
6. [internal/service/study_active_session_test.go:94](../../internal/service/study_active_session_test.go) — `Complete`가 flush 후 Redis Working Set을 삭제하는지 확인한다.
7. [internal/service/study_active_session_test.go:129](../../internal/service/study_active_session_test.go) — incomplete state 완료가 거부되는지 확인한다.
8. [internal/bot/study_flow_test.go:92](../../internal/bot/study_flow_test.go) — bot flow 테스트가 start → next → finish callback과 flush 결과를 검증하는지 확인한다.

## Notes

- `[UNKNOWN: ...]` 항목 없음.
- 새로 생성되는 Study Session부터 Vocabulary-only 8개 정책이 적용된다. 이미 생성된 pending/in_progress `session_materials`는 기존 Material을 유지한다.
- `StudySessionService`는 생성 전용이다. 진행 상태 변경은 `StudyActiveSessionService`가 Redis Working Set에서 처리한다.
- `SessionMaterialRepository`는 현재 생성 직후 child row insert만 담당한다. card별 direct DB mark/progress write path는 제거됐다.
- card 이동 중에는 Redis Working Set만 갱신한다. `session_materials.studied_at`와 `user_material_progress`는 완료 시 transaction으로 flush된다.
- completed session 재완료 callback은 DB flush를 건너뛰어 progress 중복 증가를 막는다.
- `sessions.mode`는 DB default가 아니라 `SessionRepository.CreateSession`에서 application enum validation으로 강제된다.
