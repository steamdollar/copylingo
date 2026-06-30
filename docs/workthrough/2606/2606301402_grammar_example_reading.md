# Grammar 예문 읽기(furigana) 보강 — `example_reading` 필드

- **날짜**: 2026-06-30
- **Case**: A(결정 → ADR-030) → B(구현/검증/종료)
- **요청**: "grammar study 때 한자가 많이 나오는데 읽는 법을 모르겠다 — 칼럼 추가? 해결 방법?"

## 배경 / 문제

- Grammar Study Material의 예문(`example`: 私は学生です。)은 한자 포함인데 읽기가 없어 N5 학습자가 못 읽음.
- Vocabulary는 이미 `kana` 필드가 `읽기:`로 렌더돼 동일 문제 없음 (`renderVocabularyPayload`).
- "칼럼 추가" 직관은 맞으나 실제론 **DB 칼럼이 아니라 catalog JSON 필드** — grammar는 JSON → `Material.Payload`(JSONB)로 시드되므로 마이그레이션 불필요.

## 결정 (ADR-030)

- 사용자 선택: **A안(전문 かな 단일 줄) + grammar 예문만**.
- 표현: 전문 ひらがな(가타카나·문장부호 원형 유지) `example_reading` 필드, vocab `읽기:`와 동일한 별도 줄.
- 기각: B안 인라인 괄호 furigana, C안 런타임 형태소 분석(kagome 수십 MB 의존성, 정적 80개엔 과함).
- 정적 catalog은 시드 타임 미리 계산해 payload에 박음(수만 유저 가정 §4: 렌더마다 분석 대신 1회 계산 후 싸게 서빙).

## 변경 파일

| 파일 | 변경 |
|---|---|
| `cmd/ja/catalog/data/n5_grammar.json` | 80개 entry에 `example_reading`(전문 かな) 추가 — `example` 바로 뒤 |
| `cmd/ja/catalog/datasets.go` | `GrammarPoint.ExampleReading` 필드 + 의도 주석 |
| `cmd/ja/catalog/materials.go` | `GrammarMaterialPayload.ExampleReading` + `BuildGrammarMaterials`에서 thread |
| `internal/bot/study_flow.go` | `grammarStudyPayload.ExampleReading` + `renderGrammarPayload`에서 예문 아래 `읽기:` 줄(비-bold) |
| `cmd/ja/catalog/datasets_test.go` | 무결성 검사 필수 필드에 `ExampleReading` 추가(80개 전수 강제) |
| `cmd/ja/catalog/materials_test.go` | grammar 009 payload 단언에 `ExampleReading` 추가 |
| `internal/bot/study_flow_test.go` | 렌더 테스트 payload에 `example_reading` 추가 + `읽기` 줄 출력 단언 |

## 렌더 결과 (예)

```
예문: 私は学生です。
읽기: わたしはがくせいです。
해석: 저는 학생입니다.
```

## 검증

- `make test` 전체 그린.
- 재시드: `go run ./cmd/ja/seeder` → materials 828 / questions 2447 upsert(멱등 `ON CONFLICT ... DO UPDATE SET payload=EXCLUDED.payload`).
- `make restart-app` → `:8080/health` healthy.
- DB 스팟체크: `materials.payload->>'example_reading'` 확인
  - `ja:grammar:001` → わたしはがくせいです。
  - `ja:grammar:009` → つくえのうえにほんがあります。
  - `ja:grammar:072` → クラスでたなかさんがいちばんせがたかいです。 (카타카나 보존 확인)

## 범위 밖 / 후속

- quiz `cloze_prompt`·vocab_context cloze 읽기 보강(동일 `example_reading` 패턴 확장).
- 동적 AI 콘텐츠(Phase 2.4 아티클)는 런타임 furigana 필요 → LLM 생성 파이프라인이 읽기를 함께 출력하는 별도 설계(ADR-030 대안 참조).
- `example_reading` 정확성은 무결성 테스트가 보장 못 함(존재만 강제) → 리뷰 의존.
