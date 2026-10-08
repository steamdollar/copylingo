# Seed 도구를 `cmd/seeding` 언어별 구조로 이동 (B1)

- 날짜: 2026-10-08
- 계기: `cmd/ja`의 Material·dataset 구성을 언어 무관하게 일반화하고 싶다는 요청. JSON은 `cmd/seeding` 아래 언어별 폴더에 두는 구조를 원했다.
- 결정: [ADR-065](../../adr/ADR_from_61_to_80.md#adr-065-seed-도구를-언어별-데이터-구조의-cmdseeding으로-옮긴다). B1(위치·언어 구분 계층)만 진행하고, B2(일본어 builder 분리)는 미착수.
- 동작 변화: 없음. 실행 명령만 `go run ./cmd/ja/seeder` → `go run ./cmd/seeding`(`-language`, 기본 `ja`)으로 바뀌었다.

## 구조

```
cmd/seeding/
  main.go            ← -language 플래그, DB 연결, Material→Question upsert
  catalog/           ← 타입, registry(언어+level), Material builder, key helper
  data/
    embed.go         ← //go:embed */*.json
    ja/*.json        ← 기존 cmd/ja/catalog/data/*.json 그대로
```

## 변경 내역

| 대상 | 내용 |
|---|---|
| `cmd/ja/seeder/*` → `cmd/seeding/` | seeder main·test 이동. import alias `ja`를 패키지명 `catalog`로 바꿨다. 패키지명과 겹치지 않도록 loop 변수 `catalog`를 `entry`로 바꿨다. `-language` 플래그를 추가하고, 등록되지 않은 언어면 `log.Fatalf`로 종료한다. key 접두어 `"ja:"` 문자열을 `catalog.Japanese`에서 만든다 |
| `cmd/ja/catalog/*.go` → `cmd/seeding/catalog/` | `VocabLanguage`를 `Japanese`로 바꿨다. `LevelCatalog.Language` 필드를 추가했다. `LevelCatalogs()`는 `LevelCatalogsFor(language)`로, `LevelCatalogFor(level)`은 `LevelCatalogFor(language, level)`로, `DefaultProficiencyLevel()`은 `DefaultProficiencyLevel(language)`로 바꿨다. JSON은 `data.FS`에서 `<language>/<file>`로 읽는다 |
| `cmd/seeding/data/embed.go` (신규) | `go:embed`는 `..`를 쓸 수 없으므로, 데이터 디렉토리 위에 embed 전용 패키지를 둔다 |
| `cmd/admin/generate_listening_audio/main.go` | import 경로를 바꾸고, `-language`/`-level` 기본값을 `catalog.Japanese`에서 가져온다 |
| `cmd/seeding/catalog/datasets_test.go` | registry 테스트에 언어 차원 검증을 추가했다. 조회 시 언어 정규화(`" JA "`), 미등록 언어(`el`)가 빈 결과를 받는지를 확인한다 |
| `README.md`, `docs/HANDWRITING_MINIAPP_INGRESS.md` | seeder 실행 명령 갱신 |
| `docs/adr/ADR_from_61_to_80.md`, `ADR_from_21_to_40.md` | ADR-065를 추가하고, ADR-027의 `cmd/ja` 위치 결정에 이동 표시를 넣었다 |

수정한 Go 파일 전체에 `goparams`를 적용했다. 그래서 기존에 포맷되지 않았던 매개변수 목록도 같이 바뀌었다(`main.go`, `materials.go`에서 diff가 큼).

## 검증

- seed 출력 동등성: HEAD worktree와 변경 후 코드에서 같은 조립 순서로 material·question 전체를 JSON으로 dump했다. material ID는 가짜 store로 주입했다. 두 dump가 `cmp`로 일치했다(material 2,242개, question 5,318개). 즉 DB `material_key`·`question_key`·payload가 바뀌지 않으므로 재seed가 필요 없다. dump용 임시 테스트와 worktree는 삭제했다.
- 기존 테스트의 `"ja:..."` key 문자열 기대값은 회귀 고정용으로 그대로 두었다.
- `make test`: 통과.
- 서버(`cmd/server`)는 `cmd/seeding`을 import하지 않으므로 런타임 재시작은 생략했다.

## 남은 일

- B2: kana·vocab(kana/kanji) 문항 builder, `ScriptLabel`, 탁점 hint를 `cmd/seeding/ja`로 분리한다. 언어 무관 builder(listening/reading/word order/question seed)는 언어를 인자로 받도록 바꾼다.
- commit 시 rename 이력 보존: 이동 전 경로에서 `goparams` 포맷만 적용한 commit을 먼저 두면, 이동 commit의 유사도가 95% 이상으로 유지된다. 그러지 않으면 `materials.go` 48%, `main.go` 55%로 rename 인식이 끊기거나 경계선이다.
