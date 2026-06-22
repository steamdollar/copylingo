# 2026-06-21 Architecture / Package Refactor 검토

## 결론

현재 구조는 큰 dependency direction 자체는 대체로 정상이다.

`repository -> service -> bot/miniapp/scheduler` 흐름은 유지되고 있고, service 내부에도 테스트용 작은 interface가 있어 단위 테스트 seam은 확보되어 있다.

문제는 "레이어 분리"가 오래 유지되면서 `service`와 `bot`이 flat package로 비대해졌다는 점이다. 기능이 늘어날수록 파일만 같은 package 안에 추가되고, `quiz`, `study`, `handwriting`, `tip`, `ai` 같은 domain boundary가 코드 구조에 드러나지 않는다.

우선순위:

1. `miniapp -> bot` package import 제거
2. `service` God package를 domain package로 분리할지 ADR 결정
3. `handwriting` 또는 `tip`처럼 작은 domain부터 package 분리
4. `quiz active session + grader + srs`를 하나의 quiz domain으로 묶는 리팩터링 검토
5. `docs/ARCHITECTURE.md`와 실제 코드 구조 drift 보정

검증 범위:

- 코드 변경 없음
- `make test` 미실행: 문서 export only

---

## 현재 관찰된 구조

파일 수 / LOC 기준:

| package | 파일 수 | LOC |
|---|---:|---:|
| `internal/bot` | 23 | 4,956 |
| `internal/service` | 26 | 4,139 |
| `internal/repository` | 14 | 1,857 |
| `internal/miniapp` | 4 | 856 |
| `internal/pipeline` | 6 | 735 |
| `internal/model` | 10 | 599 |
| `internal/scheduler` | 2 | 427 |

근거:

- [`internal/service/services.go`](../internal/service/services.go#L12)
- [`internal/bot/handler.go`](../internal/bot/handler.go#L41)
- [`internal/miniapp/handler.go`](../internal/miniapp/handler.go#L16)
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md#L31)

---

## Issue 1. `service` package가 God package로 커지고 있음

**Issue**: `service.Services`가 서로 다른 domain의 service를 한 container에 모두 담고 있다. 현재 `User`, `SRS`, `SessionBuilder`, `StudySession`, `StudyActiveSession`, `ActiveSession`, `Grader`, `Handwriting`, `Analyzer`, `Tip`, `LLM`이 한 package와 한 composition type에 묶여 있다.

근거:

- [`internal/service/services.go`](../internal/service/services.go#L12)
- [`internal/service/services.go`](../internal/service/services.go#L27)

**Options**:

- **A. recommended**: domain package로 분리
  - 예: `internal/quiz`, `internal/study`, `internal/handwriting`, `internal/tip`, `internal/ai`
- **B**: package는 유지하되 파일명 prefix와 interface 정리
  - 예: `quiz_grader.go`, `quiz_active_session.go`, `study_session.go`
- **C. Do nothing**: 현재 flat service package 유지

**Metrics**:

- Option A
  - 구현 공수: 2~4일
  - 리스크: 중간
  - 타 코드 영향도: 높음
  - 유지보수 부담: 크게 감소
- Option B
  - 구현 공수: 0.5~1일
  - 리스크: 낮음
  - 타 코드 영향도: 낮음
  - 유지보수 부담: 소폭 감소
- Option C
  - 구현 공수: 없음
  - 리스크: 단기 낮음, 장기 증가
  - 타 코드 영향도: 없음
  - 유지보수 부담: 계속 증가

**Recommendation**: Option A. 파일 수 자체보다 package cohesion 부족이 핵심 문제다. CopyLingo를 수만~수십만 사용자 규모의 포트폴리오 architecture로 본다면 domain boundary가 코드 구조에 드러나는 편이 낫다.

**Decision**: 사용자 결정 대기.

---

## Issue 2. `miniapp`가 `bot` 구현에 의존함

**Issue**: `internal/miniapp`가 `internal/bot`을 import하고, Redis에 저장된 handwriting message ref를 parsing할 때 `bot.ParseHandwritingMessageRef`를 직접 호출한다. HTTP Mini App ingress와 Telegram Bot ingress는 형제 boundary여야 하므로, 한 ingress가 다른 ingress 구현체를 import하는 구조는 coupling이 강하다.

근거:

- [`internal/miniapp/handler.go`](../internal/miniapp/handler.go#L16)
- [`internal/miniapp/handler.go`](../internal/miniapp/handler.go#L240)
- [`internal/bot/util.go`](../internal/bot/util.go#L10)

**Options**:

- **A. recommended**: shared parser를 `internal/callback` 또는 별도 `internal/telegramref`로 이동
- **B**: `miniapp` 내부에 parser를 중복 구현
- **C. Do nothing**: `miniapp -> bot` import 유지

**Metrics**:

- 구현 공수: 0.5일
- 리스크: 낮음
- 타 코드 영향도: 낮음
- 유지보수 부담: 감소

**Recommendation**: Option A. 가장 작은 atomic refactor이며, BIG CHANGE 전에 먼저 처리해도 안전하다.

**Decision**: 사용자 결정 대기.

---

## Issue 3. `bot` package가 Telegram ingress, flow orchestration, rendering helper를 모두 가짐

**Issue**: `Bot`은 Telegram API wrapper, update dispatcher, session push, message edit helper, flow holder 역할을 함께 수행한다. `SessionFlow`, `StudyFlow`로 일부 분리되어 있지만 package 전체로 보면 presentation rendering, callback dispatch, business flow coordination이 섞여 있다.

근거:

- [`internal/bot/handler.go`](../internal/bot/handler.go#L41)
- [`internal/bot/session_flow.go`](../internal/bot/session_flow.go)
- [`internal/bot/study_flow.go`](../internal/bot/study_flow.go)
- [`internal/bot/llm_question.go`](../internal/bot/llm_question.go)

**Options**:

- **A. recommended**: `internal/telegram` boundary와 domain flow를 분리
  - 예: Telegram adapter는 send/edit/callback parsing만 담당
  - quiz/study flow는 domain package 또는 application package로 이동
- **B**: 현재 package 유지, flow별 파일 group과 helper 정리만 수행
- **C. Do nothing**: 현재 bot package 유지

**Metrics**:

- Option A
  - 구현 공수: 2~3일
  - 리스크: 중간
  - 타 코드 영향도: bot/miniapp/scheduler/test 전반
  - 유지보수 부담: 크게 감소
- Option B
  - 구현 공수: 0.5~1일
  - 리스크: 낮음
  - 타 코드 영향도: 낮음
  - 유지보수 부담: 소폭 감소
- Option C
  - 구현 공수: 없음
  - 장기 변경 비용 증가

**Recommendation**: Option A는 `service` domain 분리와 같이 설계해야 한다. 단독으로 먼저 크게 뜯으면 package 이동만 많고 domain boundary가 애매해질 수 있다.

**Decision**: 사용자 결정 대기.

---

## Issue 4. `docs/ARCHITECTURE.md`가 실제 architecture를 덜 반영함

**Issue**: architecture 문서의 layer 설명은 초기 구조에 가깝고, 현재 추가된 Study Session, Redis ActiveSession Working Set, Handwriting Mini App, Tip, LLM question path가 충분히 반영되어 있지 않다.

근거:

- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md#L31)
- [`internal/service/study_active_session.go`](../internal/service/study_active_session.go)
- [`internal/service/handwriting.go`](../internal/service/handwriting.go)
- [`internal/service/tip.go`](../internal/service/tip.go)
- [`internal/bot/llm_question.go`](../internal/bot/llm_question.go)

**Options**:

- **A. recommended**: refactor 결정 후 `docs/ARCHITECTURE.md`를 domain architecture 기준으로 갱신
- **B**: 현재 구조 기준으로만 문서 drift를 보정
- **C. Do nothing**: 문서 drift 유지

**Metrics**:

- 구현 공수: 0.5~1일
- 리스크: 낮음
- 타 코드 영향도: 없음
- 유지보수 부담: 감소

**Recommendation**: Option A. 먼저 구조 방향을 정하고 문서를 갱신해야 다시 drift가 생기지 않는다.

**Decision**: 사용자 결정 대기.

---

## 권장 진행안

### SMALL CHANGE

목표: coupling 감소부터 시작한다.

1. `bot.ParseHandwritingMessageRef`를 shared package로 이동
2. `miniapp -> bot` import 제거
3. 관련 테스트 이동/정리
4. `make test`

Pros:

- 리스크 낮음
- 리뷰 쉬움
- 바로 merge 가능한 atomic change

Cons:

- `service` God package 문제는 그대로 남음
- 전체 architecture 개선 효과는 제한적

### BIG CHANGE

목표: domain boundary를 코드 구조에 반영한다.

1. ADR 작성: layer-first에서 domain-oriented internal package로 전환
2. target package map 확정
3. `tip` 또는 `handwriting`부터 package 분리
4. `quiz` domain으로 `srs`, `active_session`, `grader`, `session_builder` 정리
5. `study` domain으로 `study_session`, `study_active_session`, `working_set` 정리
6. `docs/ARCHITECTURE.md` 갱신
7. `make test`

Pros:

- 장기 유지보수성 개선 큼
- 포트폴리오 관점에서 architecture 의도가 명확해짐
- 신규 기능 추가 위치가 명확해짐

Cons:

- import churn 큼
- 테스트 mock 정리 필요
- 중간 상태에서 리뷰 부담이 커질 수 있음

---

## Opinionated Recommendation

권장안은 **SMALL CHANGE 먼저, 이후 BIG CHANGE ADR**이다.

이유:

1. `miniapp -> bot` import 제거는 독립적이고 즉시 이득이 있다.
2. domain package 분리는 ADR 없이 바로 들어가면 이동 범위가 커진다.
3. `handwriting`, `tip`, `quiz`, `study`, `ai` 중 어디까지를 domain package로 볼지 먼저 합의해야 한다.

사용자 결정 필요:

- **A. SMALL CHANGE 진행**: `miniapp -> bot` coupling 제거부터 구현
- **B. BIG CHANGE 설계**: ADR/package map부터 작성
- **C. Do nothing**: 현재 구조 유지
