# N4 seed 데이터를 공통 record 형식으로 통일

- 날짜: 2026-10-08
- 계기: 문항 유형마다 JSON key가 달라서 통합하자는 요청. 최종 목표는 JSON을 한 형식으로 맞추고, 문항 유형과 무관하게 하나의 로직으로 DB seeding하는 것이다. N5의 자동 생성 문항은 나중에 정하기로 하고 N4부터 진행했다.
- 결정: [ADR-066](../../adr/ADR_from_61_to_80.md#adr-066-seed-데이터를-db-row-모양의-공통-record로-통일한다-n4-먼저)
- 동작 변화: 없음. seed 결과(material·question)가 변경 전과 key별로 같다.

## 구조

```
cmd/seeding/data/ja/
  kana.json, n5_*.json          ← N5 legacy (기존 builder 유지)
  n4/records.json               ← {materials: [{material_key, category, title, difficulty, payload,
                                               questions: [{question_key, item_type, type, category, prompt,
                                                            options, correct_answer, explanation, difficulty}]}],
                                    questions: [ 청해처럼 material 없는 문항 (audio_script 포함) ]}
```

seeder 흐름: material upsert(legacy + record) → 문항이 달린 record material의 id를 한 번에 조회 → level마다 legacy builder와 `buildRecordQuestions` 실행 → question upsert.

같은 날 2단계로 진행했다. 1단계(`ced9148`)는 `materials/`·`questions/` 디렉토리에 나눠 두고 `material_key`로 연결했다. 2단계에서 문항을 소속 material 안에 넣었다. DB의 `questions.material_id` 1:N 구조와 같게 맞춘 것이다. 파일은 level당 하나로 합쳤다(38,744줄, 1.8MB). 로더가 level 디렉토리의 `*.json`을 모두 합치기 때문에, 나중에 개수 기준으로 나눠도 코드는 바뀌지 않는다.

## 변경 내역

| 대상 | 내용 |
|---|---|
| `cmd/seeding/data/ja/n4_*.json` (5개 삭제) → `data/ja/n4/records.json` | 변환 시점의 builder 출력을 그대로 저장했다. material 1,125개, 그 안의 문항 1,325개, 최상위 청해 80개. 문항마다 기존 `question_key`를 그대로 적었다 |
| `cmd/seeding/data/embed.go` | embed 패턴을 `*/*.json`에서 `*/*`로 바꿨다. level 디렉토리가 재귀로 포함된다 |
| `cmd/seeding/catalog/datasets.go` | `MaterialRecord`(문항 중첩)·`QuestionRecord` 타입, 파일 모양 `levelRecords{materials, questions}`, `LevelCatalog.Materials/Questions`, registry 항목의 `recordDir`, level 디렉토리의 `*.json`을 이름순으로 합치는 `loadRecordDir`를 추가했다. `QuestionSeed`를 제거했다 |
| `cmd/seeding/catalog/materials.go` | `BuildRecordMaterials` 추가. `BuildAllMaterialsForLevels`가 record material을 뒤에 붙인다 |
| `cmd/seeding/main.go` | `loadRecordMaterialIDs`(문항이 달린 material만 조회)·`buildRecordQuestions`(필수 필드, 중첩·최상위를 합친 key 중복, material 아래에 청해 문항 금지, 미해결 material을 error로 반환)를 추가했다. `buildQuestionSeeds`·seed key 규칙·`categoryForItemType`·`materialIDsForCatalogKeys`를 제거했다. 내용이 같던 store interface 4개를 `materialKeyStore`로 합쳤다 |
| `cmd/seeding/catalog/datasets_test.go` | N4 전용 fixture 테스트를 `TestRecordCatalogIntegrity`로 바꿨다. 모든 record level에 적용되며, key 유일성(level 간 포함), 언어 접두어, payload 객체 여부, 정답이 선지에 있는지, 어순 조각을 이으면 정답이 되는지, 청해 규칙(material 밖에만 두고 audio_script 필수, 최상위 문항은 청해만)을 검사한다. `TestN4RecordsCoverOfficialItemTypes`는 N4 문항 유형 15종을 고정한다 |
| `cmd/seeding/catalog/materials_test.go` | N4 material 기대 개수를 record 수로 바꾸고, language 기록을 검사한다 |
| `cmd/seeding/main_test.go` | N4 builder 통합 테스트와 결정성 테스트를 `buildRecordQuestions` 단위 테스트(중첩·최상위 변환, 거부 5가지)와 `loadRecordMaterialIDs` 조회 대상 테스트로 바꿨다 |
| `docs/adr/ADR_from_61_to_80.md` | ADR-066을 추가하고, ADR-065 B2에 대체 표시를 넣었다 |

수정한 Go 파일에 `goparams`를 적용했다.

## 검증

- seed 출력 비교(1단계·2단계 각각): 변경 전(`61edb6a`)과 후의 전체 seed 출력을 같은 조립 순서로 dump했다. material ID는 가짜 store로 넣었다. material은 `material_key`, question은 `question_key` 기준으로 비교했고, `material_id`는 material_key로 바꿔 비교했다. 결과: material 2,242개, question 5,318개, key 차이 0, 필드 차이 0. dump용 임시 테스트는 삭제했다.
- `make test`: 통과.
- 서버는 `cmd/seeding`을 import하지 않으므로 재시작은 생략했다. 기존 DB를 다시 seed할 필요는 없다(결과가 같음).

## 남은 일

- N5 전환: 자동 생성 문항 3,668개(kana, 어휘 뜻·회상·손글씨·한자, 문법 뜻)를 데이터로 저장할지 생성 도구로 남길지 정한 뒤, 작성된 N5 문항과 함께 record로 옮긴다. 다 옮기면 legacy builder와 타입 필드를 지운다.
