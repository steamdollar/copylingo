# Luna 기반 N5 Listening Seed 5→10문항 확장 (Case B)

- **날짜**: 2026-07-20
- **범위**: 기존 original N5 listening 5문항에 추가 5문항을 작성해 source seed를 10개로 확장. DB seed와 TTS 생성은 이번 범위에서 실행하지 않았다.
- **위임**: 사용자 요청에 따라 `gpt-5.6-luna` subagent가 JSON 초안을 작성했고 main agent가 콘텐츠·schema·정답 유일성을 검수했다.

## 콘텐츠 검수

- Luna 1차안 중 기존 가격 문제와 직접 겹친 0006, 기존 날씨→행동 문제와 유사한 0008은 기각하고 Luna에 재생성을 요청했다.
- 최종 추가 주제는 음식 주문, 도서관 이후 일정, 물건 위치, 병원 요일, 가족 행동이다.
- 0009 prompt의 `언제`는 script에 요일과 시각이 함께 있어 모호하므로 main agent가 `무슨 요일`로 좁혔다.
- 전 문항은 original이며 4지선다, options 내 exact answer 1개, N5 수준 script, 기존 허용 listening skill을 유지한다.

## 변경

- `cmd/ja/catalog/data/n5_listening.json`: `n5_listening_0006`~`0010` 추가.
- `cmd/ja/catalog/datasets_test.go`: embedded dataset 기대 수량 5→10.
- `cmd/ja/seeder/main_test.go`: listening Question mapping 기대 수량 5→10.

## 검증

- `jq empty cmd/ja/catalog/data/n5_listening.json` 통과.
- `go test ./cmd/ja/catalog ./cmd/ja/seeder` 통과.
- `make test` 전체 통과.
- `git diff --check` 통과.

## 미실행

- 새 5문항은 source seed에만 준비했다. 로컬 DB upsert, Gemini TTS quota 사용, MinIO object 생성은 외부-state 작업이라 별도 승인 전 실행하지 않았다.
