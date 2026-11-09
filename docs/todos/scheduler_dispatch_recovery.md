# scheduler 발송이 끊기거나 놓치면 이어서 보내고, 사람이 고칠 수 있게 하기

> Case C TODO. 2026-11-09 graceful shutdown 작업(ADR-085) 중 발견. 사용자가 "todo에 두자"로 결정.
> **상태: 후보안 기록만. 채택 미결.** 사용자 판단(2026-11-09): A안은 "좀 무거워지는 느낌". 착수 전에 아래 A/B/C 중 하나를 사용자와 먼저 고른다(Case A).
> §3 이하는 **A안을 고를 때의** 세부 설계다.

## 0. 후보안

| 안 | 내용 | 막는 구멍(§1) | 크기 |
|---|---|---|---|
| **A** (§3~) | Redis claim을 DB 상태 기록(`slot_dispatches`)으로 바꾸고, 끊긴 단계부터 재개 + 놓친 slot 늦게 발송 + admin 조회·재발송 | 1·2·3 전부. 프로세스가 죽은 경우도 재개 | 큼: 테이블 1, repository 1, admin 명령 1, tick·dispatcher 변경, Redis claim 삭제 |
| **B** (경량) | Redis claim 유지. ① build·push가 **오류로** 실패하면 claim key를 지운다 ② `tick`이 지난 window 안 30분 칸도 조회한다(§3 D9와 같은 방식, 중복은 기존 claim이 막음) ③ 시작 직후 tick 1회(D10) | 1, 2의 "오류" 경우만. 처리 도중 프로세스가 죽으면 claim이 남아 그날 안 감. 상태 조회 불가(수동 = `redis-cli DEL` 후 다음 tick) | 작음: dispatcher·tick 수십 줄 |
| **C** | 현행 유지 | 없음 | 0. 근거: 11-06~11-09 로그상 scheduler job은 모두 1초 미만이라 끊긴 적이 없다 |

A는 outbox 패턴 그 자체는 아니다. outbox는 업무 데이터와 "보낼 이벤트"를 같은 transaction에 쓰고 relay가 발행하는 방식이다. A는 발송 단계를 DB에 남기는 job 상태 기록(checkpoint)이다. 둘 다 DB 기록 + 재시도로 at-least-once를 얻는다는 점은 같다.

## 1. 배경

scheduler는 30분마다 `tick`을 돌려 slot(아침/저녁 study·quiz) 세션을 만들고 Telegram으로 보낸다. 지금 구조에는 세 가지 구멍이 있다.

| # | 구멍 | 원인 (코드) | 결과 |
|---|---|---|---|
| 1 | 서버가 꺼져 있던 시각의 slot은 영영 안 간다 | `tick`이 **지금 이 30분 칸**만 조회한다 ([scheduler.go `tick`](../../internal/scheduler/scheduler.go), `GetUsersBySlot(slot, "HH:MM", tz)`) | 08:00에 재시작 중이면 그날 아침 study 누락 |
| 2 | build·push 도중 끊기거나 실패하면 그날은 다시 안 보낸다 | claim(Redis `SETNX copylingo:push:lock:{user}:{slot}:{date}`, 24h)을 **build 전에** 잡는다 ([dispatcher.go `dispatchUser`](../../internal/scheduler/dispatcher.go), [push_claims.go](../../internal/redisstore/push_claims.go)) | claim만 남고 세션은 안 감. shutdown 10초 한도(ADR-085)를 넘겨 잘려도 같다 |
| 3 | 어떤 사용자·slot이 어디까지 갔는지 볼 수단이 없다 | 기록이 로그뿐이고 Redis key는 "잡았다"만 안다 | 문제가 생겨도 조회·수동 재발송 불가 |

부수 문제: `today := time.Now().Format("2006-01-02")`가 **서버** 날짜다. 사용자 timezone 날짜와 다를 수 있다(지금 사용자는 서버와 같은 Asia/Seoul이라 드러나지 않음).

## 2. 목표

1. **기록**: 사용자·slot·날짜마다 발송이 어느 단계까지 갔는지 DB에 남긴다.
2. **자동 재개**: 끊긴 발송은 남은 단계부터 이어 하고, 놓친 slot은 늦게라도 보낸다(범위 안에서만).
3. **수동 수정**: admin 명령으로 상태를 보고, 특정 사용자·slot을 다시 보내게 한다.

```
 claim(INSERT)      build 성공          push 성공
 ─────────────▶ claimed ─────────▶ built ─────────▶ pushed   (끝)
                   │                  │
                   │ backlog≥3 → remind 성공 ─▶ reminded     (끝)
                   │ 만들 내용 없음 ─────────────▶ empty      (끝)
                   │                  │
            (끊김/실패: 상태 그대로, attempts·last_error 기록)
                   ▼                  ▼
         다음 tick: claimed → build+push / built → 기존 session push만
```

## 3. A안 세부 결정 (A 채택 시)

| # | 결정 | 이유 / tradeoff |
|---|---|---|
| D1 | 기록은 **Postgres 테이블 `slot_dispatches`**. Redis `PushClaims`는 지운다 | 상태 전이·조회·재시도 횟수가 필요하다. Redis key는 TTL로 사라지고 상태가 없다. 비용: 사용자·slot마다 하루 INSERT 1 + UPDATE 1~2. 수만 명 기준에서도 작다 |
| D2 | claim = `INSERT ... ON CONFLICT DO NOTHING`. 상태 전이 = `UPDATE ... WHERE <pk> AND state=<기대 상태>` | 여러 instance가 동시에 돌아도 한 곳만 잡는다(AGENTS.md §4 규모 가정) |
| D3 | `local_date`는 **사용자 timezone 기준** 날짜(slot 예정 시각의 날짜) | 위 부수 문제 해결. 자정 넘는 lookback에서도 날짜가 맞는다 |
| D4 | 재개: `claimed` → build+push, `built` → **이미 만든 session을 push만** | 세션을 두 번 만들지 않는다 |
| D5 | 중복 발송 가능성은 **수용**(at-least-once) | push 성공 직후 `pushed` 기록 전에 죽으면, 같은 session의 "도착" 메시지가 한 번 더 간다. 같은 세션이라 피해가 작다. exactly-once는 Telegram 쪽에 멱등 키가 없어 불가 |
| D6 | "버려진 행" 판정 lease = **10분**(`runJob` 한도와 같음). `updated_at < now - 10분`인 `claimed`/`built`만 가져간다 | 다른 instance가 진행 중인 행을 뺏지 않는다. 끊긴 발송은 다음 tick(최대 30분 뒤)에 재개 |
| D7 | 재시도 **최대 3회**(`attempts`). 넘으면 ERROR 로그 후 멈추고 사람이 admin으로 처리 | 계속 실패하는 사용자가 매 tick 자원을 먹지 않게 |
| D8 | claim DB 오류 시 **그 사용자를 건너뛴다**(fail-closed). 지금 Redis 오류는 그냥 진행(fail-open) | 재개 경로가 생겼으니 다음 tick이 다시 잡는다. DB가 죽었으면 어차피 build도 실패한다 |
| D9 | 놓친 slot 찾기: `tick`이 **지금 칸 + 지난 범위 안 30분 칸들**을 모두 조회. 이미 행이 있는 사용자는 SQL에서 뺀다(`NOT EXISTS`) | 기존 `GetUsersBySlot`를 그대로 넓힌다. 평상시 추가 비용은 빈 결과 조회 몇 번뿐 |
| D10 | 서버 시작 직후 `tick`을 **한 번 바로** 돈다 | 재시작 중 놓친 slot을 다음 30분 칸까지 기다리지 않는다 |
| D11 | 범위 판정 기준 컬럼은 `due_at`(보내야 하는 절대 시각). admin `retry`는 `due_at=now()`로 옮긴다 | 수동 재발송도 같은 재개 경로를 탄다. admin이 Telegram client를 만들 필요가 없다 |
| D12 | 보존 30일. 기존 cron에 하루 1번 `DELETE ... WHERE local_date < current_date - 30` | 하루 4행/사용자라 수만 명이면 1년에 수천만 행이 된다 |
| D13 | 착수할 때 이 결정들을 ADR 하나로 남긴다(ADR-086 이후 빈 번호, `docs/adr/ADR_from_81_to_100.md`) | Redis claim → DB 상태 기록은 구조 결정이다 |

## 4. 미결 (착수 전 사용자 확인)

- **늦게 보내는 범위(window)**: 예정 시각에서 얼마까지 늦게 보낼지. **권장 2시간**(08:00 study가 10:00까지만). 길게 잡으면 아침 study가 오후에 오는 일이 생긴다. 확인 전에는 2시간으로 구현하되 상수 하나(`lateDispatchWindow`)로 둔다.

## 5. 바꿀 파일

### 5.1 `migrations/001_init.sql` — 테이블 추가 (새 002 파일 금지, 001에 합친다)

`sessions` 정의 뒤에 추가한다.

```sql
-----------------------------------------------------------
-- slot_dispatches (scheduler 발송 단계 기록, 재개·수동 재발송용)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS slot_dispatches (
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot        VARCHAR(20) NOT NULL,                 -- model.SessionSlot
    local_date  DATE NOT NULL,                        -- 사용자 timezone 기준 날짜
    due_at      TIMESTAMPTZ NOT NULL,                 -- 보내야 하는 시각, admin retry가 now()로 옮긴다
    state       VARCHAR(20) NOT NULL,                 -- claimed | built | pushed | reminded | empty
    session_id  INT REFERENCES sessions(id) ON DELETE SET NULL,
    attempts    INT NOT NULL DEFAULT 1,
    last_error  TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, slot, local_date)
);

CREATE INDEX IF NOT EXISTS idx_slot_dispatches_resumable
    ON slot_dispatches(due_at) WHERE state IN ('claimed', 'built');
```

dev DB에는 `make migrate`로 적용한다(001 전체가 `IF NOT EXISTS`라 다시 돌려도 된다. 착수 시 한 번 확인).

### 5.2 `internal/model/` — 상태 타입

`SessionSlot` 옆(`internal/model/user.go` 또는 새 파일)에 `SlotDispatch` struct와 `SlotDispatchState` 상수 5개를 둔다. 상태 문자열은 위 SQL 주석과 같다.

### 5.3 `internal/repository/slot_dispatch_repo.go` — 새 파일

import 경계: repository는 `model`만 import 가능. 메서드(이름은 구현 때 정해도 됨):

| 메서드 | SQL 요지 |
|---|---|
| `Claim(ctx, d) (bool, error)` | `INSERT ... ON CONFLICT DO NOTHING RETURNING user_id` → 행이 나오면 true |
| `TakeOverStale(ctx, window, lease, maxAttempts, limit) ([]ResumableDispatch, error)` | `UPDATE slot_dispatches SET attempts=attempts+1, updated_at=now() WHERE state IN ('claimed','built') AND due_at > now()-window AND updated_at < now()-lease AND attempts < maxAttempts RETURNING ...` + `users` join으로 `model.User`까지 돌려준다(`BuildForSlot`이 user를 받음) |
| `MarkBuilt(ctx, key, sessionID)` | `UPDATE ... SET state='built', session_id=$, updated_at=now() WHERE <pk> AND state='claimed'` |
| `MarkDone(ctx, key, state)` | `pushed`/`reminded`/`empty`로. `WHERE <pk> AND state IN ('claimed','built')` |
| `MarkFailed(ctx, key, errMsg)` | 상태는 그대로, `last_error`·`updated_at`만 |
| `DeleteBefore(ctx, date)` | D12 정리 |

`internal/repository/user_repo.go` `GetUsersBySlot`에 `local_date`를 받아 `NOT EXISTS (SELECT 1 FROM slot_dispatches ...)`를 붙인다(D9). 기존 호출처는 `tick` 하나다.

### 5.4 `internal/scheduler/` — claim 교체 + 재개

`scheduler.go`의 `pushClaims` interface를 `slotDispatches` interface(위 메서드)로 바꾼다. scheduler import 경계는 `model`, `observability`뿐이라 interface는 scheduler 쪽에 둔다.

`dispatchUser` before → after 요지:

```go
// before
today := time.Now().Format("2006-01-02")
acquired, err := d.claims.TryClaim(ctx, user.ID, slot, today)
if err != nil {
	slog.WarnContext(ctx, "Redis idempotency lock check error; proceeding", ...)
} else if !acquired {
	return nil
}
... remind / BuildForSlot / pushBuiltSession

// after
acquired, err := d.dispatches.Claim(ctx, key)   // key = user, slot, local_date, due_at
if err != nil {
	return fmt.Errorf("claim dispatch user_id=%d slot=%s: %w", user.ID, slot, err) // D8 fail-closed
}
if !acquired {
	return nil
}
return d.runFromClaimed(ctx, user, key, unfinishedCount)
```

```go
// runFromClaimed: remind 또는 build → MarkBuilt → push → MarkDone(pushed)
// runFromBuilt:   기존 session_id로 push → MarkDone(pushed)
// 각 단계 실패 시 MarkFailed(errMsg) 후 err 반환. 상태는 그대로 남아 다음 tick이 재개한다.
```

`tick` 변경:

1. 지금 칸부터 `lateDispatchWindow`까지 30분 간격 칸들을 만든다. 칸마다 `local_date`·`due_at`을 그 칸의 사용자 timezone 시각으로 계산한다(자정 넘김 주의).
2. 칸마다 지금처럼 `GetUsersBySlot` → `CountUnfinishedBatch` → `dispatchBatch`.
3. 끝에 `TakeOverStale` 결과를 상태별로 `runFromClaimed`/`runFromBuilt`에 넘긴다. 같은 worker 4개·rate limiter를 쓴다.

`Start`: cron 등록 뒤 `go s.runJob("dynamic_user_push", 10*time.Minute, s.tick)`를 한 번 바로 부른다(D10). 정리 job(D12)도 여기서 등록한다(`"0 4 * * *"` 등).

### 5.5 `cmd/server/app.go`

```go
// before
Claims: redisstore.NewPushClaims(rdb),
// after
Dispatches: repository.NewSlotDispatchRepository(db),
```

### 5.6 지우는 파일

- `internal/redisstore/push_claims.go`, `internal/redisstore/push_claims_test.go`
- scheduler 테스트의 `mockPushClaims` → `slotDispatches` fake로 교체

### 5.7 `cmd/admin/slot_dispatches/main.go` — 새 명령

`cmd/admin/reset_learning_data/main.go`처럼 `config.Load` + `bootstrap.OpenDB`만 쓴다. Telegram client는 만들지 않는다(D11).

```
go run ./cmd/admin/slot_dispatches -list [-date 2026-11-09]
    → user_id, slot, state, attempts, due_at, last_error 표로 출력 (기본: 오늘, 끝나지 않은 행 먼저)
go run ./cmd/admin/slot_dispatches -retry -user 2006481393 -slot morning_study -date 2026-11-09
    → state = CASE WHEN session_id IS NULL THEN 'claimed' ELSE 'built' END,
      attempts = 0, due_at = now(), updated_at = 'epoch'
    → 실행 중인 서버의 다음 tick(최대 30분)이 재개한다
```

`pushed` 행도 retry할 수 있다(같은 session을 다시 보냄).

## 6. 검증

- repository(`COPYLINGO_TEST_DATABASE_URL` 있을 때 실제 Postgres, 기존 `session_repo_test.go` 방식):
  - `Claim` 두 번째 호출 false
  - `TakeOverStale`: lease 안 지난 행·window 밖 행·`attempts>=3` 행·끝난 상태 행은 안 가져옴
  - `MarkBuilt`가 기대 상태가 아니면 아무것도 안 바꿈
- scheduler(fake):
  - `claimed` 재개 → build+push 후 `pushed`
  - `built` 재개 → `BuildForSlot` 호출 없이 기존 session push
  - push 실패 → `MarkFailed`, 상태 `built` 유지
  - claim 오류 → build 안 함(D8)
  - lookback 칸 계산: 00:30 local, window 2h → 전날 22:30 칸의 `local_date`가 전날
- 전체: `make test` 통과 → goparams 포맷 → `make restart-app` 후 로그에서 시작 직후 tick 1회 확인
- 수동: dev DB에서 오늘 행 하나를 `claimed`·`updated_at` 과거로 바꾸고 다음 tick에 `pushed`가 되는지 본다

## 7. 손대지 않는 것

- session 편성 로직(`internal/service/session_builder.go`)
- graceful shutdown·`shutdownTimeout`(ADR-085)
- bot update handler의 재시도 (Telegram update 재처리는 별개 문제)
- slot 시각 설정 UX, `users` 테이블 slot 컬럼

## 8. 배포 주의

- 배포 당일 Redis claim으로 이미 보낸 slot은 `slot_dispatches`에 행이 없다. 그래서 배포 시각 기준 window(2h) 안에 있던 slot은 **한 번 더 갈 수 있다**. slot 시각(08:00, 12:00, 16:30, 21:00) 직후 2시간을 피해 배포하거나 1회 중복을 수용한다.
- 처음 켤 때 서버 시작 직후 tick(D10)이 window 안 slot을 바로 보낸다. 의도한 동작이다.
