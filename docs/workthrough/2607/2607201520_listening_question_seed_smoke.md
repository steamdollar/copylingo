# N5 Listening 5문항 Seed 및 Live Smoke (Case B)

- **날짜**: 2026-07-20
- **설계 근거**: ADR-031/032(Gemini TTS·S3/Telegram cache), ADR-034(초기 Question-only quiz seed)
- **범위**: original N5 listening comprehension MCQ 5개 작성, 멱등 seed 연결, local live e2e 검증. Listening Study Material은 범위 밖이다.

## 변경

- `cmd/ja/catalog/data/n5_listening.json`: 가격·시간·장소·행동·순서 주제의 original 4지선다 5문항 추가.
- `cmd/ja/catalog/datasets.go`: embedded listening dataset과 `ListeningQuestion` schema 추가.
- `cmd/ja/seeder/main.go`: `QuestionListening`/`CategoryListening`, `audio_script`, stable `question_key` 매핑 후 기존 `UpsertSeedBatch`에 포함.
- `cmd/ja/catalog/datasets_test.go`: 5건 수량, 필수 필드, ID/script 중복, 허용 skill, 난이도, option/정답 integrity 검증.
- `cmd/ja/seeder/main_test.go`: Question field mapping과 generated/cache/material field nil, key uniqueness 검증.
- `STATUS.md`: listening seed TODO 제거 및 완료 기록. 기존 `docs/todos/listening_question_seed.md`는 Case C 완료 규칙에 따라 삭제.

## 검증

- Targeted: `go test ./cmd/ja/catalog ./cmd/ja/seeder` 통과.
- Required: `make test` 전체 통과.
- Seed: JA seeder 결과 listening 5건, `audio_script` 5/5, 초기 `audio_path` 0/5.
- TTS/object store: `TopUpAudio(ja,N5)` 한 cycle에서 pending=5, generated=5, failed=0. DB `audio_path` 5/5 및 distinct path 5개 확인.
- Runtime: `make restart-app` 성공, `http://localhost:8080/health` 정상.
- Telegram: listening-only session `#166`(5문항) 발송. Redis working set에서 3문항 answer 및 correct=true 3건 확인, 현재 index=3. Telegram `audio_file_id` cache 3건 확인.

## 결과 / 남은 범위

- 실제 Gemini→ffmpeg OGG→MinIO→Telegram voice→MCQ exact grading→다음 문항 전환이 연속 3회 정상 동작했다.
- 남은 2문항의 Telegram 재생은 동일 pipeline이며, seed/TTS/object path는 5건 모두 검증됐다.
- Listening Study Material 및 Quiz `material_id` 연결은 ADR-034에 따라 후속 Case A 범위다.
