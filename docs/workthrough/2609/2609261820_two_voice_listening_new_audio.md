# 신규 청해 대화 음성에 A/B 두 화자 적용

## 결정과 변경

- [ADR-056](../../adr/ADR_from_41_to_60.md#adr-056-새-청해-대화-음성은-ab-두-화자로-합성한다)에 따라 앞으로 생성하는 청해 대화에만 A/B 목소리를 적용한다. 이미 생성된 음성은 재생성하거나 DB·Telegram 캐시를 변경하지 않는다.
- `internal/config/config.go`, `internal/config/load.go`, `config.yaml`: 기존 `tts_voice_name`을 A(기본 Kore)로 유지하고 `tts_voice_name_b`(기본 Puck)를 추가했다. 환경변수 이름은 `COPYLINGO_LLM_TTS_VOICE_NAME_B`다.
- `internal/external/tts_client.go`: `「발화」「발화」` 전체가 대화인 스크립트를 A→B→A 순서로 라벨링해 Gemini `multiSpeakerVoiceConfig`로 보낸다. 단일 화자 스크립트는 기존 `voiceConfig`로 보낸다. A/B 라벨은 문제 화면에 표시하지 않는다.
- `internal/service/audio.go`, `internal/service/services.go`, `cmd/admin/generate_listening_audio/main.go`: 새 음성의 저장 키에 A/B 목소리를 함께 포함해 기존 단일 목소리 클립을 재사용하지 않도록 했다. 저장 경로가 이미 있는 문항은 기존 조회 조건에 따라 생성 대상에서 제외된다.
- `internal/config/config_test.go`, `internal/external/tts_client_test.go`, `internal/service/audio_test.go`, `internal/bot/session_question_test.go`: 새 설정, 대화 요청, 저장 키, 생성자 변경을 검증했다.

## 검증

- `make test` 통과.
- `git diff --check` 통과.
- `make restart-app` 통과, `/health` 준비 상태 확인.
- 실제 Gemini 음성 품질은 API를 호출하지 않고 요청 본문 테스트로 확인했다.
