# TODO: 세션 빌드 시 학습 팁(Tips) AI 생성 통합

## 배경 및 목적

[ADR-015](../ADR.md#adr-015-학습-팁tips-시스템-도입--llm-채점-대기-시간-활용) 에서 결정된 tips 시스템의 **생성 파이프라인** 구현.

- 스키마/모델/repository 는 이미 완료:
  - [migrations/001_init.sql](../../migrations/001_init.sql) — `tips` 테이블 포함
  - [internal/model/tip.go](../../internal/model/tip.go) — `TipCategory` 화이트리스트 7개 정의됨
  - [internal/repository/tip_repo.go](../../internal/repository/tip_repo.go) — `Create`, `ListActive`, `CountActive` 제공
- 본 TODO 는 **scheduler 사이클에서 LLM 호출로 tips 잔고를 50개까지 점진 채워 넣는 로직**을 추가한다.

**목표**: scheduler 가 morning/evening 세션 빌드를 돌 때마다 등장하는 (language, proficiency_level) 조합에 대해, 잔고가 50 미만이면 한 번에 2-3개씩 LLM 으로 새 tip 을 생성·저장한다. 50 도달 후 자동 정지.

---

## 핵심 설계 결정 (변경 금지)

| 항목 | 결정 |
|---|---|
| 잔고 임계치 | `TIP_BUCKET_TARGET = 50` (코드 상수) |
| 1 사이클 생성 수 | 2-3개 (구체 값은 코드 상수 `TIP_GENERATE_PER_CYCLE`, **3 권장**) |
| 트리거 위치 | `internal/scheduler/scheduler.go` 의 `buildAndPushSessions` 안, 세션 빌드/푸시 **이후** |
| 사용자 그룹화 | 활성 사용자 전체에서 등장하는 distinct (language, level) 페어 추출 후 페어 단위 처리 (사용자 수와 무관하게 페어당 최대 1번 LLM 호출) |
| dedup | **없음** (ADR-015 결정) |
| 카테고리 선택 | 7개 enum 에서 균등 회전 (least-recently-generated). 단순화하려면 random.choice 도 허용 — 코드에서 선택 |
| 실패 처리 | LLM 호출 실패 시 해당 페어만 skip, 로그 남기고 다음 세션 사이클을 기다림. retry 없음 |
| LLM 모델 / prompt 버전 | `cfg.LLM.Model` 사용. `source_prompt_ver` 는 코드 상수 `TIP_PROMPT_VERSION = "v1"` 로 시작 |

---

## 변경할 파일

### 1. `internal/external/llm.go` — `GenerateTips` 메서드 추가

기존 `LLMClient` 는 `GradeAnswer`, `GradeHandwriting` 만 갖고 있다. 새 메서드 추가:

```go
type GeneratedTip struct {
    Body string `json:"body"`
}

// GenerateTips asks the LLM for N short learning tips for the given
// (language, level, category). Returns parsed JSON array.
// 카드 UI 의 eyebrow 라벨은 category.DisplayName() 으로 코드 측에서 박히므로
// LLM 은 body 만 생성하면 된다.
func (c *LLMClient) GenerateTips(ctx context.Context, language, level string, category model.TipCategory, n int) ([]GeneratedTip, error)
```

Prompt 의 핵심 요구 (한국어로 작성, model 출력은 JSON):

- 시스템 메시지: "당신은 외국어 학습 팁 작성자입니다. JSON 배열로만 응답하세요."
- 사용자 메시지에 다음을 포함:
  - 대상 언어 / 레벨 / 카테고리 (`category.DisplayName()` 의 한국어 명 + 영문 enum 키를 함께 전달, e.g. "요음 (kana_youon): 히라가나·가타카나의 작은 ゃ, ゅ, ょ")
  - 정확히 N개 생성 요청
  - 각 tip 의 `body` 는 한국어로 1-2 문장 (최대 200자, DB 한도는 500자지만 카드 UI 가독성 우선)
  - 어조: 학습자에게 짧고 명확하게
  - 카테고리 안에서 서로 다른 각도로 작성하라고 명시 (같은 호출 안 내부 중복 회피)
  - 출력 JSON 스키마 예시 명시 (`[{"body": "..."}, ...]`)
- 응답 파싱 시 JSON 추출 (LLM 이 ``` 펜스를 붙일 가능성 대비 trim)

### 2. `internal/service/tip_generator.go` (신규)

scheduler 가 직접 LLM 호출을 들고 있지 않게 service 레이어에 배치 (기존 Service 인터페이스 패턴 참고 — [internal/service/](../../internal/service/) ).

```go
type TipGenerator struct {
    tips repository.TipRepositoryIface  // 인터페이스 추출 권장 — service 단 단위 테스트 용이
    llm  external.LLMClientIface
    log  *log.Logger // 또는 표준 log 사용 (CLAUDE.md §5.6)
}

const (
    TipBucketTarget     = 50
    TipGeneratePerCycle = 3
    TipPromptVersion    = "v1"
)

// TopUpBucket checks the (language, level) bucket and generates up to
// TipGeneratePerCycle new tips if below TipBucketTarget. Idempotent w.r.t.
// the target — once full, this is a no-op.
func (g *TipGenerator) TopUpBucket(ctx context.Context, language, level string) error
```

내부 흐름:
1. `tips.CountActive(ctx, language, level)` 호출
2. `count >= TipBucketTarget` → 즉시 return nil
3. 카테고리 선택 (전체 enum 에서 random 또는 round-robin — 본 TODO 안에서 random 권장; round-robin 은 last_used 추적 필요해 과한 복잡도)
4. `llm.GenerateTips(ctx, language, level, category, TipGeneratePerCycle)` 호출
5. 반환된 각 tip 에 대해 `repo.Create` (실패한 항목은 로그 후 skip, 나머지 진행)
6. 처음부터 끝까지 trace 로그 (`[TipGen] language=ja level=N5 category=... generated=N skipped=M`)

`repo.Create` 호출 시 `Tip` 필드:
- `Language`, `ProficiencyLevel`, `Category` → 입력값
- `Body` → LLM 응답
- `SourceModel` → `cfg.LLM.Model` 포인터
- `SourcePromptVer` → `&TipPromptVersion`
- `IsActive` → `true`

### 3. `internal/service/services.go` — 와이어업

기존 Services 구조체에 `TipGenerator` 필드 추가. `NewServices` 에서 의존 주입.

> grep 으로 정확한 wiring 위치 확인 후 패턴 그대로 따를 것.

### 4. `internal/scheduler/scheduler.go` — `buildAndPushSessions` 후 훅

`buildAndPushSessions` 내부, 사용자 루프 종료 직후 (`for _, user := range users` 끝난 뒤) **distinct (language, level) 페어** 를 추출해 각 페어에 대해 `TipGenerator.TopUpBucket` 호출:

```go
// 세션 빌드/푸시 루프 종료 후
pairs := distinctLangLevelPairs(users)
for _, p := range pairs {
    if err := s.services.TipGenerator.TopUpBucket(ctx, p.Language, p.Level); err != nil {
        log.Printf("[Scheduler] tip top-up failed language=%s level=%s: %v", p.Language, p.Level, err)
        continue
    }
}
```

`distinctLangLevelPairs` 헬퍼는 같은 파일에 둠. `users` 슬라이스를 한 번 순회하며 `map[struct{L,P string}]struct{}` 로 dedup.

**중요**: 세션 빌드 실패와 tip 생성 실패를 분리할 것. tip 생성이 실패해도 세션은 정상 푸시되어야 한다.

### 5. (선택) repository 인터페이스 추출

[ADR-015 (Phase 2.5)](../../docs/workthrough/2605091440_service_layer_refactoring.md) 흐름에 따라 service 단 단위 테스트가 강조되어 있다면, `TipRepository` 인터페이스를 `internal/service/interfaces.go` (또는 기존 인터페이스 정의 파일) 에 추가해 `TipGenerator` 가 인터페이스에 의존하게 할 것.

> Phase 2.5 패턴 따라가는 게 안전. 기존 service 들의 의존 패턴을 그대로 베껴 일관성 유지.

---

## 검증

```bash
go build ./...
make test
```

추가 작성할 단위 테스트 (Phase 2.5 service-layer 테스트 패턴):

- `internal/service/tip_generator_test.go`:
  - bucket 이 이미 50 이상이면 LLM 호출 없이 종료 (`llm.GenerateTips` mock 이 0회 호출)
  - bucket 이 50 미만이면 LLM 호출 1회 + repo.Create 가 반환 수만큼 호출
  - LLM 이 비어 있는 배열 반환 시 repo.Create 0회, 에러 아님
  - LLM 이 에러 반환 시 에러 전파 (scheduler 가 skip)
  - 개별 Create 실패 시 나머지 진행 (부분 성공)

수동 검증 (선택, 비용 발생):
- 로컬 LLM API 키 설정 후 `make run` 으로 scheduler 켜기
- DB 의 `tips` 테이블에 새 row 가 누적되는지 확인
- 50 도달 후 추가 호출 없는지 로그로 확인

---

## 건드리지 말 것

- `migrations/001_init.sql`, `internal/model/tip.go`, `internal/repository/tip_repo.go` — 본 TODO 시작 시점에 이미 확정됨. 시그니처가 부족하면 사용자에게 보고 후 추가.
- 다른 LLM 호출 경로 (`GradeAnswer`, `GradeHandwriting`) — 본 TODO scope 아님.
- 손글씨 채점 흐름 / Mini App / Telegram 봇 메시지 — 별도 TODO scope.
- `cfg.LLM.RPM` / RPD 관리 — 본 TODO 범위 밖. 단, 한 scheduler 사이클의 LLM 호출 수가 distinct pair 수와 동일함을 workthrough 에 기록할 것.

---

## 의사결정 / 결정된 사항

- 카테고리 선택은 **random** (round-robin 의 last_used 추적은 과한 복잡도). 50개가 차오르는 동안 자연스럽게 7개 카테고리에 분산되는 걸 기대. 분산 편향이 심하다고 판명되면 추후 ADR 갱신.
- 실패 시 retry 없음. 다음 세션 사이클 (12시간 후) 이 자연 재시도 역할.
- `source_prompt_ver` 는 코드 상수. prompt 본문 바뀌면 상수 bump (e.g. `v2`). 과거 tip 들과 신규 tip 의 출처 구분 가능.
- scheduler 의 tip top-up 은 **세션 빌드와 같은 트랜잭션이 아니다** — 세션 푸시 후 독립적으로 실행. 한쪽 실패가 다른 쪽에 전파되지 않게 분리.
