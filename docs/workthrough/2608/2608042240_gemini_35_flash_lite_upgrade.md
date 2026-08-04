# Main LLM Gemini 3.5 Flash-Lite 업그레이드

## 결론

Main LLM을 `gemini-3.1-flash-lite`에서 stable `gemini-3.5-flash-lite`로 변경했다. TTS 전용 모델 `gemini-2.5-flash-preview-tts`는 변경하지 않았다.

## 변경 파일

- `config.yaml`
  - 기본 Main LLM model을 `gemini-3.5-flash-lite`로 변경.
- `.env`
  - 현재 로컬 runtime override `COPYLINGO_LLM_MODEL`을 `gemini-3.5-flash-lite`로 변경.
- `internal/config/config.go`
  - LLM default와 주석을 새 model ID로 갱신.
- `internal/external/llm.go`
  - Gemini 3.x에서 deprecated된 `temperature`를 learning question, handwriting grading, tip generation request에서 제거.
- `internal/config/config_test.go`, `internal/external/llm_test.go`
  - 새 default model과 temperature 미전송 계약을 검증.
- `docs/adr/ADR_from_21_to_40.md`
  - ADR-038 기록.

## 결정 및 범위

- 손글씨 image grading은 `gemini-3.5-flash-lite`의 multimodal input과 strict JSON output을 그대로 사용한다.
- TTS는 별도 native API 경로이므로 이번 작업에서 유지한다.
- 사용자가 아직 추가 작업 중이므로 서버·Docker app을 재시작하지 않았다. 새 설정은 다음 재시작부터 runtime에 반영된다.

## 검증

- `go test ./internal/config ./internal/external` 통과.
- `make test` 통과.
- `git diff --check` 통과.
- 실제 runtime 재시작 및 live Gemini 호출은 의도적으로 실행하지 않았다.
