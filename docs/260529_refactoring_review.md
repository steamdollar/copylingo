# 2026-05-29 프로젝트 리팩토링 검토

## 결론

`go test ./...`, `go vet ./...` 통과.

코드 변경은 하지 않았고, 검토 시점의 uncommitted 변경은 [`web/miniapp/handwriting/app.js`](../web/miniapp/handwriting/app.js#L145)에만 있었다.

우선순위:

1. User-scoped SRS/stat 분리
2. Redis ActiveSession 원자성 보강
3. SessionBuilder transaction/reservation 정리
4. Bot/MiniApp 의존성 분리
5. Legacy/dead API 제거

---

## Issue 1. User-scoped 학습 상태가 global `questions`에 섞임

**Issue**: SRS 필드가 `questions` row에 있고, `GetDueReviews`가 user/language를 받지 않는다. 통계도 user filter 없이 전체 집계한다.

근거:

- [`migrations/001_init.sql`](../migrations/001_init.sql#L53)
- [`internal/repository/question_repo.go`](../internal/repository/question_repo.go#L57)
- [`internal/service/session_builder.go`](../internal/service/session_builder.go#L95)
- [`internal/service/analyzer.go`](../internal/service/analyzer.go#L38)

**Options**:

- **A. recommended**: `user_question_progress` 또는 `srs_schedules` 도입, `(user_id, question_id)` 기준 SRS SSOT 분리
- **B**: 단기적으로 language/user filter만 추가
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 3~5일
- 리스크: 중간
- 타 코드 영향도: schema/repository/service/test 전반
- 유지보수 부담: 크게 감소

**Recommendation**: Option A. 수만 사용자 가정에서는 현재 구조가 데이터 오염을 만든다.

**Decision**: ADR + migration plan 승인 필요.

---

## Issue 2. Redis ActiveSession update가 atomic하지 않음

**Issue**: `RecordAnswer`가 `Get → mutate → save` 구조라 double click, text submit, Mini App submit 경쟁 시 lost update 가능성이 있다.

근거:

- [`internal/service/active_session.go`](../internal/service/active_session.go#L137)
- [`internal/service/active_session.go`](../internal/service/active_session.go#L204)
- [`internal/bot/handler.go`](../internal/bot/handler.go#L69)

**Options**:

- **A. recommended**: Redis `WATCH/MULTI` 또는 Lua CAS로 `RecordAnswer`/`SetCurrentIndex` atomic 처리
- **B**: duplicate answer key만 `SETNX`로 방어
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 1~2일
- 리스크: 중간
- 타 코드 영향도: service tests 중심
- 유지보수 부담: 감소

**Recommendation**: Option A. Redis working set을 쓰는 순간 atomic mutation 경계가 필요하다.

**Decision**: 먼저 이 이슈만 SMALL CHANGE로 쪼개기 적합.

---

## Issue 3. Session 생성이 transaction/reservation 없이 분리됨

**Issue**: `CreateSession` 후 `CreateSessionQuestions`가 실패하면 orphan session이 남는다. SRS/new question fetch error도 log 후 계속 진행한다.

근거:

- [`internal/service/session_builder.go`](../internal/service/session_builder.go#L97)
- [`internal/service/session_builder.go`](../internal/service/session_builder.go#L178)
- [`internal/service/session_builder.go`](../internal/service/session_builder.go#L186)

**Options**:

- **A. recommended**: repository에 `CreateSessionWithQuestionsTx` 추가
- **B**: 실패 시 compensating delete
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 0.5~1일
- 리스크: 낮음
- 타 코드 영향도: 좁음
- 유지보수 부담: 감소

**Recommendation**: Option A. transaction boundary가 명확하고 rollback이 자연스럽다.

**Decision**: Issue 2 다음에 처리 권장.

---

## Issue 4. Bot/MiniApp/Scheduler 의존성 결합도가 높음

**Issue**: `scheduler`가 concrete `bot.Bot`에 의존하고, `miniapp`도 `bot` 패키지를 import한다. `Bot`은 API, config, services, Redis, flow를 모두 들고 있다.

근거:

- [`internal/scheduler/scheduler.go`](../internal/scheduler/scheduler.go#L10)
- [`internal/miniapp/handler.go`](../internal/miniapp/handler.go#L16)
- [`internal/bot/handler.go`](../internal/bot/handler.go#L25)

**Options**:

- **A. recommended**: `TelegramMessenger`/`SessionPusher` interface를 별도 boundary로 추출
- **B**: helper만 `internal/callback` 또는 `internal/telegram`으로 이동
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 1~2일
- 리스크: 중간
- 타 코드 영향도: bot/miniapp/scheduler
- 유지보수 부담: 감소

**Recommendation**: Option A. 테스트와 기능 추가 모두 쉬워진다.

**Decision**: BIG CHANGE로 묶는 편이 안전.

---

## Issue 5. Legacy/dead API와 duplicate domain type

**Issue**: Redis working set 이후 직접 DB 기록 경로가 남아 있다. `SessionStatus`도 `config`와 `model`에 중복이다.

근거:

- [`internal/repository/question_repo.go`](../internal/repository/question_repo.go#L79)
- [`internal/repository/session_question_repo.go`](../internal/repository/session_question_repo.go#L48)
- [`internal/repository/session_repo.go`](../internal/repository/session_repo.go#L66)
- [`internal/config/constants.go`](../internal/config/constants.go#L6)
- [`internal/model/session.go`](../internal/model/session.go#L16)

**Options**:

- **A. recommended**: 사용 안 하는 API 제거 + status type을 `model`로 단일화
- **B**: deprecated comment만 추가
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 0.5일
- 리스크: 낮음
- 타 코드 영향도: tests 일부
- 유지보수 부담: 감소

**Recommendation**: Option A. DRY 관점에서 즉시 정리 가치가 있다.

**Decision**: Issue 3과 같이 처리 가능.

---

## Issue 6. Batch/scale path가 아직 synchronous + N+1

**Issue**: scheduler는 모든 user를 한 번에 가져와 순차 처리하고, content saver는 `ExistsByURL → Create` N+1이다. `ORDER BY RANDOM()`도 커지면 병목이다.

근거:

- [`internal/scheduler/scheduler.go`](../internal/scheduler/scheduler.go#L98)
- [`internal/pipeline/saver.go`](../internal/pipeline/saver.go#L28)
- [`internal/repository/content_repo.go`](../internal/repository/content_repo.go#L46)
- [`internal/repository/question_repo.go`](../internal/repository/question_repo.go#L50)

**Options**:

- **A**: Outbox/queue 또는 paginated worker + batch upsert
- **B. recommended**: 단기 batch insert/upsert만 적용
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 2~5일
- 리스크: 중간
- 타 코드 영향도: scheduler/pipeline/repository
- 유지보수 부담: 중간

**Recommendation**: Option B 먼저. Outbox는 세션/SRS 분리 후 판단.

**Decision**: 성능 리팩토링 milestone으로 분리 권장.

---

## Issue 7. Error handling/lifecycle 정합성

**Issue**: repository에서 log를 찍거나 raw error를 반환하는 곳이 남아 있고, bot update는 `context.Background()` 기반 goroutine이다.

근거:

- [`internal/repository/question_repo.go`](../internal/repository/question_repo.go#L29)
- [`internal/repository/session_repo.go`](../internal/repository/session_repo.go#L60)
- [`internal/bot/handler.go`](../internal/bot/handler.go#L150)
- [`cmd/server/server.go`](../cmd/server/server.go#L55)

**Options**:

- **A. recommended**: repository는 wrapping only, boundary layer logging only, worker context/shutdown 정리
- **B**: 에러 wrapping만 우선
- **C. Do nothing**: 현 구조 유지

**Metrics**:

- 구현 공수: 1일
- 리스크: 낮음
- 타 코드 영향도: 넓지만 기계적
- 유지보수 부담: 감소

**Recommendation**: Option A. 운영 디버깅 품질이 올라간다.

**Decision**: dead API cleanup 후 진행 권장.

---

## 확인된 검증 결과

```bash
go test ./...
go vet ./...
```

둘 다 통과.

## 확인되지 않은 영역

`[UNKNOWN: 실제 운영 DB에 repo 외부에서 별도 index/constraint가 수동 적용됐는지는 확인하지 않음]`

## 권장 진행 순서

1. Issue 2: Redis ActiveSession atomic mutation
2. Issue 3 + Issue 5: Session 생성 transaction + legacy API/type cleanup
3. Issue 1: User-scoped SRS/stat ADR 및 migration 설계
4. Issue 4: Bot/MiniApp/Scheduler boundary 정리
5. Issue 6 + Issue 7: batch/scale path 및 lifecycle/error handling 정리
