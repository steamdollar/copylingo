# N5 독해 Study·Quiz MVP (ADR-036)

- **날짜**: 2026-07-23
- **Case**: C 실행 (plan: `docs/todos/n5_reading_study_quiz.md` — 완료 후 삭제, git history에 보존)
- **ADR**: ADR-036 (Material-first 독해 도입 + 세션당 1개 cap + 자동 chaining 제외)

## 무엇을 했나

original N5 short passage 10개를 Study Material → Quiz MCQ → 사용자별 SRS 경로로 추가했다.

```text
n5_reading.json (원본 SSOT)
  └─ materials: ja:reading:n5_reading_XXXX (Study: 원문+읽기 보조+핵심 어휘)
       └─ questions: ja:reading:n5_reading_XXXX:question:1 (Quiz MCQ, material_id FK)
            └─ user_question_progress (사용자별 Quiz SRS)
```

- Study 카드는 원문·전문 かな 읽기 보조·핵심 어휘만 공개. 한국어 근거는 Quiz `explanation`에서만 공개.
- 독해 Quiz는 해당 지문 Material을 Study 완료한 뒤에만 신규 후보 (미학습 fallback 예외 없음).
- Study Session·Daily Quiz 모두 세션당 독해 최대 1개.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `cmd/ja/catalog/data/n5_reading.json` | **신규** — original 지문 10개 (`reading_short`, 지문당 MCQ 1개, difficulty 1~3) |
| `cmd/ja/catalog/datasets.go` | `n5_reading.json` embed, `ReadingPassage`/`ReadingVocabulary` 타입, `N5ReadingPassages` |
| `cmd/ja/catalog/materials.go` | `ReadingMaterialPayload`, `BuildReadingMaterials()`, `MaterialKeyForReading()`, `BuildAllMaterials()` append |
| `cmd/ja/seeder/main.go` | `loadReadingMaterialIDs()`, `buildReadingQuestions()` (prompt = 지문+질문, 읽기 보조 미포함), 결과 append |
| `internal/model/material.go` | `MaterialCategoryReading = "reading"` |
| `internal/repository/material_repo.go` | Study 후보 category에 reading 추가(`studySessionMaterialCategories` var 추출), ranked CTE 뒤 `category_rank <= 1` cap |
| `internal/bot/study_flow.go` | `renderReadingPayload()` (HTML escaping, 빈/불량 payload fallback), category label "Reading" |
| `internal/repository/question_repo.go` | `newQuestionsForStudiedMaterialsQuery`에 `(q.category <> 'reading' OR ump.material_id IS NOT NULL)` admission gate |
| `internal/service/session_builder.go` | relay에 `CategoryReading` 추가, `maxReadingPerSession = 1` admission gate + relay alloc clamp |
| `docs/adr/ADR_from_21_to_40.md` | ADR-036 기록 |

## 테스트

- `cmd/ja/catalog/datasets_test.go` — `TestN5Reading_Integrity`: 10개·ID 형식·필수 필드·중복 없음·`reading_short` 전용·4지선다 유일성·정답 포함·HTML 특수문자 금지(seeder가 escaping 없이 HTML prompt에 inline하므로 dataset 차원에서 차단).
- `cmd/ja/catalog/materials_test.go` — reading material metadata/payload 검증 + quiz 전용 필드(payload) 누출 금지 + `BuildAllMaterials` 총계 갱신.
- `cmd/ja/seeder/main_test.go` — `buildReadingQuestions` (key/category/material link/prompt 구성/읽기 보조 미누출), `loadReadingMaterialIDs`(+missing).
- `internal/repository/*_test.go` — Study query reading cap·category 목록, Quiz admission gate 문자열 검증 (기존 query-string 테스트 관례).
- `internal/service/session_builder_test.go` — `TestBuildSession_CapsReadingAtOne`: due review 2개 중 1개만 admission, relay는 cap 소진 시 reading fetch 자체를 skip, fallback이 돌려준 reading도 거부.
- `internal/bot/study_flow_test.go` — reading 카드 렌더(제목/원문/읽기/핵심 어휘), HTML escaping(`<b>`→`&lt;b&gt;`), kana-only 어휘 읽기 중복 미표기, 빈/불량 payload fallback.

## 검증 결과

- `make test` 전체 통과.
- `go run ./cmd/ja/seeder` 2회 실행 → `materials(category=reading)` 10, `questions(category=reading)` 10, distinct `question_key` 10 유지 (멱등 확인). reading question 전건 `material_id` 링크, difficulty 1~3.
- `make restart-app` → `http://localhost:8080/health` OK.

## 남은 수동 검수 (자동화 불가)

- 실제 Telegram에서 10개 지문의 원문·보기·해설 표시 수동 검수 (Telegram HTML/길이·N5 난이도 체감).
- Study 완료 → Quiz 후보 진입 → 정답/오답 → due 재출제 live 흐름 확인.

## 범위 밖 (ADR-036 기록)

NHK/외부 기사 ingest, runtime LLM 지문 생성, 전문 번역 토글, 전용 독해 메뉴, Study→Quiz 자동 chaining, corpus 50개 확장(`reading_mid`/`information_retrieval` 분포는 공식 sample 검증 후 결정).
