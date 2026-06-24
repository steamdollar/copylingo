# N5 학습 데이터 Go 리터럴 → embedded JSON 분리 (grammar PoC → vocab/kana 확장)

## 배경

- `cmd/ja/grammar.go`에 N5 grammar 60개가 Go struct 리터럴(약 660행)로 내재화되어 있었다.
- 사용자가 "데이터를 jsonl 같은 파일로 빼면 어떨지" 검토를 요청했고, content/code 분리·확장성(N4·N3)·비엔지니어 편집 관점에서 데이터 층만 파일로 분리하기로 했다.
- 전체(grammar/vocab/kana) 일괄 전환 대신 **grammar 단일 범위 PoC**로 먼저 검증 후 확장하는 방향을 택했다.

## 결정 사항

- **포맷: JSON (pretty array)**. JSONL/YAML 대신 선택한 이유:
  - `encoding/json`은 stdlib이고 seeder가 이미 사용 중 → 신규 의존성 0 (yaml은 viper 통한 간접 의존만 존재).
  - 60건 손편집 큐레이션 셋이라 줄단위 JSONL보다 pretty array가 diff·리뷰에 유리.
  - 주석(JLPT 출처 등)이 필요해지면 그때 YAML 승격을 재검토.
- **데이터/로직 경계**: raw 콘텐츠(`GrammarPoint` 배열)만 파일로 분리. question 생성 로직(option 셔플, prompt 템플릿, deterministic seed)은 Go에 유지.
- **단일 바이너리 유지**: `go:embed`로 JSON을 바이너리에 포함.
- ADR 갱신 없음: 기존 seed 구조의 데이터 출처 이동이며, 생성 로직·DB 스키마·런타임 동작은 불변.

## 변경 파일

- `cmd/ja/data/n5_grammar.json` (신규)
  - grammar 60개 콘텐츠 SSOT. snake_case 키.
  - 기존 리터럴로부터 일회성 generator로 직렬화 생성(전사 오류 0). generator는 실행 후 삭제.
- `cmd/ja/grammar.go`
  - `GrammarPoint`에 `json` 태그 추가.
  - 660행 리터럴 제거, `var N5GrammarPoints = mustLoadN5Grammar()`로 교체.
- `cmd/ja/grammar_loader.go` (신규)
  - `//go:embed data/n5_grammar.json` + `LoadN5Grammar()` / `mustLoadN5Grammar()`.
- `cmd/ja/grammar_loader_test.go` (신규)
  - 데이터 무결성 회귀: 60건/필수 필드 non-empty/ID 중복/`CorrectAnswer ∈ FormOptions`.
- `STATUS.md`
  - 최근 완료 항목 추가.

## 검증 (2-게이트)

- **Gate ① 데이터 동등성 (DB 불필요)**: 리터럴을 남겨둔 채 `reflect.DeepEqual(LoadN5Grammar(), N5GrammarPoints)` → PASS. 이후 리터럴 제거하고 무결성 테스트로 전환.
- **Gate ② DB end-to-end 회귀**: seeder가 deterministic(`rand.NewSource(1)`) + idempotent upsert(`material_key`/`question_key` 충돌)임을 이용.
  - 리터럴 기반 seed 결과를 스냅샷 → JSON 기반 seeder 재실행 → grammar materials 60 / questions 120 **diff 0건**.
- `go build ./...`, `go test ./cmd/ja/...` 통과.

## 확장 (vocab + kana)

grammar PoC가 2-게이트를 통과해, 동일 패턴을 vocab·kana로 확장했다.

- `cmd/ja/data/n5_vocab.json` (신규) — vocab 500건. `VocabWord`에 `json` 태그 추가, 리터럴 약 500행 제거.
- `cmd/ja/data/kana.json` (신규) — kana→romaji 208건(JSON object). `KanaMap` 리터럴 제거.
- `cmd/ja/vocab.go` / `cmd/ja/kana.go` — 각각 `var N5Words = mustLoadN5Vocab()`, `var KanaMap = mustLoadKana()`로 교체.
- `cmd/ja/vocab_loader.go` / `cmd/ja/kana_loader.go` (신규) — `go:embed` + 얇은 로더.
- `cmd/ja/data_extraction_gate_test.go` (신규) — vocab(500/필수필드/ID중복)·kana(208/non-empty) 무결성 회귀.

경계 주의로 남겨뒀던 kana는 파일 자체가 `map[string]string` 단일 데이터였고 `ScriptLabel`·disambiguation hint 로직은 별도 파일/seeder에 있어, 데이터/로직 분리가 grammar와 동일하게 깨끗했다.

**확장 검증**: Gate ①(`DeepEqual(loaded, 리터럴)`) vocab/kana 모두 PASS 후 리터럴 제거. Gate ②는 전체 DB(materials 768 / questions 2242) 스냅샷 vs JSON 기반 재실행 **diff 0건**. `go build ./...`·`go test ./cmd/ja/...` 통과.

## 파일 정리 (loader 중복 제거)

확장 후 데이터셋마다 `*_loader.go`가 거의 동일하게 3개로 늘어 `cmd/ja`가 비대해졌다. 외부에서 `Load*`를 쓰지 않고(var들이 `mustLoad*`만 사용) 테스트만 호출하던 점을 확인하고 DRY로 정리했다.

- `cmd/ja/dataset.go` (신규) — 제네릭 `loadJSON[T]`/`mustLoadJSON[T]` 한 벌 (go 1.25).
- `grammar.go`/`vocab.go`/`kana.go` — 각자 `//go:embed` + `var X = mustLoadJSON[T](...)`를 갖는 self-contained 형태로 통합.
- 삭제: `grammar_loader.go`·`vocab_loader.go`·`kana_loader.go`·`grammar_loader_test.go`·`data_extraction_gate_test.go`.
- `cmd/ja/dataset_test.go` (신규) — 세 데이터셋 무결성 테스트 통합. 외부 미사용 `Load*` 래퍼 제거에 맞춰 package var(`N5GrammarPoints`/`N5Words`/`KanaMap`)를 직접 검사 = seeder가 소비하는 그 값.

결과: `cmd/ja`의 go 파일 10 → 7, loader boilerplate 약 90행 제거.

이어서 카탈로그 라이브러리를 `cmd/ja/catalog/` 패키지(`package catalog`)로 모았다. `cmd/ja`를 import하던 건 자기 seeder 하나뿐이라(`cmd/`엔 main만 두는 관례와도 어긋났음) 영향이 작았다.

- 이동·통합: 데이터 정의 4개(`grammar/vocab/kana/dataset.go`)를 `cmd/ja/catalog/datasets.go` 하나로, `materials.go`·`materials_test.go`·`data/`를 `catalog/`로 이동.
- 테스트: `dataset_test.go` → `catalog/datasets_test.go`.
- seeder: import만 `cmd/ja` → `cmd/ja/catalog`로 교체(alias `ja` 유지 → 본문 `ja.X` 참조 불변).
- 결과 구조: `cmd/ja/{catalog/, seeder/}`. catalog는 `datasets.go`(struct+embed+generic loader)·`materials.go`·각 `_test.go`·`data/*.json`로 cohesive.

`go build ./...`·`go vet`·`gofmt -l`(무출력)·`go test ./...` 모두 통과.

## 남은 후보

- N4·N3 등 상위 레벨 콘텐츠는 이제 JSON 파일 추가만으로 확장 가능.
- 손편집 시 주석(JLPT 출처 등)이 필요해지면 YAML 승격 재검토 (현재는 stdlib `encoding/json` 유지).
