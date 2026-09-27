# 설정 소스·파일 정리와 LLM·TTS 설정 통합

2026-09-26 17:07 설정 조사, 17:35 파일 분리, 18:05 TTS 활성화 설정 제거, 18:09 LLM·TTS 설정 통합을 묶었다.

## 미사용 설정 제거와 파일 분리

- `internal/config/config.go`에서 실행 시 읽히지 않는 설정 필드 5개(`tts.cred_path`, `tts.audio_dir`, `tts.language_code`, `schedule.morning_build_cron`, `schedule.evening_build_cron`)와 각 기본값·환경변수 바인딩·검증 항목을 제거했다.
- `config.yaml`에서 같은 키를 제거하고, `.env.example`의 사용되지 않는 `COPYLINGO_OPENAI_API_KEY`를 실제 키인 `COPYLINGO_LLM_API_KEY`로 바꿨다.
- `docs/review_flows/2608241500_project_structure_improvements_review_flow.md`의 관련 체크리스트를 갱신했다. 오디오 디렉터리의 Compose 마운트와 `.dockerignore` 항목은 별도 검토 대상으로 남겼다.
- 앞선 설정 조사에서 `schedule.dynamic_push_cron`을 `bindEnv`에 추가하고 `AutomaticEnv`를 제거했다. YAML 없이 환경변수만 설정한 회귀 테스트를 `internal/config/config_test.go`에 추가했다.
- `internal/config/load.go`로 기존 `Load`와 `bindEnv`를 옮기고 `config.go`에는 타입·검증을 남겼다. 같은 패키지 안의 파일 이동으로 공개 API와 설정 처리 순서는 바꾸지 않았다.

17:07에는 실행 중이던 기존 푸시 cron 4개를 보존했고, 17:35 파일 분리 때도 `CronExpr`와 스케줄 검증을 남겼다. 이 설정과 타입은 이후 [단일 cron·발송 규칙 정리](./2609261755_single_push_cron.md)에서 제거됐다.

## TTS 설정 제거·통합

- 18:05 [ADR-054](../../adr/ADR_from_41_to_60.md#adr-054-tts-활성화-설정을-제거한다): `TTSConfig.Enabled`, 기본값, 환경변수 연결, YAML 항목을 제거했다. `internal/service/services.go`는 API 키가 있으면 청해 음성 서비스를 구성하고, 없으면 기존처럼 `Audio`를 nil로 둔다. `cmd/admin/generate_listening_audio/main.go`도 API 키 유무만 검사한다.
- 18:09 [ADR-055](../../adr/ADR_from_41_to_60.md#adr-055-llm과-tts-설정을-하나의-타입으로-관리한다): TTS 모델·음성 이름을 `LLMConfig`에 합쳤다. `config.go`, `load.go`, `config.yaml`에서 `TTSConfig`와 `tts` 구역을 제거하고 `llm.tts_model`, `llm.tts_voice_name`을 추가했다. 환경변수는 `COPYLINGO_LLM_TTS_MODEL`, `COPYLINGO_LLM_TTS_VOICE_NAME`으로 바꿨다.
- `internal/external/tts_client.go`, `internal/service/services.go`, 관리 명령은 새 필드를 사용한다. `internal/external/errors.go`의 오래된 비활성화 설명과 [기존 청해 ADR](../../adr/ADR-031_032_listening_audio_pipeline.md)의 설정 경로 안내도 갱신했다.
- 채팅의 OpenAI 호환 API와 TTS의 Gemini native API 호출 방식은 유지했다. 이후 추가된 B 화자 설정·음성 저장 키 변경은 [두 화자 청해 작업](./2609261820_two_voice_listening_new_audio.md)에 별도로 기록했다.

## 설정 소스 조사와 다음 결정

이번 작업에서 유지한 우선순위는 공개 URL의 예외를 제외하면 `프로세스 환경변수 > config.yaml > 코드 기본값`이다. `.env`는 로컬 실행에서 환경변수를 채우고, Compose도 `.env`를 치환에 사용한다. `COPYLINGO_SERVER_PUBLIC_BASE_URL`만 로컬 터널 URL 갱신을 위해 `.env` 값이 이미 있는 프로세스 환경변수보다 우선한다. `COPYLINGO_ENV_FILE`과 `COPYLINGO_TUNNEL_TARGET_URL`은 터널 스크립트, `COPYLINGO_TEST_DATABASE_URL`은 저장소 테스트에서 읽으므로 제거하지 않았다.

17:07 조사 당시 YAML과 코드 기본값은 대부분 중복됐지만 `telegram.debug`와 스케줄이 달랐다. YAML은 동적 푸시를 `*/30 * * * *`로 설정하고 기존 cron 4개를 비웠으며, YAML 없는 실행은 반대로 기존 cron 기본값을 활성화했다. 이 스케줄 차이는 이후 ADR-052에서 단일 30분 cron으로 정리됐다.

환경변수와 코드 기본값으로 입력을 통합하고 YAML 읽기·Compose 마운트를 제거하는 방안은 **제안만 했으며 미결정·미구현**이다. 이 방안에서도 `.env`는 로컬 주입 수단으로, DB/Redis/MinIO 호스트명은 Compose 환경변수로 남긴다. 반대로 YAML을 필수로 만들면 단독 이미지에 없는 설정 파일에 대한 배포 의존성이 늘어난다. 이번 정리에서 YAML을 제거한 것으로 해석하지 않는다.

## 검증

- 17:07 기준 설정 필드 37개와 `bindEnv` 키 37개 일치, 제거한 키의 활성 참조 없음 확인. 필드 수는 후속 설정 제거·통합 전의 값이다.
- 17:35 파일 분리 후 `go test ./internal/config` 통과. `internal/config/config_test.go`에는 YAML 없이 환경변수로 로드하는 회귀 검증과 LLM·TTS 기본값·새 환경변수 매핑 검증을 반영했다.
- 네 작업 각각 `make test`, `git diff --check` 통과. 각 작업 후 `make restart-app`과 `http://localhost:8080/health` 정상 응답을 확인했다.
