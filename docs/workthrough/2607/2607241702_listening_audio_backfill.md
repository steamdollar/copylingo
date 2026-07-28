# Listening audio path 복구 및 backfill CLI

- **날짜**: 2026-07-24
- **범위**: N5 Listening Question의 `audio_path` 누락 원인을 보정하고, 세션 생성이나 Telegram push 없이 남은 audio pointer를 모두 복구한다.

## 원인

- `QuestionRepository.UpsertSeedBatch`는 seeder가 제공하는 NULL `audio_path`를 기존 값 위에 그대로 덮어썼다.
- TTS object는 MinIO에 남아 있었지만, 이후 JA seeder를 재실행하면 Question의 pointer만 사라진다.
- Scheduler의 `TopUpAudio`는 세션 push 이후 한 cycle에 최대 5개만 처리하므로, pointer가 대량으로 초기화되면 빠르게 복구되지 않는다.

## 변경

- `internal/repository/question_repo.go`
  - 같은 `audio_script`를 seed할 때 기존 `audio_path`를 보존한다.
  - script가 달라지면 `audio_path`를 NULL로 갱신해 새 TTS 생성 대상으로 만든다.
- `internal/repository/question_repo_test.go`
  - script-aware `audio_path` upsert contract를 검증한다.
- `cmd/admin/generate_listening_audio/main.go`
  - `go run ./cmd/admin/generate_listening_audio -language ja -level N5`로 pending audio를 모두 채우고, 남은 pending 수가 0인지 확인하는 one-shot CLI를 추가했다.
  - scheduler를 호출하지 않으므로 learning session 생성 및 Telegram push가 없다.

## 실행 및 검증

- `go test ./internal/repository ./cmd/admin/generate_listening_audio` 통과.
- `make test` 통과.
- `go run ./cmd/admin/generate_listening_audio -language ja -level N5 -timeout 15m` 실행.
  - pending 40개를 8 cycle로 처리했고, MinIO existing object 40개를 재사용했다.
- DB 최종 상태: listening 50개, scripted 50개, `audio_path` 50개, pending 0개, distinct audio path 50개.

## 결정

- 기존 script와 연결된 content-addressed object는 유효하므로 seeder가 runtime pointer를 지우지 않도록 한다.
- script 변경 때는 content hash가 달라지므로 pointer를 유지하지 않고 다음 TTS backfill 대상에 넣는다.
