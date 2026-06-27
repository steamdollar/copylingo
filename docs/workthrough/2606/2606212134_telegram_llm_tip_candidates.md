# Telegram `/llm` owner-only 질문 및 tip candidate 수집

## 배경

Telegram에서 `/llm`으로 질문 mode를 활성화하고, 다음 메시지 1개를 LLM에 전달해 답변을 받는 기능을 추가했다. 질문/답변은 추후 학습 tip 후보로 큐레이션할 수 있도록 `tip_candidates`에 저장한다.

## 변경 파일

- `internal/bot/handler.go`, `internal/bot/llm_question.go`: `/llm` command, Redis pending state, owner user ID 배열 check, LLM 답변 전송, candidate 저장.
- `internal/external/llm.go`: 자유 학습 질문용 `AnswerLearningQuestion` 추가.
- `internal/service/llm.go`, `internal/service/services.go`: LLM provider client를 `LLMService`로 감싸 service registry에 연결.
- `internal/model/tip.go`, `internal/service/tip.go`, `internal/repository/tip_repo.go`: `tip_candidates` model과 저장 경로를 기존 Tip domain에 통합.
- `migrations/001_init.sql`: `tip_candidates` 테이블과 조회 index 추가.
- `internal/config/constants.go`: owner Telegram user ID 배열과 Redis key/command 설정 추가.
- `internal/bot/handler_dispatch_test.go`, `internal/config/config_test.go`, `internal/external/llm_test.go`: 주요 flow 테스트 추가.
- `internal/service/study_session_test.go`: 기존 stale hardcoded limit assertion을 구현 상수 기준으로 보정.
- `docs/adr/ADR_from_21_to_40.md`: ADR-028 추가.
- `STATUS.md`: 최근 완료 기록 추가.

## 결정 사항

- `/llm`은 Redis `user:{id}:llm_pending` key로 10분 TTL의 1회성 mode를 제공한다.
- 허용 사용자는 `config.LLMAllowedTelegramUserIDs` 배열로 관리한다.
- 허용 user ID 배열에 포함되지 않으면 LLM 호출과 DB write 없이 return한다.
- 저장 테이블명은 오타 가능성이 있는 `tip_candidiates` 대신 `tip_candidates`로 정리했다.
- raw Q/A는 `tips`가 아닌 `tip_candidates`에 저장한다.
- pass-through에 가까운 `LLMQuestionService`, `TipCandidateService`, `TipCandidateRepository`는 두지 않는다. LLM 호출은 공통 `LLMService`, candidate 저장은 `TipService.CreateCandidate` → `TipRepository.CreateCandidate` 경로로 처리한다.

## 검증

- `go test ./...` 통과.
- `make test` 통과.
- `PGPASSWORD=copylingo make migrate` 통과.
- `make restart-app` 통과, `http://localhost:8080/health` ready 확인.
