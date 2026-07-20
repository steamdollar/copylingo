# N5 Listening 50문항 확장 및 TTS Fill (Case B)

- **날짜**: 2026-07-20
- **설계 근거**: ADR-031/032(Gemini native TTS·S3/Telegram cache), ADR-034(Question-only quiz seed)
- **범위**: listening source seed를 10→50문항으로 확장하고 로컬 DB seed 및 MinIO TTS object를 50/50 준비.

## 위임 / 검수

- 사용자 요청에 따라 읽기·대량 생성 작업을 `gpt-5.6-luna` native subagent 3개에 병렬 위임했다.
  - `0011..0024`: home/school/daily routine/object/rule 14건
  - `0025..0037`: shop/restaurant/service/transport/work 13건
  - `0038..0050`: health/hobby/plan/invitation/cooking/family 13건
- Subagent는 파일을 수정하지 않고 JSON 초안만 반환했고, main agent가 단일 JSON SSOT에 병합했다.
- Main 검수에서 다음을 보정했다.
  - 0028 한·일 혼용 prompt와 0032 options/correct exact-match 위반은 해당 Luna에 재작성을 요청.
  - `かもしれません`, `～ている間`, `～ため`, `～すぎる`, `誘う` 등 N4 표현 6곳을 N5 문법으로 단순화.
  - 부자연스러운 `明日の学校で`, `犬と近所を歩く` 표현과 중복 prompt를 수정.

## 변경

- `cmd/ja/catalog/data/n5_listening.json`: original N5 comprehension MCQ 40건 추가, 총 50건.
- `cmd/ja/catalog/datasets_test.go`: 기대 수량 50, ID `0001..0050` 연속성, script/prompt 유일성 검증 추가.
- `cmd/ja/seeder/main_test.go`: listening Question mapping 기대 수량 50으로 갱신.
- 최종 분포: `listening_task=19`, `listening_key_point=18`, `listening_outline=13`; difficulty 1=41, 2=9.

## 검증

- `jq empty cmd/ja/catalog/data/n5_listening.json` 통과.
- `go test ./cmd/ja/catalog ./cmd/ja/seeder` 통과.
- `make test` 전체 통과.
- `git diff --check` 통과.
- App restart 생략: server code/config는 변경하지 않았고 runtime은 DB seed 결과를 조회한다.

## DB / TTS 실행 결과

- JA seeder: 총 2,928 questions upsert, listening=50.
- Seed 직후: listening 50, `audio_script` 50, pending `audio_path` 50, 기존 Telegram `audio_file_id` 5 보존.
- `TopUpAudio` 10 cycle 실행:
  - 기존 content-addressed MinIO object 재사용 5건
  - Gemini TTS→ffmpeg OGG→MinIO 신규 생성 45건
  - 실패 0, 최종 pending 0
- 최종 DB: listening 50, `audio_path` 50, distinct path 50, `audio_file_id` 5.
- MinIO에는 OGG 51개가 있으며 DB 참조 50개는 전부 존재한다. 나머지 1개는 과거 테스트 잔여 object로 참조되지 않아 삭제하지 않았다.
