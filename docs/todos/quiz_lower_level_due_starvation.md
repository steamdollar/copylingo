# 정규 Quiz에서 하위 레벨 due 복습이 영원히 뽑히지 않는 버그

> Case C TODO. 2026-10-13 통계 진도 작업 중 발견. 사용자가 "지금은 놔두고 기록만" 결정.

## 배경

[ADR-050](../adr/ADR-050_current_level_quiz_focus.md)의 규칙:

- 정규 Quiz(아침 17문항, 저녁 12문항)의 최소 80%(아침 14, 저녁 10)를 현재 레벨로 채운다.
- **남은 기본 자리(아침 3, 저녁 2)는 하위 레벨 due 복습으로 채운다.**

실제로는 하위 레벨 due가 한 문제도 나오지 않는다.

### 증거 (dev DB, user 2006481393, 현재 레벨 N4)

| 항목 | 값 |
|---|---|
| 마지막으로 N5 Quiz 문제가 나온 날 | 2026-09-28 |
| 2026-09-29 이후 Quiz의 N5 문제 | 0개 |
| 지금 밀린 N5 due 문제 | 512개 (가장 오래된 due 2026-09-10) |
| 지금 밀린 N4 due 문제 | 125개 |

## 원인

[`internal/service/session_builder.go`](../../internal/service/session_builder.go) 정규 Quiz 편성:

```go
// L275: due를 한 번만, 세션 크기+2개만 가져온다
due := loadDue(totalQuestions + 2)
...
// L428 부근: 하위 레벨 due를 같은 due 목록 안에서 찾는다
appendDue(
	totalQuestions,
	func(q model.Question) bool {
		return isLowerAdjacentQuestion(q, language, level)
	},
)
```

`loadDue` → `QuestionRepository.GetDueReviews`의 SQL(`dueReviewsForStudiedMaterialsQuery`)은
`level_priority`(현재 레벨 0, 그 외 1)로 먼저 정렬한 뒤 `LIMIT`을 건다.
그래서 현재 레벨 due가 `totalQuestions + 2`개(아침 19, 저녁 14) 이상이면 `due` 목록이 전부 현재 레벨이다.
하위 레벨 pass는 빈 후보에서 찾게 되어 아무것도 넣지 못한다. 남은 자리는 이어지는 `fillNew(currentLevels, ...)`가 현재 레벨 신규로 채운다.

```
N4 due 125개 ──level_priority 정렬·LIMIT 19──▶ [N4 x19] ──isLowerAdjacent──▶ 0개
N5 due 512개 ─────────────────────────────────▶ (조회되지 않음)
```

## 수정 방향 (결정됨)

- 하위 레벨 due pass는 **하위 인접 레벨로 범위를 좁힌 별도 조회**로 후보를 가져온다. 한 번 가져온 `due` 목록을 재사용하지 않는다.
  - 예: `srsService`/`srsScheduler`에 level 범위를 받는 조회를 추가하거나, 기존 `GetDueReviews`에 범위를 넘길 수 있게 한다. Repository SQL은 이미 `levels []string`(`$3`)을 받으므로 SQL 변경은 필요 없다.
  - 하위 인접 레벨 목록은 `sessionLevelsFor`의 순서를 따른다(`isLowerAdjacentQuestion`과 같은 기준).
- `LIMIT`만 늘리는 방식은 쓰지 않는다. 현재 레벨 due가 그보다 많으면 같은 문제가 다시 생긴다.
- 80% 목표, reserved listening/reading, kanji recall 상한 등 ADR-050의 나머지 규칙은 바꾸지 않는다.
- 밀린 N5 512개를 한꺼번에 소화하는 별도 정책은 넣지 않는다. 아침 3, 저녁 2 자리로 자연 소화한다.

## 변경 대상

- `internal/service/session_builder.go` — 하위 레벨 due pass의 후보 조회
- `internal/service/srs.go` — 필요하면 level 범위를 받는 조회 추가
- `internal/service/session_builder_level_policy_test.go` — 회귀 테스트

## 검증

- 회귀 테스트: 현재 레벨 due가 `totalQuestions + 2`개보다 많고 하위 레벨 due도 있을 때, 정규 Quiz에 하위 레벨 due가 `totalQuestions - currentTarget`개(아침 3, 저녁 2) 들어가는지 확인.
- 기존 ADR-050 테스트(현재 레벨 80% 확보 등) 통과.
- `make test`.
- 수정 후 dev DB에서 다음 Quiz 세션에 N5 문제가 섞여 나오는지 확인.

## 손대지 말 것

- Study 신규 자료의 현재 레벨 우선 정책(ADR-047). 사용자가 2026-10-13에 유지하기로 했다.
- `dueReviewsForStudiedMaterialsQuery`의 정렬 기준.
