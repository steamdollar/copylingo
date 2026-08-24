# Project Structure Improvements Review Flow

## Scope

- 2026-08-24 구조 리뷰에서 확인된 repository SSOT drift, Scheduler boundary/scalability, Bot dependency boundary를 하나씩 처리하기 위한 순서표다.
- 각 Work Item은 별도 세션에서 하나만 진행한다. 이 문서는 navigation artifact이며, architecture 선택이나 구현을 승인한 기록은 아니다.

## Route Summary

| Category | Purpose |
|---|---|
| Repository SSOT | 동작 변경 전에 dead link, 잘못된 example config, legacy 경로와 backlog drift를 정리한다. |
| Dependency Wiring | Scheduler가 Telegram Bot concrete type을 요구하는 불필요한 결합부터 제거한다. |
| Scheduler Boundary | cron runner와 session/content/asset job 책임을 분리한다. |
| Scalability Decision | 전체 사용자 동기 순회를 queue/bounded worker로 바꿀지 Case A에서 결정한다. |
| Bot Boundary | `Bot` god object 대신 Flow별 narrow dependency를 주입한다. |
| Deferred Hotspots | 앞선 경계 개선 후에도 churn이 남는 파일만 후속 분리한다. |
| Tests & Close | 각 Work Item별 targeted test, 전체 test, runtime 확인과 문서 close를 수행한다. |

## Work Item Index

| ID | Work Item | Case | Effort | Risk | Start Gate |
|---|---|---:|---:|---:|---|
| S1 | README navigation 정합화 | B (docs) | XS | Low | 바로 계획 가능 |
| S2 | example LLM API key 정합화 | B (config docs) | XS | Low | 실제 config key 재확인 |
| S3 | legacy local audio 경로 정리 | B (config) | S | Medium | runtime reference와 rollback 확인 |
| S4 | orphan TODO backlog 정합화 | B (docs) | S | Low | 등록/보관/폐기 선택 |
| S5 | ROADMAP 역할 정합화 | A 또는 milestone close | S | Low | milestone-only 규칙 우선 |
| A1 | Scheduler concrete Bot dependency 제거 | B | S | Low | 작은 refactor plan 승인 |
| A2 | Scheduler job 책임 분리 | B | M | Medium | 파일/port 경계 plan 승인 |
| A3 | Queue + bounded worker 도입 결정 | A → B | L | High | ADR 결정 필수 |
| B1 | `SessionFlow` narrow dependency 주입 | A → B | M | Medium | `FlowDeps` 경계 합의 |
| B2 | Bot update dispatch와 command 처리 분리 | B | M | Medium | B1 완료 후 재측정 |
| C1 | `SessionBuilder` 책임 재점검 | 0 → A/B | M | Medium | A/B 작업 후 churn 재측정 |
| C2 | LLM use-case adapter 분리 재점검 | 0 → A/B | M | Medium | A/B 작업 후 churn 재측정 |

## Review Order

### 1. Repository SSOT

1. [ ] **S1 — README navigation 정합화**: [README.md:254](../../README.md)에서 존재하지 않는 `docs/ADR.md`, `docs/HISTORY.md`, `CURRENT_TASK.md` 링크와 구식 workflow를 현재 SSOT인 `STATUS.md`, `docs/adr/`, `docs/workthrough/`로 교체한다.
2. [ ] **S2 — example LLM API key 정합화**: [.env.example:10](../../.env.example)의 `COPYLINGO_OPENAI_API_KEY`를 실제 runtime key인 `COPYLINGO_LLM_API_KEY`와 맞추고 [docker-compose.yml:89](../../docker-compose.yml), [README.md:100](../../README.md)의 명칭과 교차 확인한다.
3. [ ] **S3 — legacy local audio 경로 정리**: [config.yaml:31](../../config.yaml), [docker-compose.yml:101](../../docker-compose.yml), [.dockerignore:11](../../.dockerignore)에 남은 `./data/audio`가 ADR-032 이후에도 필요한 compatibility path인지 먼저 확인한 뒤, 불필요하면 설정·mount·ignore를 함께 제거한다.
4. [ ] **S4 — orphan TODO backlog 정합화**: [STATUS.md:25](../../STATUS.md)에 없는 [02_integration_test_plan.md:1](../todos/02_integration_test_plan.md), [03_e2e_test_plan.md:1](../todos/03_e2e_test_plan.md)을 현재 backlog로 등록할지, archive할지, 폐기할지 결정하여 TODO SSOT를 하나로 만든다.
5. [ ] **S5 — ROADMAP 역할 정합화**: [ROADMAP.md:4](../../ROADMAP.md)의 오래된 날짜·경로·phase 상태는 즉시 임의 수정하지 않고, `ROADMAP.md`는 milestone 완료 때만 갱신한다는 project rule과 충돌하지 않는 정리 방식을 먼저 결정한다.

Completion gate:

- S1은 dead link와 구식 workflow 검색 결과가 0건이어야 한다.
- S2는 config loader, Compose, README, `.env.example`의 key가 동일해야 한다.
- S3은 `docker compose config`와 `make test`가 통과하고, 제거 시 git으로 즉시 rollback 가능해야 한다.
- S4/S5는 중복 backlog 또는 별도 SSOT를 새로 만들지 않아야 한다.

### 2. Dependency Wiring

1. [ ] **A1 — Scheduler port seam**: [scheduler.go:28](../../internal/scheduler/scheduler.go)의 `sessionPusher` interface를 이미 정의해 놓고도 [scheduler.go:34](../../internal/scheduler/scheduler.go) 생성자가 `*bot.Bot`을 받는 모순을 제거한다.
2. [ ] [server.go:106](../../cmd/server/server.go)의 composition root에서 concrete Bot을 interface parameter로 넘기고, Scheduler package의 `internal/bot` import를 제거한다.
3. [ ] public behavior와 cron schedule은 바꾸지 않고 constructor dependency만 `bot *bot.Bot` → `pusher sessionPusher`로 좁힌다.

Completion gate:

- `internal/scheduler`가 `internal/bot`을 import하지 않는다.
- Scheduler 단위 테스트에서 fake `sessionPusher`를 직접 주입할 수 있다.
- targeted test 후 `make test`, `make restart-app`, `http://localhost:8080/health` 확인까지 완료한다.

### 3. Scheduler Boundary

1. [ ] **A2 — runner와 job handler 분리**: [scheduler.go:34](../../internal/scheduler/scheduler.go)의 lifecycle/cron registration과 [scheduler.go:216](../../internal/scheduler/scheduler.go)의 per-user session delivery를 같은 파일·책임으로 유지할 필요가 있는지 검토한다.
2. [ ] session build/push, content collection, tip top-up, audio top-up, reminder를 우선 같은 package의 작은 job handler로 분리하여 behavior-preserving refactor로 제한한다.
3. [ ] 파일 이동만 하지 말고 각 job이 필요로 하는 service/port를 좁혀 Scheduler가 orchestration만 담당하게 한다.

Completion gate:

- cron 등록/중단과 각 job 실행 책임이 별도 type 또는 narrow function boundary로 구분된다.
- 기존 schedule, log event, failure counting semantics가 유지된다.
- 각 job에 focused test가 있고 `make test`가 통과한다.

### 4. Scalability Decision

1. [ ] **A3 — Case A 선결**: [scheduler.go:216](../../internal/scheduler/scheduler.go)의 `GetAllUsers` + 동기 loop를 그대로 둘지, DB batch + queue + bounded worker로 바꿀지 합의한다.
2. [ ] 최소 결정 항목은 batch cursor, concurrency bound, idempotency key, retry/DLQ, overlapping cron 방지, Telegram rate limit, partial failure recovery다.
3. [ ] 결정 후 최신 [ADR range file](../adr/ADR_from_41_to_60.md)에 기록하고, migration/rollback 가능한 작은 단계로 구현 계획을 나눈다.

Completion gate:

- 사용자 승인과 ADR 전에는 queue/worker 구현을 시작하지 않는다.
- tens-of-thousands-user 가정에서 처리량과 failure isolation 근거가 있어야 한다.
- 기존 synchronous path를 rollout/rollback fallback으로 유지할지 명시한다.

### 5. Bot Boundary

1. [ ] **B1 — `SessionFlow`부터 narrow deps 주입**: [handler.go:51](../../internal/bot/handler.go)에서 `Bot`이 `api/cfg/services/rdb`를 모두 소유하고 [session_flow.go:22](../../internal/bot/session_flow.go)에 자기 자신을 넘기는 구조를 먼저 끊는다.
2. [ ] `SessionFlow`가 실제 사용하는 messenger, session service, Redis state, LLM service, config 값만 interface/value로 주입하는 `FlowDeps` 경계를 Case A에서 합의한다.
3. [ ] [study_flow.go:24](../../internal/bot/study_flow.go)는 rendering helper가 비교적 cohesive하므로 `SessionFlow` 패턴이 검증된 뒤 같은 방식으로 옮긴다.
4. [ ] **B2 — dispatch 분리**: B1 이후에도 [handler.go:74](../../internal/bot/handler.go)의 Telegram polling/update dispatch와 command/menu 처리가 함께 높은 churn을 보이면 transport dispatch를 분리한다.

Completion gate:

- Flow test가 concrete `*Bot` 없이 구성된다.
- Telegram API 호출, Redis state, domain service 중 하나를 바꿔도 나머지 Flow constructor가 불필요하게 흔들리지 않는다.
- callback/message behavior regression test와 `make test`, app restart/health 확인을 완료한다.

### 6. Deferred Hotspots

1. [ ] **C1 — Session service 재측정**: [session_builder.go:82](../../internal/service/session_builder.go)의 build algorithm과 [session_builder.go:328](../../internal/service/session_builder.go)의 query/lifecycle method가 계속 함께 변경될 때만 별도 service로 나눈다.
2. [ ] **C2 — LLM adapter 재측정**: [llm.go:68](../../internal/external/llm.go)의 provider transport와 grading/handwriting/tips use case가 계속 함께 변경될 때만 use-case adapter/prompt 파일로 분리한다.
3. [ ] [cmd/ja/seeder/main.go:69](../../cmd/ja/seeder/main.go)는 997줄이지만 low-churn이고 대응 테스트가 충분하므로 현재는 분리하지 않는다.

Completion gate:

- A/B 작업 후 git churn과 co-change를 다시 측정한 증거가 있을 때만 C1/C2를 승격한다.
- 단순 LOC 또는 보기 불편하다는 이유만으로 package/file을 추가하지 않는다.

### 7. Tests & Close

1. [ ] 코드/config 변경 Work Item은 관련 package targeted test를 먼저 실행하고 마지막에 `make test`를 실행한다.
2. [ ] runtime-affecting 변경은 [Makefile:1](../../Makefile)의 target manifest를 확인한 뒤 `make restart-app`과 health check를 수행한다.
3. [ ] 각 Work Item 완료 시 별도 workthrough를 작성하고, 해당 작업이 기존 `STATUS.md` 항목을 완료한 경우에만 status를 이동한다.
4. [ ] non-trivial architecture 결정은 구현 전에 ADR에 기록한다.

## Notes

- 추천 실행 순서는 `S1 → S2 → S3 → S4 → S5 gate → A1 → A2 → A3 gate → B1 → B2 → C1/C2 재평가`다.
- 먼저 시작할 기본값은 **S1**이다. 이후 사용자는 Work Item ID만 지정해도 해당 범위 하나만 진행할 수 있다.
- Repository aggregate, 월별 `docs/workthrough/`, ADR range/special-file 분리는 현재 구조 부채로 보지 않는다.
- `[UNKNOWN: ...]` 항목은 없다.
