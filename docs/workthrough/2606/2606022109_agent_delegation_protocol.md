# Native Spawn 및 Gemini CLI External Delegation Protocol 문서화

## 배경

N5 Vocabulary Material Catalog 확장에서 Gemini CLI를 반복 호출했다.
대량 content 생성 자체는 위임할 수 있었지만 provider capacity 오류, Tool Call 오류, partial edit 검수 비용이 발생했다.
이 방식은 runtime native subagent spawn이 아니라 별도 OS process를 실행하는 external delegation이다.
다음 위임 작업에서 두 방식을 혼동하지 않도록 운영 규칙을 문서화했다.

## 변경 사항

- `docs/GEMINI_CLI_DELEGATION.md`
  - Gemini CLI external delegation을 위한 caller protocol로 분리하고 agent 친화적인 영어로 작성
  - 위임 대상과 제외 대상 정의
  - 기본 model `gemini-3.1-flash-lite`, fallback `gemini-2.5-flash` 정의
  - 간결한 TODO 문서 기반 prompt 규칙 추가
  - `429`, `503` 재시도와 Tool Call 오류 복구 절차 분리
- `docs/GEMINI_CLI_EXECUTION.md`
  - 호출된 Gemini CLI executor가 매번 읽는 최소 실행 contract 추가
  - 범위 준수, partial edit 확인, 자체 검증, 완료 보고 책임 정의
- `docs/NATIVE_SUBAGENT_DELEGATION.md`
  - Runtime native child-agent spawn을 기본 위임 방식으로 명시
  - Gemini CLI external delegation과 경계를 분리
- `AGENTS.md`
  - SSOT에서 caller protocol과 executor contract 링크 추가
- `STATUS.md`
  - 최근 완료 항목 추가

## 결정 사항

- Provider 일시 장애는 5초, 10초 대기 후 재시도하고 이후 fallback model로 전환한다.
- 빈 stream, malformed Tool Call, `replace` 필수 인자 누락은 throttle로 취급하지 않는다.
- Tool Call 오류 뒤에는 partial edit를 확인하고 새 session에서 복구 작업을 위임한다.
- 같은 Tool Call 오류가 반복되면 긴 source file 직접 편집을 중단하고 독립 artifact와 validator 방식으로 전환한다.
- Gemini CLI 호출 prompt는 `docs/GEMINI_CLI_EXECUTION.md`와 task TODO 문서만 가리키도록 짧게 유지한다.
- Wrapper 자동화는 별도 TODO `docs/todos/future_gemini_cli_invocation_stabilization.md`로 분리한다.

## 검증

문서-only 변경이므로 `make test`는 실행하지 않는다.

```bash
git diff --check
```

- `multi_agent_v1.spawn_agent`로 native child agent connectivity smoke test 실행
- Child agent에 정확히 `hi`만 반환하도록 요청
- 응답 `hi` 확인 후 `close_agent`로 종료
