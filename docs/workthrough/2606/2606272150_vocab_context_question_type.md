# 단어 문맥규정(vocab_context) 문항 유형 도입

- 날짜: 2026-06-27
- Case: C(분리된 TODO) 실행 → B(plan→implement→verify→close)
- 한 줄: 기존 540 단어 중 예문 보유 단어에 한해 "문장 빈칸 + 같은 POS 4지선다" 문맥규정 문항을 정적으로 생성. grammar cloze 구조를 미러.

## 결정 (이번 세션 확정)

todo 문서의 미결정 3개 + 발견된 1개를 사용자 승인으로 확정:

1. **데이터 위치**: 별도 파일 `n5_vocab_context.json` (word_id 키, 부분 커버리지). vocab.json 비대화 회피·점진 확대.
2. **form_options**: 명시 저장 (grammar 패리티·결정론, 기존 테스트 패턴 재사용).
3. **question type**: `QuestionMultipleChoice` 재사용 + `Skill=SkillVocabContext`. 렌더(`session_question.go` default case)·채점 경로가 그대로 동작, 추가 wiring 0.
4. **(신규 발견) "예문 2~3개 + 세션마다 랜덤 1개 출제"의 실현**: **정적 N문항** 채택. 예문 N개 → 단어당 N개 별도 문항(키 분리). serving/스키마 변경 0으로 catalog→seeder→테스트 범위 내 완결.
   - tradeoff: 각 문항은 자기 문장을 SRS 재출제 때 반복(결정 근거를 부분만 충족). **런타임 문장 회전**(단어당 1문항 + 출제 시점 랜덤 픽)은 serving·예문 저장 위치 변경이 필요해 후속 ADR로 분리.

### FK 동작 검토 (논의 후 현행 유지)

- `questions.material_id`는 `REFERENCES materials(id) ON DELETE SET NULL` (nullable FK, `migrations/001_init.sql:81`).
- seeding 순서가 FK 유효성 보장: materials upsert → `GetByMaterialKeys`로 id 역조회 → 질문에 박음 → 질문 upsert. vocab_context는 새 material을 만들지 않고 기존 `ja:vocab:word_XXX`(단어당 1개)를 **재사용**(다대일).
- "넣자마자 다시 읽는 두 패스 / 중첩 struct co-upsert(GORM식)" 대안 검토 → **현행 유지**. FK가 SERIAL id를 참조해 단일 INSERT로는 못 박고, sqlx는 association 인식이 없으며(ORM 아님), seeder의 핵심은 idempotent upsert라 stable-key 역조회가 더 적합. 중첩이 노리는 "orphan 방지" 실익은 아래 2겹 방어로 더 싸게 달성.
- **nullable FK 함정 방어(2겹)**: ① `TestN5VocabContextIntegrity`가 모든 `word_id ∈ N5Words` 단언, ② 생성기는 grammar의 관용 skip과 달리 lookup miss를 `log.Fatalf`로 hard-fail(조용한 NULL 차단).

## 변경 파일

- `cmd/ja/catalog/data/n5_vocab_context.json` (신규) — 15단어 × 3 cloze = 45. noun 10 + verb 3 + adj 2. 모든 단어·보기는 catalog 실재어, 각 cloze는 정답만 유일하게 성립.
- `cmd/ja/catalog/datasets.go` — `VocabContext` 구조체 + `//go:embed` + `N5VocabContext`.
- `cmd/ja/seeder/main.go` — `buildVocabContextQuestions`(grammar builder 미러, rng로 옵션 셔플해 위치 편향 제거, hard-fail) + `wordsByID` 헬퍼 + main() wiring + 로그 라인에 `vocab_context=%d` 추가.
- `cmd/ja/catalog/materials_test.go` — `TestN5VocabContextIntegrity`(word_id 존재·옵션 len4/중복/정답포함·cloze `__`/정답 substring 금지·cloze ≥2·카운트 15/45).
- `cmd/ja/seeder/main_test.go` — `TestBuildVocabContextQuestions`(문항 45·전부 MC+SkillVocabContext·키 unique·material_id·옵션 검증).

## 카운트 단언 영향

별도 builder라 기존 단언 전부 불변: vocab 1620 / grammar 160 / material count(= kana+540+80). 신규 단언만 추가(vocab_context 45).

## 검증

- `go test ./cmd/ja/...` → ok (catalog, seeder)
- `go test ./...` (= make test) → 전체 green, FAIL 없음.
- 데이터 제약은 작성 직후 스크립트로도 선검증(entries=15, total_cloze=45, substring/len4/정답포함/word_id 존재 ALL OK).
- 런타임 반영(DB 적재)은 별도 운영 단계: `go run ./cmd/ja/seeder` 실행 시 신규 45문항이 upsert됨. 서버 코드 변경은 없어 `make restart-app` 불필요.

## 후속 (분리)

- **런타임 문장 회전** (단어당 1문항 + 출제 시점 랜덤 cloze): 예문 저장 위치(question 신규 컬럼 vs material payload 조회) + `session_question.go` 변경. ADR 대상.
- **`Material.UpsertBatch`에 `RETURNING id, material_key` 추가**: 역조회 select 제거(공용 경로, kana/vocab/grammar 공통 혜택). 별도 cleanup TODO.
- 예문 커버리지 확대: 현 15단어(45문항) MVP → 점진 확대.
