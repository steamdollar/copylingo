# TODO: `showQuestion` 반복 DB hit 개선

## 배경 및 목적

`internal/bot/session_flow.go`의 `showQuestion`은 문제를 렌더링할 때마다:
1. `GetSessionQuestions(ctx, sessionID)`로 `session_questions` 전체를 다시 조회
2. 현재 문항의 `GetQuestion(ctx, sq.QuestionID)` 호출

즉 문제 이동마다 최소 2번의 DB read가 발생한다.

**영향**: 현재 세션 규모는 10~15문제로 작아 큰 병목은 아니지만, 텔레그램 callback / 손글씨 Mini App 왕복이 늘어나면 불필요한 DB hit가 누적된다.

**목표**: `showQuestion` 호출 시 DB hit를 최소화한다.

---

## 수정 위치

- `internal/bot/session_flow.go`: `showQuestion`, `isQuestionAnswered`, `nextUnansweredQuestionIndex`
- `internal/service/session_builder.go`: 세션 문항 조회 흐름

---

## 수정 방향 (둘 중 하나 선택, 구현 전 사용자에게 확인)

### 방안 A: Redis/session cache 도입
- 세션 시작 또는 첫 `showQuestion` 호출 시 세션 문항 목록 + 필요한 question 데이터를 함께 로드해 Redis에 저장.
- 이후 문제 이동에서는 cache를 우선 사용.
- cache miss 또는 stale data일 때만 DB 조회.

### 방안 B: JOIN 쿼리 메서드 추가
- repository/service에 `session_questions JOIN questions` 기반 조회 메서드 추가.
- `showQuestion`의 2회 read를 1회 read로 축소.
- 캐시 도입 없이 단순 쿼리 최적화.

**추천**: 현 단계에서는 **방안 B**가 단순하고 stale data 위험이 없어 우선. 캐시는 RPS가 늘어날 때 도입.

---

## 주의사항

손글씨 문항은 Mini App 제출 결과가 비동기로 `session_questions.is_correct`에 기록된다.

- cache를 쓸 경우, 정답 여부 확인(`isQuestionAnswered`, `nextUnansweredQuestionIndex`)이 stale 상태가 되지 않게 다음 중 하나가 필요:
  - DB 재조회로 fallback
  - Mini App 제출 시 명시적 cache invalidation
- 방안 B(JOIN)를 선택하면 매번 DB를 보므로 stale 이슈는 자동 해결.

---

## 수용 기준

- 정상 세션 진행은 기존과 동일하게 동작한다.
- 손글씨 제출 후 `제출 후 다음 문제 →` 버튼이 기존처럼 제출 여부를 정확히 확인해야 한다.
- DB hit 감소 방식과 검증 결과를 workthrough에 기록한다.

---

## 검증 방법

```bash
go build ./...
make test
```

수동 검증:
- 세션 시작 → 객관식/주관식/손글씨 모든 유형의 문제를 끝까지 풀어본다.
- DB 로그(또는 일시적 `log.Printf`)로 문제 이동 시 쿼리 횟수 확인.
- 손글씨 제출 직후 다음 문제 버튼 동작이 정확한지 확인.
