# Integration Test 보강 계획 (for Gemini)

> 목표: **DB(SQL/스키마) regression** 과 **HTTP 핸들러 ↔ 서비스 ↔ DB 연동 regression** 차단.
> **제약: 테스트 코드만 작성한다. production 코드는 절대 수정하지 않는다.**
> 이 계층이 현재 가장 큰 공백이다: repository 4.7%, DB 연동 테스트 0건.

---

## 0. 왜 integration이 핵심인가

repository 패키지는 `sqlx` + `lib/pq` raw SQL이다. 현재 테스트된 것은 [question_repo_test.go](../../internal/repository/question_repo_test.go)의 `buildQuestionBatchInsertQuery`(순수 문자열 조립)뿐.
**실제 SQL 실행 경로는 0%.** 컬럼 추가/이름 변경/JOIN 수정 같은 변경은 컴파일을 통과하고 런타임에 깨진다 — 전형적 회귀인데 지금은 못 잡는다.

대상 스키마: [migrations/001_init.sql](../../migrations/001_init.sql) (users, contents, questions, sessions, session_questions, ...).

---

## 1. 인프라 결정: testcontainers-go 로 ephemeral Postgres 기동

sqlmock은 SQL 문자열을 정규식으로 맞추는 방식이라 **스키마 정합성을 검증하지 못한다**(우리가 막으려는 회귀를 못 막음). 따라서 **실 Postgres**를 띄운다.

**채택: `github.com/testcontainers/testcontainers-go` + `.../modules/postgres`.**
테스트가 컨테이너를 **스스로 기동/정리**한다 → 사람이 사전에 `docker compose up` 할 필요 없이 `go test -tags=integration` 한 줄로 끝나고, CI에서도 동일하게 돈다(hermetic). 이미지는 production과 동일한 **`postgres:16-alpine`** (docker-compose.yml 기준).

**제약(테스트 코드만):** testcontainers는 **테스트 전용 의존성**이다. production import 그래프에는 절대 들어가지 않으므로(빌드 태그 + `_test.go` 한정) production 영향 없음. `go.mod`에는 require가 추가된다(아래 0-1 절차).

> Docker 데몬이 없는 환경(일부 CI 러너)에서는 컨테이너 기동이 실패하므로, 기동 에러 시 `t.Skip("docker unavailable: ...")` 로 graceful skip 한다. `go test ./...`(태그 없음)에는 애초에 포함 안 되니 무관.

### 0-1. 의존성 추가 (1회)

```bash
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
go mod tidy
```

> 이건 production 로직 변경이 아니라 테스트 의존성 추가다. go.mod/go.sum diff만 생기고 `internal/**` production `.go`는 무변경이어야 한다.

### 공용 테스트 하니스: `internal/repository/integration_helper_test.go`

```go
//go:build integration

package repository

// 모든 integration 테스트 상단에 //go:build integration 태그.
// 일반 `go test ./...` 에서는 제외, `go test -tags=integration ./...` 로만 실행.

// 권장 구조:
//  - TestMain(m) 에서 컨테이너 1개를 띄워 패키지 전체가 공유 (테스트마다 컨테이너 새로 띄우면 느림).
//    각 테스트는 TRUNCATE 로 격리 → 빠르고 hermetic.
//
// setupContainer(ctx):
//  1. postgres.Run(ctx, "postgres:16-alpine",
//         postgres.WithDatabase("copylingo"),
//         postgres.WithUsername("copylingo"),
//         postgres.WithPassword("test"),
//         testcontainers.WithWaitStrategy(
//             wait.ForLog("database system is ready to accept connections").
//                 WithOccurrence(2).WithStartupTimeout(60*time.Second)),
//     )
//     // 기동 실패(Docker 없음) → t.Skip 으로 처리할 수 있게 err 반환
//  2. dsn, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
//  3. db := sqlx.MustConnect("postgres", dsn)   // import _ "github.com/lib/pq"
//     // production(cmd/server/server.go)도 sqlx.Connect("postgres", cfg.DB.DSN()) 를 쓴다.
//     //  → 같은 드라이버/타입 매핑이라 스키마 회귀가 동일 조건에서 잡힌다. production 함수는 호출/수정하지 않는다.
//  4. 마이그레이션 적용: os.ReadFile("../../migrations/001_init.sql") → db.Exec(string)
//     // 멱등(CREATE TABLE IF NOT EXISTS)이라 안전. 추후 migrations 파일 늘면 디렉터리 정렬 순 Exec.
//  5. cleanup: testcontainers.CleanupContainer(t, pgContainer) (또는 TestMain 종료 시 Terminate)
//
// truncateAll(t, db):
//  - 각 테스트 시작 시 호출. "TRUNCATE users, contents, questions, sessions,
//    session_questions, tips RESTART IDENTITY CASCADE" (FK 때문에 CASCADE 필수).
//  - SERIAL/BIGINT PK 채번을 리셋해 테스트 간 ID 가정이 깨지지 않게 한다.
```

> 참고: testcontainers-go 의 정확한 API 시그니처(`postgres.Run` vs 구버전 `postgres.RunContainer`)는 받은 버전에 맞춰 확인할 것. `go doc github.com/testcontainers/testcontainers-go/modules/postgres` 로 현재 버전 시그니처를 보고 맞춘다.

---

## 2. internal/repository — repo별 CRUD/쿼리 검증 (//go:build integration)

각 repo는 `New<Repo>Repository(db *sqlx.DB)` 로 생성. 실제 시그니처는 아래 참조(이미 확인됨). **모든 메서드를 실제 DB에 대해 round-trip 검증**한다: write → read → 값 일치.

### `internal/repository/user_repo_integration_test.go`
대상 [user_repo.go](../../internal/repository/user_repo.go):
- `GetOrCreate`: 최초 호출 시 insert, 재호출 시 동일 row 반환(중복 생성 안 함)
- `GetByID` / `Update`
- `UpdateStreak`: `last_active_date`가 어제면 streak+1, 오늘이면 변화 없음, 그 이전이면 reset(=1). `timeNowDate`/`timeYesterdayDate` 로직과 DB 상태를 함께 검증 — **streak 회귀의 핵심**
- `GetAllUsers`

### `internal/repository/session_repo_integration_test.go`
대상 [session_repo.go](../../internal/repository/session_repo.go):
- `CreateSession`(ID 채번 확인) → `GetByID`
- `GetSessionsByStatus`(config.SessionStatus 별 필터)
- `ListInProgress`
- `Start`(status/started_at 갱신) / `Complete`(correct_count, completed_at 갱신)
- `GetTodaySessions`(날짜 경계 — 오늘 것만)
> FK: sessions.user_id → users.id, 먼저 user 시드 필요.

### `internal/repository/session_question_repo_integration_test.go`
대상 [session_question_repo.go](../../internal/repository/session_question_repo.go):
- `CreateSessionQuestions`(배치 insert) → `GetBySession`
- `RecordAnswer`(user_answer/is_correct/answered_at 갱신)
- `GetWrongAnswers`(is_correct=false만)
- `GetCategoryAccuracy`(집계 정확도 — 정답/총계 비율 계산 검증)
- `GetTodayStats`(오늘 답변 수/정답 수)

### `internal/repository/question_repo_integration_test.go`
대상 [question_repo.go](../../internal/repository/question_repo.go) (기존 unit은 유지):
- `CreateBatch` → `GetByID`
- `GetNewQuestions`(language/level/category 필터 + excludeIDs 제외 + limit)
- `GetDueReviews` / `GetDueReviewCount`(srs_due_date <= today 인 것만)
- `UpdateSRS`(srs_interval/ease/repetitions/due_date 갱신) — SRS 회귀 핵심
- `IncrementServed` / `IncrementCorrect`

### `internal/repository/active_session_repo_integration_test.go`
대상 [active_session_repo.go](../../internal/repository/active_session_repo.go) — **트랜잭션 로직이라 최우선**:
- `LoadActiveSession`: sessions + session_questions + questions JOIN 결과를 `ActiveSessionState`로 올바르게 조립하는지(아이템 수, 정답 여부, 문제 본문 매핑)
- `FlushActiveSession`: 트랜잭션으로 `markSessionCompleted` + `flushSessionQuestions` + `flushQuestions`가 모두 반영되는지, 중간 실패 시 롤백되는지(가능하면 일부러 위반 데이터로 실패 유도)
- 완료 처리 멱등성(이미 completed면 중복 처리 안 함)

### `internal/repository/content_repo_integration_test.go`, `tip_repo_integration_test.go`
- content: `Create`/`GetByID`/`GetArticles`(language/level/limit)/`ExistsByURL`(UNIQUE url)
- tip: `Create`/`ListActive`(language/level/limit, is_active 필터)/`CountActive`

---

## 3. internal/miniapp — HTTP 핸들러 통합 (httptest, DB는 mock 또는 실DB)

기존 [miniapp/handler_test.go](../../internal/miniapp/handler_test.go)가 이미 `httptest` + gin(`gin.CreateTestContext`)으로 핸들러를 in-process 호출한다. 그 패턴을 확장.

> ⚠️ **실제 코드(확인됨):** 핸들러 생성은 `NewHandler(HandlerDeps{...})`, 라우트 일괄 등록은 `RegisterRoutes(r, cfg, services, rdb, messenger)`. 핸들러 메서드는 **`(*Handler).ListTips`**, **`(*Handler).SubmitHandwriting`** (gin.HandlerFunc 형태). 인증은 미들웨어 함수가 아니라 **`InitDataVerifier`**: `NewInitDataVerifier(botToken, maxAge)` + `(*InitDataVerifier).Verify(initData)`. `handleGetQuestion`/`handleHealth`/`NewAuthMiddleware` 같은 이름은 **없다** — 위 이름만 사용.

두 갈래:
- **(a) 서비스 mock 주입형 (build tag 불필요):** `HandlerDeps`에 mock 서비스를 끼워 `NewHandler(...)` → `gin.CreateTestContext` + `httptest.NewRecorder` 로 메서드 직접 호출. 기존 `TestListTips` 패턴 그대로. handler↔service 배선/직렬화/상태코드 회귀를 잡는다.
- **(b) 실 DB 연동형 (//go:build integration):** repo→service→handler 전체를 실 Postgres 위에서. 핵심 1~2개만.

### `internal/miniapp/handler_integration_test.go` (또는 기존 handler_test.go 확장)
대상 [handler.go](../../internal/miniapp/handler.go):
- `ListTips`: 정상 → 200 + tip JSON 배열; 파라미터 누락 → 4xx(기존 "missing parameters" 케이스 존재, 보강); repo 에러 → 5xx
- `SubmitHandwriting`: 정상 채점 → 200 + 결과; AI 미가용(`service.ErrAIUnavailable`) → `handwritingPublicError`가 매핑하는 status/메시지; 잘못된 multipart·이미지 누락 → 4xx
- **인증 통합** [auth.go](../../internal/miniapp/auth.go): `InitDataVerifier.Verify` — 유효 initData(올바른 HMAC-SHA256 서명) → user 반환, 위조 서명/`maxAge` 만료 → 에러. 기존 [auth_test.go](../../internal/miniapp/auth_test.go)와 중복되지 않는 케이스만. `RegisterRoutes`로 라우트에 verifier가 붙었을 때 보호 엔드포인트가 무서명 요청을 401로 막는지(통합) 검증.

---

## 4. 실행 & 검수 기준

```bash
# 전제: 로컬/CI에 Docker 데몬이 떠 있을 것 (testcontainers가 컨테이너를 알아서 띄움 — 사전 준비 불필요)

# integration 테스트만 실행 (컨테이너 자동 기동·정리)
go test -tags=integration ./internal/repository/... ./internal/miniapp/...

# 일반 빌드에서 제외되는지 확인 (tag 없으면 integration 파일 무시 → Docker 없어도 통과)
go test ./...
```

**완료 조건:**
1. `//go:build integration` 태그가 모든 실DB 테스트 파일 최상단에 있고, Docker 데몬 부재 시 `t.Skip("docker unavailable")` (CI 러너 안전)
2. `go test ./...`(태그 없음)은 영향 없이 통과 — integration은 옵트인, Docker 불필요
3. `go test -tags=integration ./...` 가 testcontainers Postgres에서 통과
4. repository 각 메서드가 최소 1개 round-trip 테스트로 검증됨
5. production `.go`(`internal/**`, `cmd/**`) `git diff` 비어 있음 — 변경은 **테스트 파일 + go.mod/go.sum(testcontainers require)** 으로 한정
6. UNIQUE/FK 제약, 날짜 경계(streak/SRS due/today stats), 트랜잭션 롤백이 명시적으로 테스트됨

production 수정이 필요해 막힌 항목은 하단 `## BLOCKED` 에 기록.
