# Study용 N5 Vocabulary Material Catalog 500개 확장

## 배경

Study Session에서 사용할 Vocabulary Material이 100개뿐이라 초기 학습 분량이 부족했다.
공식 JLPT 고정 목록을 재현하지 않고, Kana 학습 직후 필요한 기초 단어를 repo-owned curated Catalog로 관리하기로 했다.

## 변경 사항

- `cmd/ja/material_seeder/vocab.go`
  - 기존 `n5_word_001` ~ `n5_word_100` 유지
  - `n5_word_101` ~ `n5_word_500` 추가
  - 가족, 시간, 장소, 교통, 생활용품, 음식, 신체, 날씨, 활동, 형용사, 부사, Counter 영역 포함
- `cmd/ja/material_seeder/material_test.go`
  - `TestN5WordsIntegrity` 추가
  - 수량, ID 연속성, 빈 필드, 품사 whitelist, 완전 중복, Material Key 중복 검증

## 작업 방식

- Gemini CLI `gemini-2.5-flash`에 Vocabulary 초안 작성을 50개 단위로 위임했다.
- 각 batch 이후 ID 수량, 문법, 품사 whitelist, `(Kana, Kanji, MeaningKo)` 완전 중복을 검증했다.
- 검토 중 발견한 중복, 표기 불일치, 오타를 교정했다.
- 외부 Scraping 또는 외부 Dataset Import는 사용하지 않았다.

## 검증

```bash
gofmt -w cmd/ja/material_seeder/vocab.go cmd/ja/material_seeder/material_test.go
go test ./cmd/ja/material_seeder -v
make test
git diff --check
go run ./cmd/ja/material_seeder
go run ./cmd/ja/material_seeder
```

- 전체 테스트 통과
- Vocabulary Material `500`건 Upsert
- Seeder 재실행 후에도 Vocabulary Material `500`건 유지: Idempotency 확인
- 로컬 DB의 기존 Kana Material `208`건은 별도 승인 전까지 유지
