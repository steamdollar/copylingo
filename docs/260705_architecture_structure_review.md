# 2026-07-05 구조 검토 (260621 리뷰 delta 포함)

> 검토 방식: main agent 종합 + 병렬 sub-agent 3개(아키텍처 분석 / 코드품질·컨벤션 후보 / 테스트·성능·DB 후보).
> 핵심 주장은 main agent가 직접 spot-check로 검증했고, 미검증 항목은 본문에 표기했다.
> 기준 문서: [`260621_architecture_package_refactor_review.md`](260621_architecture_package_refactor_review.md) (이전 구조 리뷰), [`AGENTS.md`](../AGENTS.md) §4 (수만 유저 가정 + right-sizing 판단 기준).

## 결론

**Layering은 건강하다 — 의존 방향 위반 0건.** `repository -> service -> bot/miniapp/scheduler` 방향 유지, `model`은 internal 무참조, 06-22에 제거한 `miniapp -> bot` coupling 재발 없음. 신규 audio/tip 코드는 narrow consumer interface로 seam을 잘 잡았고, ADR-031/032(listening audio pipeline)는 repo에서 가장 잘 설계된 부분이다.

문제는 두 축:

1. **260621 리뷰에서 "결정 대기"로 남긴 이슈(1: service God package, 3: bot 혼재, 4: 문서 drift)가 2주간 전부 심화됐다.** service package 26f/4,139 LOC → 30f/5,595 LOC(`Services` 11→13필드), bot +817 LOC, ARCHITECTURE.md drift 그대로.
2. **신규 미결 스케일 축: scheduler가 숨은 orchestration 계층이 됐다.** 전 유저 순차 fan-out + push job timeout 무제한. §4의 수만 유저 가정에서 유일하게 ADR 없이 방치된 축이며, 코드 스스로 TODO로 인지 중.

추가로 잠재 버그 1건(SRS due review의 language 무스코프)과 repository 레이어 coverage 7.5% 공백을 확인했다.

우선순위:

1. [Case B] SRS due review language 스코프 픽스 (Issue 4)
2. [Case B] bot 전송 에러 무시 44곳 처리 (Issue 3)
3. [Case A] domain package 전환 ADR — `tip`/`audio` 선추출 (Issue 2 = 260621 Issue 1 재확정)
4. [Case A] scheduler fan-out 방향 ADR (Issue 1)
5. [Case B] 문서 drift 3건 보정 (ARCHITECTURE.md / CONVENTIONS.md 모순 / callback 규약 누락)

검증 범위:

- 코드 변경 없음
- `make test` 미실행: 문서 export only

---

## 260621 리뷰 이슈 현재 상태

| 이슈 | 06-21 | 2026-07-05 | 상태 |
|---|---|---|---|
| **Issue 1** service God package | 26f / 4,139 LOC (`Services` 11필드) | 30f / 5,595 LOC (13필드), prod 17f/2,216 | **GROWN** — +4파일/+1,456 LOC, +2 domain(audio, tip-gen) |
| **Issue 2** miniapp→bot coupling | 해결(06-22) | miniapp import에 bot 없음 (재검증) | **RESOLVED 유지** |
| **Issue 3** bot 혼재 | 23f / 4,956 LOC | 23f / 5,773 LOC, prod 9f/2,370 | **GROWN(LOC)** — 파일 수 동일, 기존 파일 비대화. audio transport가 `session_question.go`/`handler.go`에 in-line |
| **Issue 4** ARCHITECTURE.md drift | drift | 여전히 stale | **UNCHANGED** — audio pipeline/S3/MinIO/tip-gen/청해 미반영, 다이어그램에 OpenAI/Google TTS로 오기, deps 표에 aws-sdk-go-v2 누락 |
| (신규) scheduler 축적 | 2f / 427 LOC | 2f / 500 LOC, prod 414 | **GROWN** — fan-out + top-up orchestration 축적 |
| (신규) external audio adapters | — | tts_client(240) + audio_store(132) | 신규 — 배치 적절 |

핵심: pending 이슈 3건 전부 개선 없이 심화. 단 Issue 1은 "코드 품질 하락"이 아니라 "정상 품질 코드가 flat package에 쌓임"이라, domain seam이 이미 interface 레벨에 존재해 **분리 난이도는 오히려 낮아졌다**.

---

## Issue 1. scheduler 순차 fan-out — un-ADR'd 스케일 축 (우선순위 최상)

**Issue**: [`internal/scheduler/scheduler.go`](../internal/scheduler/scheduler.go#L211)의 `buildAndPushSessions`(211–297)가 `GetAllUsers`([`internal/repository/user_repo.go:92-96`](../internal/repository/user_repo.go#L92), LIMIT/페이지네이션 없음)를 메모리에 올려 **단일 goroutine 순차 루프**로 유저당 세션 빌드(DB 최대 ~9회 왕복) + Telegram push를 실행한다. push 계열 job 4개가 전부 `runJob(..., 0, ...)` = **timeout 무제한**([scheduler.go:70](../internal/scheduler/scheduler.go#L70), :91, :110, :129 — 직접 확인). 추가로 `distinctLangLevelPairs` 집계, `topUpTips`/`topUpAudio` 오케스트레이션 등 business logic이 scheduler에 축적 중 — cron이 "트리거"가 아니라 숨은 application-service 계층이 됐다. [scheduler.go:217](../internal/scheduler/scheduler.go#L217) TODO에 개발자 본인이 인지한 문제.

스케일(수만 유저) 리스크 3가지:

- (a) 사이클 시간이 유저 수에 선형 — 다음 push 주기와 겹칠 위험
- (b) 수평 확장 불가 — 인스턴스 2개면 leader-election/lock 부재로 **중복 push**
- (c) 루프 중 Telegram/LLM 1건 hang이 사이클 전체 stall (timeout 0과 결합)

**Options**:

- **A. recommended**: Case A로 ADR 작성 — fan-out orchestration을 application-service로 분리 + per-user timeout·bounded concurrency(worker pool) + 다중 인스턴스 대비 advisory lock. 큐/워커 완전 분리(build enqueue → push worker)는 ADR에서 "지금은 안 한다"를 명시적으로 결정.
- **B**: 최소 방어만 — push job timeout 부여 + per-user context timeout (구조 유지).
- **C. Do nothing**: 단일 유저 현실에서 동작엔 문제없음.

**Metrics**:

- Option A: 공수 1~2일 / 리스크 중간 / 포트폴리오 신호 큼 (right-sizing 결정 기록 자체가 산출물)
- Option B: 공수 0.5일 / 리스크 낮음 / 구조 문제는 잔존
- Option C: 스케일 스토리에 구멍

**Recommendation**: Option A. §4 판단 기준에서 유일하게 결정 기록 없이 남은 스케일 축이다.

**Decision**: 사용자 결정 대기.

---

## Issue 2. service God package 회귀 — 260621 Issue 1의 방치 비용 발생

**Issue**: [`internal/service/services.go:12-26`](../internal/service/services.go#L12)의 `Services`가 13필드로 증가(직접 확인). audio·tip-generation 2개 domain이 boundary 없이 flat 편입됐다. 단, **신규 코드의 내부 seam은 우수하다** — [`internal/service/audio.go:20-36`](../internal/service/audio.go#L20)이 자체 narrow interface 3개(`audioSynthesizer`/`audioObjectStore` 등)를 정의하고, adapter는 [`internal/external/tts_client.go`](../internal/external/tts_client.go)/[`audio_store.go`](../internal/external/audio_store.go)에 정상 배치, 방향은 service→external로 정상, 단위 테스트 완비. [`tip_generator.go:25-38`](../internal/service/tip_generator.go#L25)도 동일 패턴.

즉 "나쁜 코드가 쌓인 게" 아니라 "정상 품질 코드가 flat하게 쌓여서" — `internal/tip`·`internal/audio` package 추출은 import churn만 남고 의존성 재배선이 거의 없다. 지금이 추출 비용이 가장 낮은 시점.

미세 smell: [`audio.go:81`](../internal/service/audio.go#L81)이 `external.AudioKey`(content-addressing 스킴 = domain 로직)를 adapter 계층에서 당겨온다. audio를 package로 뽑을 때 key 함수는 domain 쪽으로.

**Options**:

- **A. recommended**: 260621 리뷰의 pending 결정을 확정(Case A) — domain package 전환 ADR + 재배선 최소인 `internal/tip`·`internal/audio`부터 추출. quiz/study 대형 통합은 후속 단계. bot 분리(260621 Issue 3)는 이 ADR에서 방향만 함께 정하고 구현은 별도 단위.
- **B**: 파일명 prefix 정리만 (260621 Option B).
- **C. Do nothing**: 다음 기능 추가 때마다 회귀 반복.

**Metrics**:

- Option A: 1차분(tip/audio) 공수 1~2일 / 리스크 낮음(seam 기존재) / 유지보수 부담 감소 시작
- Option B: 0.5일 / 근본 문제 잔존
- Option C: 2주간 +1,456 LOC의 회귀가 반복됨 (실측)

**Recommendation**: Option A. 06-21→07-05의 회귀가 "결정을 미루면 비용이 쌓인다"를 실증했다.

**Decision**: 사용자 결정 대기.

---

## Issue 3. bot 전송 에러 전면 무시 + handler.go 책임 혼재

**Issue**: `Bot.SendMessage/EditMessage/SendMessageWithKeyboard`(모두 `error` 반환, [`internal/bot/handler.go:116-185`](../internal/bot/handler.go#L116))의 반환값이 **호출 44곳 전부에서 무시**된다(샘플 직접 확인: [`session_question.go:82`](../internal/bot/session_question.go#L82) 등, `if err :=` 패턴 0건). Telegram 전송 실패 = 유저에게 "봇의 침묵"인데 로그도 없다. 유사하게 `rdb.Set` 결과 무시 3곳([`session_question.go:76`](../internal/bot/session_question.go#L76), :173, [`session_flow.go:318`](../internal/bot/session_flow.go#L318)) — 같은 패키지 내 다른 곳은 `.Err()` 체크를 하고 있어 일관성이 붕괴된 상태. [`session_question.go:64`](../internal/bot/session_question.go#L64)의 `TODO: err handling`이 개발자 스스로 인지한 지점.

구조 측면: [`handler.go`](../internal/bot/handler.go)(671 LOC)에 라이프사이클(`New/Start/Stop`) / 저수준 전송 헬퍼 / update 디스패치+로깅 / 커맨드 핸들러 4책임 혼재 + `log`↔`slog` 혼용(:58, :89).

**Options**:

- **A. recommended**: 에러 무시 44곳을 구조 리팩터와 분리해 선행 픽스(Case B) — 최소한 boundary 1회 로깅(WARN) 정책 적용. handler.go 분리는 Issue 2 ADR에 포함.
- **B**: Issue 2 리팩터 때 한꺼번에.
- **C. Do nothing**.

**Metrics**: Option A 공수 1일 미만 / 리스크 낮음 / 운영 관측성 즉시 개선. Option B는 리팩터 지연 시 무음 장애 기간 연장.

**Recommendation**: Option A.

**Decision**: 사용자 결정 대기.

---

## Issue 4. 잠재 버그 — SRS due review가 language 무스코프

**Issue**: [`dueReviewsForStudiedMaterialsQuery`](../internal/repository/question_repo.go#L150)의 WHERE절에 language/level 조건이 없음을 직접 확인했다. `GetDueReviews(ctx, userID, limit)` 시그니처 자체에 language 파라미터가 없어([`internal/service/srs.go:86`](../internal/service/srs.go#L86)) 세션 빌더가 ja/N5 세션에 **모든 언어의 due 문항**을 끌어올 수 있는 구조. [`session_builder.go:142`](../internal/service/session_builder.go#L142)의 TODO(`language 별로 가져와야 하는거 아닌가?`)가 정확히 이 지점이다. 현재 ja만 존재해 잠재 상태지만, 다국어 스키마(ADR-009)를 이미 깔아둔 프로젝트라 두 번째 언어 추가 순간 실버그가 된다.

참고: `questions`의 SRS 상태가 전역(per-user 아님)인 것은 ARCHITECTURE.md에 문서화된 기존 tradeoff라 재론하지 않는다.

**Options**:

- **A. recommended**: `GetDueReviews`에 language/level 파라미터 추가 + 쿼리 조건 + 테스트 (Case B).
- **B**: TODO 주석만 구체화하고 다국어 착수 시점에 처리 (Case C 분리).
- **C. Do nothing**.

**Metrics**: Option A 공수 0.5일 미만 / 리스크 낮음 / 변경 지점 3곳(interface·repo·session_builder).

**Recommendation**: Option A. 비용이 작고, 잊힐 유형의 버그다.

**Decision**: 사용자 결정 대기.

---

## 그 외 후보 (compact)

> sub-agent 수집 결과. ✓ = main agent 직접 검증, 그 외는 file:line 수집만 된 상태.

| 분류 | 내용 | 심각도 |
|---|---|---|
| 테스트 | **repository coverage 7.5%** — `content/active_session/session_material/session_question/session/tip/user_repo` 등 8개 파일에 테스트 파일 자체가 없음. 있는 테스트도 SQL substring 체크 수준(실쿼리 미검증, sqlmock/testcontainers 부재) — JOIN 오타·컬럼명 오류를 못 잡음 | high |
| 테스트 | **scheduler coverage 21.2%** — `buildAndPushSessions`/`topUpTips`/`topUpAudio`/`buildAndPushStudySessions` 직접 커버 0%. 부분 실패 카운팅, session==nil skip, push 실패 후 top-up 계속 실행 여부 미검증 | high |
| DRY | [`internal/external/llm.go`](../internal/external/llm.go) — `GradeAnswer`/`AnswerLearningQuestion`/`GradeHandwriting`/`GenerateTips` 4개 메서드가 nil 가드→요청 조립→err wrap→Choices 체크→파싱 스캐폴딩 ~300줄 반복. 공통 `callChatCompletion` 헬퍼 추출 후보 | mid-high |
| DB | `ORDER BY RANDOM()` ([`question_repo.go:99-103`](../internal/repository/question_repo.go#L99), 세션당 최대 7회 호출되는 hot read) — 풀 커질수록 선형 이상 비용 | mid |
| DB | `created_at::date = CURRENT_DATE` 캐스팅 predicate 2곳([`session_question_repo.go:111`](../internal/repository/session_question_repo.go#L111), [`session_repo.go:113`](../internal/repository/session_repo.go#L113)) — btree 인덱스로 non-sargable, expression index 또는 range rewrite 필요 | mid |
| DB | `next_review_at IS NULL` 조회 vs 정반대 조건(`IS NOT NULL`)의 partial index (`idx_questions_next_review`) — hot path라 `EXPLAIN ANALYZE` 확인 필요 `[UNKNOWN: 실측 planner 선택 미확인]` | mid-high |
| 에러 | [`question_repo.go:31`](../internal/repository/question_repo.go#L31) — repository 직접 로깅 + 언래핑 없는 bare return (CONVENTIONS 이중 위반) | high |
| 에러 | [`session_builder.go:146`](../internal/service/session_builder.go#L146), :161, :202 — 에러를 stdlib `log`로 찍고 삼킨 채 부분 결과로 진행 (silent degradation + 로깅 혼용) ✓ | mid |
| 문서 | CONVENTIONS.md("표준 log 사용") vs ARCHITECTURE.md("slog JSON Handler 사용") **상호 모순** — 실코드는 24파일 slog / 14파일 stdlib log 혼재 | mid |
| 문서 | callback 규약 SSOT(ARCHITECTURE.md)에 `study:` prefix·`q:{sid}:ask:{n}` 누락 (코드엔 구현됨: [`constants.go:71-75`](../internal/config/constants.go#L71)) | mid |
| 구조 | [`cmd/ja/seeder/main.go`](../cmd/ja/seeder/main.go)(761 LOC, repo 최대 파일) — 함수 분해는 잘 돼 있으나(~40개) 4개 카테고리 question 빌더가 catalog 도메인 로직인데 한 파일 혼재. catalog로 이동 시 seeder는 ~130줄 thin orchestration | low |
| 테스트 | [`internal/callback/callback.go:50-79`](../internal/callback/callback.go#L50) — stale mini-app 콜백 판정 로직 0% 커버 (버그 시 "전 버튼 무효화/전부 유효" 무음 장애) | mid |

참고: STATUS.md의 coverage 기록(external 79.8% / service 83.2%, 06-30 측정)과 실측(57.7% / 81.5%)의 차이는 07-01 audio 코드(tts_client/audio_store — 테스트 존재하나 얕음) 추가 영향으로 추정.

---

## 기각한 대안 / 이미 ADR로 결정돼 재론 불필요

- **audio provider·저장·전송 아키텍처 재론 → 불필요.** ADR-031(Gemini native TTS 사전 생성 + ffmpeg), ADR-032(S3 content-addressed store + Telegram file_id 캐시)가 비용/스케일 분석까지 완료. storage/egress가 유저 수 아닌 distinct 콘텐츠에 비례하도록 설계됨.
- **Redis working-set을 in-flight session SSOT로 쓰고 finish에 DB flush → ADR-024/028 채택 완료.** 유실 시 재구성 tradeoff 명시 수용. (스케일에서의 Redis HA는 별건 — 운영 항목으로만 인지.)
- **ffmpeg 외부 바이너리 의존 → ADR-031이 명시 수용** (순수 Go Opus 인코더 미성숙).
- **정적 catalog vs 런타임 형태소 분석 → ADR-030 결정.** seeder가 정적 catalog를 굽는 구조는 이 결정의 산물.
- **Cloudflare R2 / 로컬 FS store → ADR-032에서 명시 기각.**

---

## 권장 진행 순서

1. **즉시 (Case B, 각 0.5~1일)**: Issue 4 language 스코프 픽스 → Issue 3 에러 무시 44곳 → 문서 drift 3건 보정
2. **Case A ADR 2건**: Issue 2 domain package 전환(tip/audio 선추출) + Issue 1 scheduler fan-out 방향
3. **후속**: repository 테스트 하네스(testcontainers 등) 도입 여부 — coverage 7.5%는 구조 리팩터 전 안전망 관점에서도 의미 있음

**Decision**: 사용자 결정 대기.
