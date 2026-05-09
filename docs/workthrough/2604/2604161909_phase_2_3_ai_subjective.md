# Phase 2.3 AI 기반 주관식 채점 구현 (Gemini 연동)

사용자가 텍스트 입력으로 서술형/주관식 정답을 제출하면 AI(Gemini 3.0 Flash) 모델을 통해 유사도를 기반으로 채점 및 피드백을 제공하는 기능을 성공적으로 구현했습니다.

## 🚀 구현 사항 (Changes)

- **의존성 추가**: OpenAI 호환 API 통신을 위해 `sashabaranov/go-openai` 패키지를 추가했습니다.
- **Config & Env** (`internal/config/config.go`): Gemini API 엔드포인트(`https://generativelanguage.googleapis.com/...`)를 가리키도록 `OpenAIConfig.BaseURL`의 기본값을 추가했습니다.
- **AI 클라이언트 계층** (`internal/external/llm.go`): 
  - `LLMClient` 인터페이스 정의
  - 프롬프트 엔지니어링을 통해 AI가 `{'is_correct': true|false, 'feedback': '...'}` 구조의 JSON만 반환하도록 강제(`ResponseFormatTypeJSONObject` 활용)하는 로직을 구비했습니다.
- **주관식 타입 확장** (`internal/model/question.go`): 기존 완전 일치형 주관식(`fill_blank`) 외에 AI 채점이 반영되는 `QuestionSubjective` 문항 타입을 신설했습니다.
- **채점자 서비스 통합** (`internal/service/grader.go`, `services.go`): 
  - `GraderService`에 `LLMClient` 의존성을 주입했습니다.
  - `GradeAnswer` 함수가 `QuestionSubjective` 타입일 경우 AI 채점을 거쳐서 정오답 여부와 **피드백** 문자열을 반환하도록 서명을 업데이트했습니다.
- **텔레그램 UX 개선** (`internal/bot/session_flow.go`):
  - 주관식 텍스트 제출 후(AI 채점 과정 중) 대기 시간 동안 `Typing...` 상태(Chat Action)를 노출시켜 무반응으로 인한 혼동을 줄였습니다.
  - 채점 완료 시 "🤖 **AI 피드백**: ~" 형태의 코멘트가 메시지에 함께 렌더링되게 하였습니다.

## ✅ Verification Results
- Go Test 전체 성공 (`go build ./... && make test`).
- 컴파일러 에러(`NewGraderService` 내부 인자 갯수 불일치 등) 수정 완료 및 정상 빌드 확인.
