# 청해(Listening) 음성 파이프라인 구현 (Case B)

- **날짜**: 2026-07-01
- **설계 근거**: [ADR-031/032](../../adr/ADR-031_032_listening_audio_pipeline.md) (Gemini native TTS + S3 object storage). 본 작업 중 ADR-032에 스키마 amendment 추가.
- **범위**: 청해 문항의 음성을 Gemini 2.5 native TTS로 사전 생성 → S3(로컬 MinIO)에 content-addressed 캐싱 → Telegram으로 전송하는 파이프라인. **문항 콘텐츠 시드는 범위 밖(후속 TODO)**.

## 결정 (사용자 승인)

1. **스키마**: 신규 `002_*.sql` 없이 `001_init.sql`을 멱등(`ADD COLUMN IF NOT EXISTS`) 수정해 `audio_script`·`audio_file_id` 2컬럼 추가. (ADR-032 "스키마 변경 0"의 범위 정정 — amendment 기록)
2. **문항 시드**: 이번엔 파이프라인만, 실제 청해 문항 작성은 Case C TODO로 분리([docs/todos/listening_question_seed.md](../../todos/listening_question_seed.md)).

## 변경 파일

**config / infra**
- [config.go](../../../internal/config/config.go) — `TTSConfig.Model` 추가, voice 기본값 `Kore`로 교체, `StorageConfig`(endpoint/region/bucket/access·secret key/use_path_style) 신설 + defaults + bindEnv
- [config.yaml](../../../config.yaml) — tts.model + storage 섹션
- [docker-compose.yml](../../../docker-compose.yml) — `minio` 서비스 + `minio-createbucket` one-shot(mc mb) + app에 `COPYLINGO_STORAGE_*` env + `minio_data` 볼륨

**external (신규 provider 계층)**
- [tts_client.go](../../../internal/external/tts_client.go) — `TTSClient` interface + `GeminiTTSClient`: native `generateContent`(`responseModalities:["AUDIO"]`+`speechConfig`, net/http 직접, `x-goog-api-key`) → base64 PCM 24kHz/16bit/mono → **ffmpeg subprocess로 OGG/Opus transcode**. transcoder는 필드 주입(테스트에서 ffmpeg 없이 대체). native base URL은 LLM compat base(`.../v1beta/openai/`)에서 파생.
- [audio_store.go](../../../internal/external/audio_store.go) — `AudioStore`(`Exists`/`Put`/`Get`) aws-sdk-go-v2 단일 S3 client(endpoint swap + path-style) + `AudioKey(lang,voice,script)=tts/{lang}/{voice}/{sha256(script)}.ogg`
- [errors.go](../../../internal/external/errors.go) — `ErrTTSConfigMissing`

**schema / model / repo**
- [001_init.sql](../../../migrations/001_init.sql) — questions에 `audio_script`·`audio_file_id`(멱등 ALTER) + listening pending-audio 부분 인덱스, `audio_path` 주석 갱신
- [question.go](../../../internal/model/question.go) — `AudioScript`·`AudioFileID` 필드
- [question_repo.go](../../../internal/repository/question_repo.go) — batch insert/upsert에 `audio_script` 편입(14→15컬럼), `GetNewQuestions`에 `category<>'listening' OR audio_path IS NOT NULL` 가드, `GetListeningNeedingAudio`/`SetAudioPath`/`SetAudioFileID`

**service / scheduler**
- [audio.go](../../../internal/service/audio.go) — `AudioService`: `TopUpAudio`(cycle당 5개, dedup=Exists 후 synth·put, `audio_path` 세팅) + `GetClip` + `CacheFileID` (TipGenerator 패턴 미러, best-effort)
- [services.go](../../../internal/service/services.go) — `Audio` 필드 배선(TTS enabled + API key 있을 때만, 아니면 nil)
- [scheduler.go](../../../internal/scheduler/scheduler.go) — session push 후 `topUpAudio`(distinct (lang,level)별, 실패 skip)
- [session_builder.go](../../../internal/service/session_builder.go) — `defaultCategoryOrder`에 listening 편입

**bot 렌더**
- [handler.go](../../../internal/bot/handler.go) — `SendVoiceFileID`/`SendVoiceBytes`(업로드 후 file_id 반환)
- [session_question.go](../../../internal/bot/session_question.go) — `renderByType`에 `QuestionListening` case(voice 별도 전송 + comprehension 질문/보기 렌더), MCQ 키보드 빌더 `buildMCQKeyboard`로 추출·공유, `sendListeningAudio`(file_id 우선 → 무효/부재 시 store fetch+업로드+file_id 캐시). 채점은 MCQ 경로 무수정 재사용.

**tests** — `audio_test.go`, `tts_client_test.go`, `audio_store_test.go` 신규 + `question_repo_test.go`/`session_question_test.go`/`test_common_test.go` 갱신.

## 신규 의존성

- `aws-sdk-go-v2/{aws,credentials,service/s3}` + `smithy-go` (ADR-032 채택). `go mod tidy` 완료.
- **ffmpeg** (외부 바이너리, ADR-031) — 런타임 transcode 필수. **현재 개발 머신 미설치** → 라이브 TTS 스모크 전 `sudo apt install -y ffmpeg` 필요. prod Dockerfile에도 추가 필요(미반영, 별도 처리).

## 검증

- `go vet ./...` clean, `gofmt` clean(내 파일), **`make test` 전체 통과**(신규 26+ 케이스 포함).
- **MinIO S3 코드 경로 실왕복 검증**(일회성 프로그램, 사용 후 삭제): 로컬 MinIO 기동 → `Exists=false → Put → Exists=true → Get 바이트 일치` + content-addressed key 정상 확인. ADR-032의 "로컬 MinIO로 prod S3 코드 경로 검증" 목표 달성.
- **미검증(후속)**: 라이브 TTS 합성→transcode→sendVoice **end-to-end**. 사유 = (a) ffmpeg 미설치, (b) 실 Gemini API 키 + 청해 문항 시드(후속 TODO) 필요. 절차는 [listening_question_seed.md](../../todos/listening_question_seed.md) "검증 방법"에 기록.

## 후속

- Case C: 청해 문항 시드 작성 → [docs/todos/listening_question_seed.md](../../todos/listening_question_seed.md)
- ffmpeg: 로컬 설치 + prod Dockerfile 반영
- prod storage provider 최종 확정(배포 시점, ADR-032 미결 항목)
