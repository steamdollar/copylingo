# N5 일본어 문법 seed 도입 → 60개 확장 및 학습 세션 연동

## 1. 개요

N5 일본어 문법 학습을 두 단계로 구현했다.

1. **seed 도입**: `cmd/ja` seed 패턴 위에 grammar material/question을 신규로 추가 (smoke 세트 12개).
2. **확장 + 세션 연동**: 12개를 실질 학습이 가능한 60개로 확장하고, 기존 Vocabulary 카드만 노출되던 Study Session에서 Grammar 카드도 함께 노출·렌더링되도록 보강.

## 2. 변경 파일

- [cmd/ja/grammar.go](file:///home/lsj/work/copylingo/cmd/ja/grammar.go)
  - `GrammarPoint` SSOT 및 `N5GrammarPoints` 도입 후 12개 → 60개로 확장 (한국어 뜻/설명, 예문, ClozePrompt, 4개 일본어 옵션 포함).
- [cmd/ja/materials.go](file:///home/lsj/work/copylingo/cmd/ja/materials.go)
  - `GrammarMaterialPayload`, `BuildGrammarMaterials`, `MaterialKeyForGrammar` 추가, `BuildAllMaterials`에 grammar material 포함.
- [cmd/ja/seeder/main.go](file:///home/lsj/work/copylingo/cmd/ja/seeder/main.go)
  - grammar material ID 로드 후 grammar question 2종(`meaning`, `form`) 생성. `question_key`는 `ja:grammar:<id>:<variant>`로 안정화.
- [internal/repository/material_repo.go](file:///home/lsj/work/copylingo/internal/repository/material_repo.go)
  - `GetForStudySession`에서 `m.category = ANY($4)` + `pq.Array`로 `vocabulary`·`grammar` 모두 조회.
- [internal/bot/study_flow.go](file:///home/lsj/work/copylingo/internal/bot/study_flow.go)
  - grammar 전용 페이로드 파서 `grammarStudyPayload` 및 `renderGrammarPayload` 추가, `renderStudyMaterial`에서 문법 카테고리 분기 처리.
- 테스트: [cmd/ja/materials_test.go](file:///home/lsj/work/copylingo/cmd/ja/materials_test.go)(payload/count/key/integrity, 60개 반영), [cmd/ja/seeder/main_test.go](file:///home/lsj/work/copylingo/cmd/ja/seeder/main_test.go)(question builder·stable key·count 24→120문항·key `060`), [internal/bot/study_flow_test.go](file:///home/lsj/work/copylingo/internal/bot/study_flow_test.go)(`TestStudyFlowGrammarRendering`).
- [STATUS.md](file:///home/lsj/work/copylingo/STATUS.md): 최근 완료 항목 기록.

## 3. 설계 및 결정 사항

- **시딩 순서 보존**: `BuildAllMaterials` → `Material.UpsertBatch` → ID 로드 → 질문 빌드 → `Question.UpsertSeedBatch` 순서를 그대로 유지. 데이터 무결성(4지 선다·CorrectAnswer 포함 등)은 단위 테스트로 보장.
- **question type**: `QuestionWordOrder`는 model에 존재하나 bot render/grader 전용 흐름이 확인되지 않아 제외하고, 안정적으로 처리되는 `multiple_choice`만 사용.
- **grammar point당 2문항**: `meaning`(핵심 의미 선택) + `form`(예문 빈칸 particle/form 선택).
- **다중 카테고리 지원**: YAGNI 원칙으로 복잡한 비율 제어(Vocab 80%/Grammar 20% 등) 대신 단일 SQL의 `ANY`로 두 카테고리를 섞어 조회.
- **Study 노출 순서**: 단순 `id ASC`면 신규 vocabulary 500개 뒤로 grammar가 밀려, `ROW_NUMBER() OVER (PARTITION BY m.category ...)`로 category별 후보를 interleave.
- **Cloze 품질**: 모든 `ClozePrompt`에 `__` blank marker가 있고 정답을 그대로 노출하지 않도록 integrity test 추가.
- ADR 갱신 없음: 기존 material/question seed 구조를 확장한 콘텐츠 추가이며 비자명한 아키텍처 결정은 없음.

## 4. 검증

- `go test -v ./cmd/ja/...` 통과 (N5GrammarPoints 60개·생성 질문 120개 무결성).
- `go test -v ./internal/bot/...` 통과 (학습 세션 내 문법 렌더링 정상).
- `go test ./cmd/ja/... ./internal/repository ./internal/bot`, `make test` 전체 통과.
- Go 서버 runtime 동작 변경이 아닌 seeder/catalog/렌더 코드 변경이라 `make restart-app`은 미실행.
