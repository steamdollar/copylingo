# CopyLingo 의사결정 기록 (ADR) — 청해(Listening) 음성 파이프라인

> 이 문서는 ADR 시리즈(`ADR_from_21_to_40.md`)의 연속 번호(ADR-031, ADR-032)를 담는 **별도 파일**이다.
> 청해 기능의 음성 생성·저장·전송 아키텍처는 결정 범위가 커서 range 파일에 인라인하지 않고 분리했다.
> 두 결정은 하나의 기능(청해)에 속하지만 관심사가 다르므로(생성 provider vs 저장/전송 아키텍처) 각각 독립 ADR로 인용 가능하게 분리한다.

---

## ADR-031: 청해 음성(TTS) 생성은 Gemini 2.5 native TTS로 사전 생성한다

- **날짜**: 2026-07-01
- **상태**: 채택됨
- **맥락**:
  - JLPT 4대 스킬 중 청해(listening)만 완전 공백이다. `QuestionListening` / `CategoryListening` / 청해 Skill 5종([internal/model/question.go](../../internal/model/question.go))과 `TTSConfig`([internal/config/config.go](../../internal/config/config.go))·`Question.audio_path` 컬럼은 이미 스캐폴딩돼 있으나 실제 TTS client·생성·렌더·채점은 전무하다.
  - 문항 텍스트(청해 스크립트)를 음성으로 합성할 provider가 필요하다. 프로젝트는 §4.4에 따라 무료 quota 우선, `internal/external/llm.go`처럼 provider를 config로 교체하는 철학을 갖는다.
  - 기존 chat LLM은 Gemini를 **OpenAI 호환 모드**(`base_url` swap)로 호출한다. TTS도 같은 창구로 되는지 확인이 필요했다.
- **결정**:
  - Provider는 **Gemini 2.5 native TTS**(`gemini-2.5-flash-preview-tts`)를 채택한다. 공식 pricing 상 free tier "Free of charge"로 확인됐고, 기존 Gemini API 키를 그대로 재사용한다(신규 계정/billing 셋업 0).
  - 단 **OpenAI 호환 계층은 `/audio/speech`(TTS)를 지원하지 않는다**(chat/embeddings/image/audio-입력까지만). 따라서 기존 `llm.go`의 호환 방식을 재사용하지 못하고, **native `generateContent`**(`responseModalities:["AUDIO"]` + `speechConfig`)로 별도 호출 코드를 `internal/external/tts_client.go`에 구현한다. "키 재사용"이지 "client 재사용"이 아님을 명시한다.
  - TTS 전용 모델은 chat 모델(`gemini-3.1-flash-lite`)과 다르므로 config에 **`tts.model` 필드를 추가**한다.
  - 출력은 **base64 PCM(16-bit·24kHz·mono)**. Telegram `sendVoice`(OGG/Opus)·`sendAudio`(MP3/M4A)가 PCM/WAV를 받지 않으므로 **OGG/Opus로 transcode**한다(ffmpeg subprocess). 이 변환 산출물을 저장·전송한다.
  - 생성 시점은 **파이프라인/스케줄러 사전 생성**(push 모델 §4.2)으로 한다. 세션 전송 시 실시간 합성하지 않는다. → 전송 지연 0.
  - 채점은 신규 grader 없이 **기존 MCQ 경로**(options + correct_answer)를 재사용한다. 문항 형태는 청해 comprehension MCQ를 1차로 하고 dictation/받아쓰기는 후순위로 미룬다.
- **장점**:
  - 신규 외부 계정·billing 없이 기존 무료 quota 안에서 동작한다.
  - Provider가 `TTSClient` interface 뒤에 있어(ADR-032의 `AudioStore`와 동일 철학) 추후 config-swap으로 교체 가능하다.
  - 사전 생성 + 캐싱(ADR-032) 덕에 preview 모델의 빡센 rate-limit이 실질 병목이 되지 않는다(하루 몇 개씩 잔고 채움).
- **단점 / 트레이드오프**:
  - chat과 별개의 native 호출 경로가 하나 더 생긴다(호환 계층 밖).
  - PCM→OGG transcode를 위해 **ffmpeg 외부 바이너리 의존**이 추가된다(§3 "deps last resort" 위반이나, 순수 Go Opus 인코더 성숙도가 낮아 실용적 선택).
  - preview 모델이라 정식 GA 전까지 API 스펙·rate-limit이 바뀔 수 있다. [UNKNOWN: preview TTS의 정확한 무료 RPD/RPM 미확정 — 사전 생성 구조상 결정에 비치명적]
- **대안**:
  - **GCP Cloud TTS**(config 기본값 Neural2, OGG_OPUS 직출력·변환 0·GA): Telegram-native 출력이 장점이나 GCP 프로젝트+billing(카드)+service account JSON 수동 셋업 마찰이 크고, 우리 egress 프로필상 이점이 상쇄돼(ADR-032) 기각.
  - **edge-tts**(무료·무키·멀티랭): 비공식(reverse-engineered) MS endpoint라 ToS/안정성 리스크 + 포트폴리오 감점으로 기각.
  - **VOICEVOX**(self-host·무료·JA 고품질): self-hosted 신호는 강하나 JA 전용 + 8GB RAM에서 엔진 컨테이너(~1GB) 부담으로 기각.

---

## ADR-032: 청해 음성은 S3 호환 object storage에 content-addressed로 저장하고 Telegram file_id로 재전송한다

- **날짜**: 2026-07-01
- **상태**: 채택됨
- **맥락**:
  - 생성된 음성 blob을 매 전송마다 재합성하는 것은 낭비다. 한 번 만든 음성은 durable하게 저장하고 재사용해야 한다.
  - 로컬 파일시스템(`tts.audio_dir = ./data/audio`)은 앱 인스턴스가 여러 개일 때 공유 불가·durability·CDN 부재라 §4(수만 유저 가정)에서 확장이 안 된다.
  - "단순 파일 저장이니 굳이 AWS가 아니어도 되지 않냐"는 검토 요청이 있었고, 한국에서 Cloudflare 관련 차단 이슈(2026-05 방통위 협조 CDN 차단, 2023 R2 도메인 DNS 차단 전례)가 제기됐다.
- **결정**:
  - **`AudioStore` interface**(`Exists`/`Put`/`Get`)를 두고, 구현체는 **`aws-sdk-go-v2` 단일 S3 client**로 한다. "S3"는 vendor(AWS)가 아니라 **API 표준**이며, endpoint만 바꿔 어떤 S3 호환 백엔드로도 swap한다(LLM의 BaseURL swap과 동형).
  - **캐시 key는 content-addressed**: `tts/{lang}/{voice}/{sha256(text)}.ogg`. 동일 스크립트는 서로 다른 문항이어도 1개 오브젝트로 dedup된다.
  - **DB markup은 기존 `Question.audio_path` 컬럼 재사용**(object key 저장, presigned URL 아님 — 만료됨). 스키마 변경 0.
  - **전송 캐시는 Telegram `file_id`**: 최초 1회만 바이트를 업로드하고 반환된 `file_id`를 DB에 캐시한다. 이후 모든 전송(같은 유저 재복습 + 다른 유저)은 `file_id` 문자열만 넘겨 Telegram이 자기 사본을 서빙한다.
    - **object store가 SSOT, `file_id`는 캐시**다. Telegram이 오래 미사용 파일을 드물게 purge해 `file_id`가 무효화되면 store에서 재fetch → 재업로드한다.
  - **로컬 백엔드 = MinIO**(docker-compose 컨테이너). S3 코드 경로를 로컬에서 실제로 실행·검증한다.
  - **prod 타깃 = AWS S3 서울(ap-northeast-2)로 lean**하되, `AudioStore` 추상화로 배포 시 config 한 줄로 확정한다. **Cloudflare R2는 명시적으로 기각**한다(아래 근거).
  - config에 storage 섹션(endpoint / bucket / region / access key)을 추가한다.
- **차단 리스크 검증 (2026-07-01, 개발 머신=한국 네트워크에서 직접 probe)**:
  - `r2.cloudflarestorage.com` → 정상 Cloudflare IP(`172.64.66.1/2`) 해석, HTTPS 301 + TLS 핸드셰이크 96ms 정상 수립. **현시점·이 네트워크에서 R2는 도달 가능**(2023 wholesale 차단은 현재 해제된 것으로 보임).
  - 즉 R2가 지금 막혀 있어서 기각한 것이 **아니다**. (a) R2의 유일한 강점인 free-egress가 우리 설계상 무의미하고(아래 비용 분석), (b) 방통위 협조 체제라 latent 재차단 여지가 있으며, (c) S3 서울은 도메스틱 리전이라 그 리스크 노출 자체가 0이기 때문이다. 이점 0 + latent 리스크 → 채택할 이유가 없다.
- **비용/규모 분석 (결정 근거)**:
  - 가정: OGG/Opus 음성 ~24kbps(~3KB/s), 평균 클립 ~25s ≈ **100KB/클립**, distinct 클립 ~2,000개 → 총 저장 **~200MB**.
  - **단일 유저**: storage ~$0.005/mo, egress는 100GB 무료 tier 내 → 사실상 **$0**(1년차 AWS Free Tier로 문자 그대로 $0).
  - **스케일(5만 유저)**: storage **동일 ~200MB**(content-addressing → 유저 수가 아니라 distinct 콘텐츠에 비례). egress도 `file_id` 재사용으로 파일당 1회 업로드 → 유저 수 무관 → **~$0**.
  - 반례: `file_id` 재사용을 안 하고 매 전송마다 store에서 재fetch+재업로드하면 5만 유저 × 30청취/일 × 100KB ≈ **4.5TB/mo → ~$570/mo**. **바로 이 시나리오에서 R2 free-egress가 의미**를 갖지만, 우리는 `file_id` 재사용으로 시나리오 자체를 회피한다.
  - 결론: **비용을 좌우하는 건 provider가 아니라 설계(content-addressing + file_id 재사용)**다. 좋은 설계가 provider egress 단가를 무력화하며, 이것이 이 ADR의 핵심 신호다.
- **장점**:
  - storage·egress 모두 유저 수가 아니라 distinct 콘텐츠에 비례 → 스케일에서 비용이 상수에 가깝다.
  - S3 표준 추상화로 vendor lock-in 회피. R2 차단 같은 "provider 하나가 죽는" 시나리오가 endpoint 교체로 흡수된다(추상화의 존재 이유를 실제 리스크가 정당화).
  - 로컬 MinIO로 prod S3 코드 경로를 미리 검증 → "로컬은 됐는데 prod S3에서 안 됨" 사고 방지.
  - 기존 `audio_path` 컬럼 재사용으로 마이그레이션 부담 최소.
- **단점 / 트레이드오프**:
  - 로컬에 MinIO 컨테이너가 추가된다(~200MB, 8GB에서 감내 가능).
  - `file_id` purge 엣지 케이스(무효화 시 재업로드) 처리 로직이 필요하다.
  - prod 스토리지 provider 최종 확정을 배포 시점으로 미룬다(추상화 덕에 저비용이나, 미결로 남는 항목).
- **대안**:
  - **로컬 FS 구현**: 지금은 단순하나 S3 코드 경로가 prod 전까지 미검증 방치 + 수평 확장 불가로 기각.
  - **Cloudflare R2**: free-egress·free tier 우수하나 우리 egress 프로필상 이점 0 + 한국 도메인 차단 전례/latent 리스크로 기각(위 검증).
  - **Azure Blob Storage**: native S3 API 부재 → S3Proxy shim이나 별도 SDK 필요, "단일 구현 + endpoint swap" 우아함이 깨져 기각.
  - **Google Cloud Storage**: S3 XML 호환 모드는 있으나 egress 단가가 비싸 이 용도에 이점 없어 기각.

### Amendment (2026-07-01, 구현 시): "스키마 변경 0"의 범위 정정

- 위 결정문의 **"스키마 변경 0"은 `audio_path`(object key) 재사용에 한정**된 서술이었다. 구현 단계에서 두 가지가 저장 위치를 필요로 함이 드러나 컬럼 2개를 추가했다(사용자 승인 완료):
  - **`questions.audio_script TEXT`**: content-addressed key의 `sha256(text)`에서 *text* = 음성으로 합성할 청해 스크립트. 기존 `prompt`는 화면에 보이는 comprehension 질문이라 스크립트를 담을 수 없어 별도 컬럼이 필요하다. (ADR-031의 "text"의 실제 저장소)
  - **`questions.audio_file_id TEXT`**: 위 "전송 캐시는 Telegram `file_id`" 결정을 구현하는 캐시 컬럼. object store가 SSOT, 이 컬럼은 캐시다.
- **파일 정책**: 신규 `002_*.sql`를 만들지 않고 **기존 `001_init.sql`을 멱등(`ADD COLUMN IF NOT EXISTS`)하게 수정**했다(사용자 결정). fresh install은 `CREATE TABLE`에, 기존 DB는 idempotent `ALTER`로 반영된다.
- **대안(기각)**: (a) `audio_assets(object_key PK, file_id)` 정규화 테이블 — dedup된 file_id를 오브젝트 단위 1행으로 두나 렌더 시 join 추가라 MVP엔 과함, (b) file_id를 Redis 캐시로 — DB 변경은 최소지만 "DB에 캐시" 결정과 어긋나고 Redis flush 시 유실. `audio_path`와의 대칭(둘 다 question 컬럼)을 위해 per-question 컬럼 채택.

---

## 관련

- 선행/연관 ADR: ADR-026(Question Type vs Item Type taxonomy — 청해 Skill 5종), ADR-022(Material SSOT).
- 후속 구현(Case B) 범위: `internal/external/tts_client.go`(Gemini native), `AudioStore`(S3/MinIO), 파이프라인 사전 생성·dedup, bot 청해 렌더(sendVoice + `file_id` 캐시), SessionBuilder 청해 편입, config(`tts.model` + storage 섹션), docker-compose(MinIO).
- 미결(별도 트랙): 손글씨 Mini App의 `cloudflared`/`trycloudflare` 의존도 동일 Cloudflare-Korea 노출점 — Case C TODO 후보로 분리 검토.
