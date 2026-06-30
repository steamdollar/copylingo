# Quiz 결과 메시지 문제 원문 보존 + 문제 컨텍스트 LLM 질문 버튼

- **날짜**: 2026-06-28
- **유형**: Case A(설계, ADR-029) → Case B(구현)
- **관련 ADR**: [ADR-029](../../adr/ADR_from_21_to_40.md#adr-029-quiz-결과-메시지에-문제-원문-보존--문제-컨텍스트-llm-질문-버튼)

## 배경 / 문제

Quiz 답변 시 원본 문제 메시지를 `editMessage`로 덮어써 결과(정답/해설)를 보여주는데:

1. 결과 텍스트에 **문제 원문(`prompt`)이 빠져** 있어 답변 후 "무슨 문제였는지" 다시 볼 수 없었다. (해설 `question.Explanation`은 이미 정답/오답 모두 표시 중)
2. 그 자리에서 LLM에게 물어보려 해도 기존 `/llm`(ADR-028)은 **quiz context를 몰라** 문제 원문을 직접 재입력해야 했다.

사용자 결정: "무조건 자동 LLM은 과하다 → 원문은 결과에 보존하고, LLM은 **버튼으로 필요할 때만**(이어지는 flow)".

## 결정 사항 (요약)

- 결과에 문제 원문 prepend (정답/오답 공통). 해설은 기존 그대로.
- 결과에 `🤖 이 문제 질문` 버튼(owner-only). callback = 기존 `q:` prefix의 `ask` sub-action.
- 버튼 → 기존 `llm_pending` 재사용, 값에 `q:{sessionID}:{questionID}` 컨텍스트 토큰 저장 → 다음 plain text 1개를 그 문제 context(원문/정답/해설/사용자 답)로 LLM 답변.
- `external.LLMClient` 인터페이스 불변(컨텍스트 프롬프트는 bot 레이어 조립) → mock/test 영향 0.

## 변경 파일

| 파일 | 변경 |
|------|------|
| `internal/config/constants.go` | `FormatQuestionAskLLM = "q:%d:ask:%d"` 추가. `UserLLMPendingRedisKey` 값 규약 doc 갱신("1" or "q:sid:qid"). |
| `internal/model/active_session.go` | `ItemByQuestionID(questionID)` 추가 — CurrentIndex 무관하게 문제 조회(답변 후 index 이동 대비). |
| `internal/bot/session_answer.go` | `processAnswerText`에 `from *tgbotapi.User` 파라미터 추가. 결과 텍스트에 `📝 {prompt}` prepend. owner면 `🤖 이 문제 질문` 버튼 row 추가. 두 호출부(`processAnswer`=`cb.From`, `HandleTextInput`=`msg.From`) 갱신. |
| `internal/bot/session_flow.go` | `HandleAnswerCallback`에 `parts[2]=="ask"` 분기 + `handleAskLLMQuestion` 추가(owner 재검증 → pending에 컨텍스트 토큰 Set). |
| `internal/bot/llm_question.go` | `handleLLMQuestion`이 `GetDel().Result()`로 pending 값을 읽어 컨텍스트 분기. `loadQuizQuestionContext` 헬퍼 추가(토큰 파싱 → active session에서 문제 로드 → 컨텍스트 블록 조립, miss 시 "" → plain fallback). |
| `internal/bot/llm_question_test.go` | (신규) `loadQuizQuestionContext` 토큰 분기, owner-gated 버튼/원문 prepend, `handleAskLLMQuestion` pending 토큰 테스트. |
| `internal/bot/session_answer_test.go`, `coverage_boost_test.go` | `processAnswerText` 호출부 시그니처(`from` nil) 갱신. |
| `docs/adr/ADR_from_21_to_40.md` | ADR-029 추가. |

## 흐름

```
[문제 풀이] --답--> 결과 메시지(editMessage)
   📝 {원문}
   ✅/❌ + 해설
   [다음 문제 →] [🤖 이 문제 질문]   ← owner만
                       |
            q:{sid}:ask:{qid} callback
                       v
   handleAskLLMQuestion: llm_pending = "q:{sid}:{qid}" (TTL 10m)
                       v
   사용자 질문 입력 --> handleLLMQuestion
       loadQuizQuestionContext("q:{sid}:{qid}")
         → active session에서 원문/정답/해설/사용자답 로드
       AnswerLearningQuestion(컨텍스트 + 사용자 질문)
```

## 검증

- `go build ./...` OK, `go vet ./...` OK
- `make test` 전체 통과 (신규 `llm_question_test.go` 6 subtest 포함)
- `make restart-app` → `http://localhost:8080/health` healthy 확인

## 남은 과제 / 범위 밖

- 문제별 LLM 질문도 owner gate에 의존 → 다중 사용자 확장 시 per-user rate-limit/비용 통제 별도 필요(ADR-029 단점 각주).
- 컨텍스트 모드 질문도 기존과 동일하게 tip candidate로 수집된다(raw 질문 텍스트 저장). 후보는 큐레이션 단계가 있어 그대로 둠.
