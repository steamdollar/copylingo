# Unit Test 커버리지 공백 보강 (01_unit_test_plan)

- **날짜**: 2026-06-30
- **Case**: B(구현/검증) — 테스트만 추가, 운영 코드 무수정
- **TODO**: `docs/todos/01_unit_test_plan.md`
- **요청**: `01_unit_test_plan.md`의 체크리스트 중 **현재 실측상 아직 미커버인 분기/함수만** 식별해 보강. 이미 커버된 항목 중복 작성 금지. 외부 API 실호출 금지(mock).
- **경계**: 병렬 agent가 tip scheduler feature 구현 중 → `internal/external/llm.go`, `internal/service/tip_generator.go`(신규), `internal/service/services.go`, `internal/scheduler/scheduler.go`, `internal/model/tip.go`(운영 코드)·`tip_generator`/`GenerateTips` 테스트에 손대지 않음.

## 핵심 발견 — 문서의 baseline은 이미 무효

`01_unit_test_plan.md` §0의 측정값은 최근 리팩터(integrate/todos-batch1) 이전 수치다. 실측 결과 대부분의 "신규 파일"이 이미 존재·구현돼 있었다.

| 패키지 | 문서 baseline | **실측 baseline** | 보강 후 | 비고 |
|---|---|---|---|---|
| internal/bot | 3.3% | **71.4%** | 71.4% (무변경) | 이미 문서 목표(50%) 초과 — 보강 불필요 |
| internal/service | 78.5% | **75.6%** | **83.2%** | active_session/study_active_session/srs 분기 보강 |
| internal/external | 51.1% | **66.7%** | **79.8%** | llm 에러분기 + nhk transport 분기 보강 |
| internal/model | 0% | **1.2%** | **100.0%** | 로직 보유 타입 전부 커버(최대 공백) |

> ※ worktree가 구(舊) base(`39085c1`)로 생성돼 있어, 작업 전 `integrate/todos-batch1`(88ee0b5)로 fast-forward 후 실측·보강 진행. 이 ff가 없었으면 리팩터된 `external/llm.go`·`service/grader.go`·`service/llm.go`와 다른 코드에 테스트를 붙일 뻔했음.

## 보강 파일 / 케이스

### internal/model (1.2% → 100%) — 최대 가치, 무위험(순수 로직)

문서 §5가 "메서드 있는 타입만"이라 했고, `grep "^func ("`로 확인한 메서드 전부가 미커버였음.

| 파일(신규) | 대상 | 커버한 분기 |
|---|---|---|
| `internal/model/active_session_test.go` | `ActiveSessionState` 7개 메서드 | RecountAnswered(답/미답/혼합/빈), CurrentItemByQuestionID(현재일치/불일치/범위밖), ItemByQuestionID(발견/미발견), ItemAt(경계 4종), NextUnansweredIndex, CorrectCount, WrongAnswers(순서/빈슬라이스 non-nil) |
| `internal/model/study_active_session_test.go` | `StudyActiveSessionState` 7개 메서드 | RecountStudied, CaptureInitiallyStudied, ItemByOrder(발견/미발견), ItemAt, NextUnstudiedIndex, MarkStudied(정상/이미학습/미지order), NewlyStudiedMaterialIDs(초기학습제외/nil맵/빈결과) |
| `internal/model/question_test.go`(기존에 추가) | `Question.GetOptions` | JSONB 배열파싱/빈배열/타입불일치 에러/nil options 에러 |
| `internal/model/session_test.go`(신규) | `SessionMode.IsValid` | quiz/study/빈/미지 |
| `internal/model/tip_test.go`(신규) | `TipCategory.DisplayName`, `AllTipCategories` | 7개 카테고리 라벨 + 미지 fallback(drift guard), 화이트리스트 7건/중복없음 |

> `model/tip.go`는 **운영 코드 무수정**(테스트만 추가). 경계가 금지한 것은 `tip_generator`/`GenerateTips`(service 레이어 신규)이며, `model/tip.go`의 순수 enum 메서드는 별개·안정 코드라 §5 대상에 해당.

### internal/external (66.7% → 79.8%)

| 파일(신규/기존) | 대상 | 커버한 분기 |
|---|---|---|
| `internal/external/llm_error_test.go`(신규) | `NewLLMClient`, `AnswerLearningQuestion`, `GradeHandwriting`, `GradeAnswer` | NewLLMClient(커스텀 BaseURL/빈 BaseURL→default), AnswerLearningQuestion(미설정/HTTP500/빈choices), GradeHandwriting(미설정/HTTP500/빈choices/JSON파싱실패), GradeAnswer(빈choices) — 전부 httptest mock |
| `internal/external/nhk_client_test.go`(기존에 추가) | `FetchArticleBody` | transport 에러(server 종료 후 요청), article-body div 없음→빈문자열 |

보강 결과 `GradeAnswer`/`GradeHandwriting`/`AnswerLearningQuestion`/`NewLLMClient`는 100%. 실호출 없이 전부 `httptest.Server`.

### internal/service (75.6% → 83.2%)

| 파일(기존에 추가) | 대상 | 커버한 분기 |
|---|---|---|
| `internal/service/active_session_test.go` | `SetCurrentIndex`, `Delete`, `Flush`, `save`, `validateActiveSessionState` | SetCurrentIndex(정상/음수/길이초과/Get전파), Flush(user mismatch/nil repo/repo error), Delete(정상/redis del 에러), save(redis set 에러), validate(version mismatch→Corrupt) |
| `internal/service/study_active_session_test.go` | `Start`, `CreateFromDB`, `Get`, `GetOwned`, `MarkStudied`, `Delete`, `validateStudyOwnerAndMode` | Start(completed 조기반환/pending+starter없음/user mismatch/nil repo), CreateFromDB(정상/load 에러), Get(DB 복구), GetOwned(miss→DB load/user mismatch/mode mismatch), MarkStudied(미지 order), Delete |
| `internal/service/srs_test.go` | `GetDueReviews`, `GetDueCount` | 정상 반환/에러 전파(passthrough이지만 0%였음) |

기존 `fakeActiveSessionRedis`(에러 주입형)·`fakeActiveSessionRepo`·`activeSessionTestState`·`fakeStudyActiveRepo`·`studyActiveState` mock/헬퍼를 **재사용**(문서 §1 원칙 준수). 신규 mock 없음.

## 이미 충분해 스킵한 항목

- **internal/bot 전체**: 실측 71.4%로 문서 목표(50%) 이미 초과. 문서가 지목한 신규 파일(`session_answer_test.go`, `session_question_test.go`, `session_helpers_test.go`, `handler_dispatch_test.go`) 전부 이미 존재·구현됨. 추가 보강은 한계효용 낮아 생략.
- **srs.go `updateSchedule`/`ProcessAnswer`/`ScheduleAnswer`**: 기존 `srs_test.go`가 SM-2 경계(rep 0/1/default, 오답 reset, ease 하한 1.3, repo 에러)를 이미 100% 커버. 중복 작성 안 함.
- **service `tip_test.go`(문서 §3 지목)**: 작성 안 함. `service/tip.go`의 `ListActive`/`CreateCandidate`는 단순 passthrough이고, tip scheduler agent가 이 파일에 `GenerateTips` 등을 확장할 가능성이 높아 경계 충돌·머지 리스크 회피. 가치 대비 위험이 커 의도적 스킵.

## BLOCKED (운영 코드 수정 필요 — 미진행)

문서 제약("운영 코드 절대 무수정")상 아래는 테스트 불가:

- `external.FetchArticleList` (0% 유지) — 목적지 URL이 패키지 상수 `nhkListURL`(하드코딩)이고 `c.baseURL`을 쓰지 않아 `httptest.Server`로 리다이렉트 불가. 운영 코드를 `c.baseURL` 사용으로 바꾸거나 listURL을 주입 가능하게 해야 함. → `02_integration_test_plan`으로 이관 권장.
- `external.FetchArticleBody`의 `create request`·`io.ReadAll` 에러 분기 — 정상 URL/정상 body로는 도달 불가(라이브러리 내부 실패 주입 필요). 잔여 14% 미커버.
- service의 `Complete`/`Delete` flush/delete 에러 래핑 분기 일부(66.7%) — 가치 낮아 미추격.

## 검증

- `go build ./...` 통과.
- `go vet ./internal/model ./internal/external ./internal/service` 클린.
- `go test ./...` 전부 green(기존 테스트 무파손, 신규 포함).
- 운영 코드 `git diff` 비어 있음(`_test.go`만 추가/수정).

## TODO closure 판정

**대부분 이미 완료돼 있던 문서**다. 문서 baseline이 리팩터 이전 수치라, 지목한 핵심 목표(bot 50%+, 신규 테스트 파일들)는 본 작업 착수 시점에 이미 달성·존재했다. 본 작업은 그 위에 남은 model(0%대)·external·service 잔여 공백을 메웠다.

→ **완료로 닫아도 무방**. 단 `external.FetchArticleList`는 운영 코드 구조상 unit 불가(BLOCKED)이므로, 닫을 때 "`FetchArticleList` 커버리지는 integration(`02`)으로 이관"이라는 잔여 1건만 상위 문서에 남기면 깔끔하다. (운영 코드를 건드릴 수 있는 별도 TODO로 처리하거나, 그대로 둬도 회귀 위험은 낮음.)
