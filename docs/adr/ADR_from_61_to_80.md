# CopyLingo 의사결정 기록 (ADR)

## ADR-061: 여러 저장 호출의 트랜잭션 범위는 서비스에서 정한다

- 날짜: 2026-09-27
- 상태: 승인됨, Study 세션 생성부터 적용
- 배경: Study 세션과 자료 연결을 원자적으로 저장하려고 `SessionRepository.CreateStudySession`에 두 INSERT와 트랜잭션을 묶었다. 이 구조에서는 서비스가 저장 호출 사이에 다른 작업을 배치할 수 없다. Quiz 생성도 세션과 문항을 별도 호출로 저장한다.
- 결정: 여러 저장 호출을 묶어야 하는 서비스는 공유 `*sqlx.DB`를 받아 repository 계층의 `WithinTx(ctx, db, work)`를 직접 호출해 트랜잭션 범위를 정한다. 저장소 메서드는 전달받은 같은 `*sqlx.Tx`로 SQL을 실행한다. 트랜잭션 함수 값의 추가 주입이나 별도 runner 구조체·서비스 인터페이스는 두지 않는다. 먼저 Study 세션 생성에 적용하며, 기존 Quiz/Study 완료 경계는 이번 변경에 포함하지 않는다.
- 결과: 서비스에서 DB 호출의 순서와 중간 작업을 표현할 수 있고 시작·롤백·커밋 처리는 한곳에 모인다. 콜백과 저장소 계약에는 `sqlx.Tx`가 드러난다. 외부 API 호출의 효과는 PostgreSQL 롤백 대상이 아니므로 그 호출의 일관성 정책은 별도로 정해야 한다.

## ADR-062: Quiz·Study 진행 상태는 서비스를 거쳐 Redis 저장소에 직접 접근한다

- 날짜: 2026-09-27
- 상태: 승인됨
- 배경: 서비스와 Redis 저장소 사이의 `workingSetStore`가 저장 호출을 재전달하면서 오류 종류와 상태 검사를 다시 처리해 읽기 경로와 의존관계를 늘렸다. Study의 `Get`과 `CreateFromDB`는 실행 경로에서 사용하지 않는다.
- 결정: 두 서비스는 타입별 세션 저장소의 `Load/Save/Delete`를 직접 호출한다. JSON 및 버전·세션 ID 검사는 Redis 저장소의 `Load`에서 함께 처리한다. 세션 상태 미발견·손상 오류는 공통 `model.ErrSessionStoreNotFound/Corrupt`를 사용하고, 저장소 대역에 필요한 타입별 계약은 유지한다. 사용하지 않는 Study 메서드는 제거한다.
- 결과: `workingSetStore`와 그 백엔드 계약·오류 번역 설정이 사라지고 Redis 키·TTL·완료 시 DB 반영 흐름은 유지된다. 저장소 계약은 서비스 단위 테스트에서 Redis 없이 세션 상태를 주입하는 경계로 남는다.

## ADR-064: Study 플랜을 신규 어휘 중심으로 재조정한다

- 날짜: 2026-10-08
- 상태: 승인됨. [ADR-047](ADR-047_study_morning_evening_mix.md)의 분량 표를 대체한다
- 배경:
  - 최근 30일 기록을 보면 Study 시간의 약 52%가 이미 학습한 Material을 다시 보는 데 쓰였다. 내역은 N4 어휘 재학습 309분, N4 독해 재독 158분, N5 재학습 158분이다.
  - 같은 기간 재학습한 Material 644건 중 303건은 Quiz에서 이미 복습 간격 7일 이상으로 맞히고 있었다.
  - Material SRS는 Study에서만 진행되고 Quiz 결과를 반영하지 않는다. 그래서 알고 있는 단어도 3일, 6일, 15일 간격으로 다시 Study에 나온다.
  - 신규 어휘는 하루 9개 수준이었고, 플랜상 상한은 12개였다.
  - 12월 N3 목표에 비해 남은 N5·N4 미학습 어휘는 1,143개다.
- 결정:

| 유형 | 아침 신규 | 아침 복습 | 저녁 신규 | 저녁 복습 |
|------|----------:|----------:|----------:|----------:|
| Vocabulary | 14 | 2 | 10 | 4 |
| Grammar | 1 | 3 | 1 | 3 |
| Reading | 1 | 0 | 0 | 0 |
| **합계** | **16** | **5** | **11** | **7** |

  - 아침은 21개, 저녁은 18개다. 하루 신규 목표는 단어 24개, 문법 2개, 독해 1개다. 독해 지문 재독(저녁 복습 2개)은 없앤다.
  - `/study n` 스케일링은 하드코딩된 15:4:1과 신규 8/15 대신 `morningStudySessionPlan`에서 유형별 비중과 신규 비율을 계산한다. 앞으로 아침 플랜만 고치면 `/study n`도 따라간다. Reading 상한 2개는 유지한다.
- 결과:
  - 복습 자리가 줄어 Material due가 쌓일 수 있다. 숨기거나 초기화하지 않으며, 신규가 부족할 때 기존 보충 규칙(ADR-049)대로 소진된다. 학습 후 기억 확인은 Quiz의 question SRS가 맡는다.
  - 근본 해결은 Quiz 정답이 Material SRS를 늦추도록 연동하는 것이고, 별도 판단으로 남긴다. N4 사용자의 N5 복습 트랙(뜻·발음·손글씨·한자) 축소도 별도로 판단한다.
  - 1주 후 Study 소요 시간, 신규 어휘 Quiz 정답률, Material due 적체량을 보고 다시 조정한다.

## ADR-065: Seed 도구를 언어별 데이터 구조의 `cmd/seeding`으로 옮긴다

- 날짜: 2026-10-08
- 상태: 승인됨. [ADR-027](ADR_from_21_to_40.md)의 "JA seed catalog는 `cmd/ja`로 통합" 위치 결정을 대체한다
- 배경:
  - Seed 도구가 `cmd/ja/{seeder,catalog}`에 있어서 경로 이름부터 일본어 전용이었다. level 단위 registry는 이미 일반화되어 있었지만(ADR-046), 언어는 `VocabLanguage = "ja"` 상수와 `"ja:vocab:"` 같은 key 문자열 하드코딩으로만 표현됐다.
  - 실제 일본어 의존은 JSON 위치가 아니라 Go 쪽에 있다. vocab schema `VocabWord{Kana, Kanji}`, kana 자료·손글씨 문항, `ScriptLabel`, kanji recall·조수사 문항이 그렇다. vocab payload의 `kana`/`kanji` 키는 `materials.payload`에 저장되고 bot Study 화면 렌더러가 읽는다.
  - 서버 쪽도 일본어 전제다. JLPT 인접 레벨 규칙(`level_policy.go`)과 `kana_*`/`kanji_*` Skill 분류가 있다.
- 결정:
  - B1 (이번 단계): 위치와 언어 구분 계층만 일반화한다.
    - `cmd/ja/seeder` → `cmd/seeding`(main), `cmd/ja/catalog` → `cmd/seeding/catalog`, JSON → `cmd/seeding/data/<언어 코드>/`.
    - `go:embed`는 `..` 경로를 쓸 수 없으므로 `cmd/seeding/data` 패키지가 `*/*.json`을 embed하고, catalog가 `<language>/<file>`로 읽는다.
    - registry 항목과 `LevelCatalog`에 `Language`를 둔다. 조회는 `LevelCatalogsFor(language)`, `LevelCatalogFor(language, level)`, `DefaultProficiencyLevel(language)`로 바꾼다.
    - → ADR-068에서 `catalog`·`data` 패키지를 `cmd/seeding`에 합쳤다. registry 목록은 파일 경로로 대체했고, 조회 함수는 `levelCatalogsFor(language)` 하나만 남았다(이후 `loadLevelCatalogs(language)`로 바꿈).
    - 언어 코드는 `catalog.Japanese` 상수 하나로 모은다. material/question key 접두어와 `Language` 필드는 이 상수에서 만든다. 값은 그대로라서 DB의 `material_key`·`question_key`는 바뀌지 않는다.
    - seeder에 `-language` 플래그(기본 `ja`)를 둔다. 이름은 기존 `generate_listening_audio`의 `-language`에 맞췄다. 등록되지 않은 언어면 바로 종료한다.
    - → 2026-10-10: flag를 없애고 필수 위치 인자로 바꿨다(`go run ./cmd/seeding ja`). 인자 개수가 틀리거나 `data/` 아래 없는 언어면 사용법과 가능한 언어 목록을 보여주고 exit 2로 끝난다. 대소문자·공백 정규화도 지웠다.
  - B2 (다음 단계, 미착수): 일본어 전용 builder(kana, vocab kana/kanji 문항, `ScriptLabel`, 탁점 hint)를 `cmd/seeding/ja`로 분리한다. → ADR-066에서 데이터를 공통 record로 옮기는 방향으로 대체됐다.
  - 하지 않기로 한 것: vocab schema를 `word/reading` 등으로 일반화하거나 언어 plugin interface를 두는 일은 두 번째 언어가 실제로 들어올 때 한다. 구현체가 하나뿐인 interface는 설계가 맞는지 검증할 수 없다. 또 payload 키를 바꾸면 DB 데이터 migration과 bot 렌더러 수정까지 번진다.
- 결과:
  - 새 언어를 추가할 때 데이터 위치(`data/<code>/`)와 registry 항목은 정해졌다. 다만 그 언어용 schema·builder와 서버 쪽 레벨 규칙·Skill 분류는 여전히 따로 만들어야 한다.
  - 변경 전후 seed 출력(material 2,242개, question 5,318개)이 바이트 단위로 같음을 확인했다. 기존 DB 재seed가 필요 없다.
  - 실행 명령이 `go run ./cmd/ja/seeder` → `go run ./cmd/seeding`으로 바뀐다.

## ADR-066: Seed 데이터를 DB row 모양의 공통 record로 통일한다 (N4 먼저)

- 날짜: 2026-10-08
- 상태: 승인됨. N4 적용 완료, N5는 ADR-067로 적용 완료. ADR-065의 B2(일본어 builder 분리) 방향을 대체한다
- 배경:
  - 같은 문항 유형이 level마다 다른 JSON 형식이었다. N4는 거의 모든 문항이 `QuestionSeed` 형식이었고, 청해·독해만 별도 형식이었다. N5는 어휘 문맥(`word_id, form_options, clozes[]`), 어순(`grammar_id, chunks`), 문법 형태(문법 자료 안의 `cloze_prompt`)가 전부 제각각이었다.
  - 그래서 seeder에 문항 유형마다 builder와 key 규칙이 따로 있었다.
  - 전체 문항 5,318개 중 N5의 3,668개(kana, 어휘 뜻·회상·손글씨·한자, 문법 뜻)는 JSON에 없고, seeder가 실행될 때 Go로 생성한다.
- 결정:
  - 데이터 형식을 DB 테이블 row 모양 두 가지로 통일한다. seeder는 문항 유형을 몰라도 된다.
    - level 디렉토리 `data/<language>/<level>/`의 모든 JSON 파일은 `{materials, questions}` 한 가지 모양이다.
    - `materials[]`: `{material_key, category, title, difficulty, payload, questions[]}`. 그 material에 연결되는 문항을 `questions[]` 안에 넣는다. DB의 `questions.material_id` 1:N 외래키와 같은 구조라서, 없는 material을 가리키는 문항이 구조상 생길 수 없다. material 하나에 문항 수 제한은 없다. payload는 seeder가 해석하지 않고, category와 bot Study 화면 렌더러 사이의 계약이다.
    - 최상위 `questions[]`: material이 없는 문항(청해)만 둔다. 문항 필드는 `{question_key, item_type, type, category, prompt, options, correct_answer, explanation, audio_script?, difficulty}`이다.
    - language와 level은 디렉토리에서 정한다. 로더는 디렉토리 안의 `*.json`을 이름순으로 합친다. 그래서 지금은 level당 파일 하나(N4 `records.json`, 약 1.8MB)로 두고, 커지면 데이터만 개수 기준으로 나누면 된다. 코드는 바꿀 필요가 없다. → ADR-067에서 level 디렉토리를 없애고 `data/<language>/<level>.json` 파일 하나로 바꿨다.
    - (같은 날 보강) 처음에는 `materials/`와 `questions/`를 별도 파일로 두고 `material_key`로 연결했다. 위의 이유로 중첩 구조로 바꿨다.
  - `question_key`와 `material_key`는 모든 record에 명시한다(K1). DB upsert는 `ON CONFLICT (question_key)`로 기존 row를 찾고, 학습 진도는 `question_id`에 붙어 있다. 그래서 전환된 문항은 예전 builder가 만들던 key를 그대로 가진다(예: `ja:listening:n4:…`, `ja:reading:…:question:1`, `ja:question:n4:…`). key 형식이 여러 가지로 남지만 데이터의 차이일 뿐 코드에 분기는 없다.
    - 대안 K2(DB key를 한 번에 새 규칙으로 UPDATE)는 기각했다. 운영 DB를 직접 바꿔야 하고, 순서가 틀리면 중복 row가 생긴다.
  - 변환은 현재 builder의 출력을 그대로 저장하는 방식으로 한다. 그러면 변환 전후 seed 결과가 같다는 걸 key별로 비교해 증명할 수 있다.
  - N4를 먼저 전환한다. N5는 자동 생성 문항을 어떻게 할지(데이터로 저장 / 생성 도구로 보존) 결정한 뒤에 전환하고, 그때까지 기존 builder를 유지한다. → ADR-067에서 데이터로 저장하고 생성 코드는 지우기로 결정하고 전환했다.
- 결과:
  - N4 legacy 파일 5개를 `data/ja/n4/records.json` 하나(material 1,125개, 그 안의 문항 1,325개, 최상위 청해 80개)로 바꿨다. `QuestionSeed` 타입, seed key 규칙(`source_id`에서 key 생성), `categoryForItemType`, 중복이던 material store interface 4개를 제거했다.
  - `n4_grammar.json`의 cloze 필드(`cloze_prompt`, `correct_answer`, `form_options`) 100건은 N4 문법 문항 생성이 꺼져 있어서 쓰이지 않았다. payload에 없으므로 버렸다(git 이력에 남음). N4 파일에만 있던 `level` 필드도 버렸다.
  - 전환 기간에는 seeder 경로가 두 개다(N5 legacy builder, record 경로). N5까지 전환하면 legacy 경로 전체를 지운다. → ADR-067에서 지웠다.
  - 새 문항을 추가할 때 `question_key`를 직접 정해야 한다. catalog 테스트가 key의 유일성, 언어 접두어, material 참조를 검사한다.

## ADR-067: N5도 record로 전환하고 seed 때 문항을 생성하지 않는다

- 날짜: 2026-10-08
- 상태: 승인됨. ADR-066에서 보류한 N5 전환을 끝낸다
- 배경:
  - N5 문항 3,913개 중 3,668개(kana 읽기·회상·손글씨, 어휘 뜻·회상·손글씨·한자, 문법 뜻)는 seeder가 실행될 때마다 Go로 생성됐다. 그래서 seeder에 legacy builder 경로와 record 경로가 함께 있었다.
  - 오답 선지는 seeder 전체가 공유하는 난수 하나(`rand.NewSource(1)`)로 뽑았다. 그래서 N5 단어를 하나 추가하면 그 뒤에 생성되는 문항들의 오답이 재seed 때 바뀌었다.
  - 생성되는 유형은 앱이 정한 초급 연습 유형이고, JLPT 문항 유형이 아니다. JLPT는 모두 4지선다라서 입력·손글씨 문항이 없다. 뜻 고르기도 JLPT 유의어 문항(일본어→일본어)과 다르다.
- 결정:
  - 문항 유형은 JLPT 체계(N3 기준)를 따른다. 어휘(한자읽기·표기·문맥규정·유의어·용법), 문법(문법형식·문장 조립·글의 문법), 독해(단문·중문·장문·정보검색), 청해(과제이해·포인트이해·개요이해·발화표현·즉시응답)다. 이 유형들은 단어 목록에서 자동으로 만들 수 없어서 N4처럼 작성한다.
  - 그래서 문항 생성 코드는 남기지 않는다. 지금 N5 생성 문항은 builder 출력을 그대로 데이터로 저장한다. 학습 이력이 붙어 있으므로 지우지 않는다. 지울지는 별도로 결정한다.
  - level당 JSON 파일 하나를 둔다(`data/<language>/<level>.json`). ADR-066의 level 디렉토리와 디렉토리 안 파일 합치기를 대체한다.
  - seeder는 모든 level을 record 경로 하나로 처리한다. legacy builder, 유형별 데이터 타입, `KanaMap`·`ScriptLabel`, 난이도 상수를 제거했다.
  - 검토한 다른 방식:
    - A. seed 때 생성한 결과를 record로 바꿔 넣는다. 선지가 바뀌는 문제가 남고, 생성 결과를 검수할 수 없다.
    - B. 생성 도구가 JSON에 써 두고 seeder는 JSON만 읽는다. 같은 날 `cmd/seeding/generate`로 구현했다가, JLPT 유형을 따르기로 하면서 쓸 곳이 없어져 지웠다.
    - C. 정답만 저장하고 런타임에 오답을 생성한다. 기각한 이유:
      - 저장된 선지를 쓰는 작성 문항과 생성 문항으로 서버에 분기가 생긴다.
      - 일본어 전용 오답 규칙과 매 요청마다의 후보 조회가 서버에 들어간다.
      - 단어가 추가되면 과거에 보여준 선지를 다시 만들 수 없다.
- 결과:
  - N5 legacy 파일 7개(`kana.json`, `n5_*.json`)를 `data/ja/n5.json` 하나로 대체했다. material 1,117개, 그 안의 문항 3,863개, 최상위 청해 50개다. `n4/records.json`은 `data/ja/n4.json`으로 옮겼다.
  - 변경 전후 seed 출력이 key별로 같다(material 2,242개, question 5,318개, 차이 0). 기존 DB를 다시 seed할 필요가 없다.
  - 데이터 검사를 record 공통 규칙으로 옮겼다. category별 payload 필수 필드, 선택형 문항의 서로 다른 선지 4개, 난이도 범위(DB CHECK와 같은 1~10), level별 item type 고정이다. 로더는 모르는 JSON 필드를 거부한다. (→ 2026-10-10: 로드된 level 데이터에서 `Language`·`Level` 필드를 지우면서 level별 item type 고정 검사도 지웠다.)
  - 선지는 문항마다 고정이고, 런타임에는 순서만 섞인다. 매번 다른 선지가 필요하면 후보 풀을 데이터에 두고 런타임에 3개를 뽑는 방식을 별도 결정으로 다룬다.
  - 파일 하나가 너무 커지면 나눌 때 registry와 로더를 같이 고쳐야 한다(지금 N5 2.4MB, N4 1.8MB).

## ADR-068: seed catalog 패키지를 seeder에 합치고 record 타입을 model 타입으로 바꾼다

- 날짜: 2026-10-08
- 상태: 승인됨. ADR-065의 `cmd/seeding/catalog`·`cmd/seeding/data` 패키지 분리를 대체한다
- 배경:
  - `catalog` 패키지는 데이터를 material row로 바꾸는 규칙(유형별 builder, `MaterialKeyFor*` key 규칙)을 DB 쓰기와 분리하려고 만들었다(`1ca4684`). ADR-067 이후 JSON이 이미 DB row 모양이라 그 규칙이 없다. record 타입, JSON 로더, registry, 필드를 복사하는 `BuildRecordMaterials`만 남았다.
  - 패키지 밖 사용처는 seeder와 `generate_listening_audio`의 flag 기본값(`DefaultProficiencyLevel`) 하나였다. 이 기본값 때문에 admin 바이너리가 seed JSON 약 4MB를 같이 embed했다.
  - `data` 패키지는 `go:embed`가 `..`를 못 쓰기 때문에 있었다. embed 선언을 `cmd/seeding`에 두면 `data/*/*.json`은 하위 경로라서 이 제약이 없다.
- 결정:
  - `catalog`와 `data` 패키지를 `cmd/seeding`(main)의 `records.go`로 합친다. JSON 위치는 그대로다. (2026-10-10에 `records.go`·`records_test.go`를 `main.go`·`main_test.go`로 합쳤다.)
  - 운영 코드에서 쓰지 않던 것은 정리한다. `LevelCatalogFor`는 테스트 helper로(2026-10-10에 이 helper도 지웠다), category별 payload 타입은 테스트 파일로 옮긴다. `DefaultProficiencyLevel`은 지운다. registry에 직접 쓴 언어·level 값을 다시 정규화하던 코드도 지운다(flag 입력의 언어 정규화는 유지. 2026-10-10에 이것도 지웠다).
  - admin 도구는 flag 기본값을 `"ja"`, `"N5"` 문자열로 직접 쓴다.
  - (같은 날 보강) 타입 중복을 정리한다.
    - seed JSON을 `model.Material`·`model.Question`으로 바로 읽는다. 파일 key가 두 타입의 JSON tag와 같아서 가능하다. `questionRecord`, `levelFile`, `linkedRecord`, 필드 복사용 `buildRecordMaterials`를 지운다. `materialRecord`는 `model.Material`에 중첩 문항을 붙인 래퍼만 남는다.
    - 언어와 level은 파일 경로에서 정한다(`data/ja/n5.json` → `ja`, `N5`). registry 목록과 그 타입을 지운다. 새 언어나 level은 파일만 추가하면 된다.
    - `Material.UpsertBatch`가 `RETURNING id, material_key`로 각 material의 `ID`를 채운다. seeder의 ID 재조회(`materialKeyStore`, `loadRecordMaterialIDs`)와, 그 용도로만 쓰이던 `GetByMaterialKeys`를 지운다.
    - 테스트의 payload 타입 5개를 category별 필수 key 표 하나로 바꾼다.
  - (2026-10-10 보강) seeder 실행 코드를 줄인다.
    - 전역 registry(`registeredCatalogs`)와 언어 필터(`levelCatalogsFor`) 대신 `loadLevelCatalogs(language)` 함수가 요청한 언어의 파일만 읽는다. 실패는 panic 대신 error로 돌려준다. (같은 날 레벨별 묶음 없이 material·question 목록을 바로 돌려주는 `loadRecords`로 바꿨다.)
    - `buildRecordQuestions`의 내용 검사(필수 필드, 중복 key, 청해 위치)를 지운다. 같은 embed 파일을 테스트(현재 `main_test.go`)가 검사한다. DB가 돌려준 row ID로 연결하는 검사만 남긴다. (→ 2026-10-10: 이 row ID 검사도 지웠다. material upsert가 `ON CONFLICT DO UPDATE ... RETURNING`으로 모든 row의 ID를 돌려주고, 0이 들어가도 `questions.material_id` FK가 거부한다. 연결은 `collectQuestions`가 한다.)
    - question upsert가 실패해도 exit 0으로 끝나던 버그를 고친다(`log.Fatalf`).
    - DB 연결은 [ADR-069](#adr-069-cmd-바이너리-공통-셋업을-internalbootstrap으로-모은다)의 `bootstrap.OpenDB`를 쓴다.
- 결과:
  - seed 쪽 패키지가 3개에서 1개가 됐다. seed 동작과 데이터는 바뀌지 않는다.
  - admin 도구의 기본 level과 registry 첫 항목은 따로 관리된다. 어긋나도 영향은 admin flag 기본값뿐이다.
  - 타입 정리 전후 seed 출력(material 2,242개, question 5,318개)이 key별로 같다.
  - seed 순서가 파일 경로순(n4 → n5)이 된다. 빈 DB에 새로 seed하면 N4 row가 먼저 id를 받는다. 기존 DB는 key로 upsert하므로 영향이 없다.
  - seed 파일 형식이 model의 JSON tag에 묶인다. tag 이름을 바꾸면 로더가 기존 key를 모르는 필드로 거부하므로 테스트에서 드러난다.
  - model 타입에는 DB가 채우는 필드(`id`, `content_id`, `audio_path` 등)도 있어서, JSON에 써도 디코딩은 통과한다. 데이터 테스트가 이 필드들이 비어 있는지 검사한다.
  - 보강 후 seeder는 데이터 내용을 검사하지 않는다. JSON을 고친 뒤 `make test` 없이 seed하면 빈 문자열 같은 값이 DB 제약에 걸리지 않고 들어갈 수 있다. 중복 question key는 같은 INSERT 문 안이면 PostgreSQL이 거부하지만, batch가 나뉘면 뒤 row가 앞 row를 덮어쓴다.
  - 다른 언어를 추가할 때 seeder 변경은 없다. `data/<code>/<level>.json` 파일만 추가한다. 언어별 차이(문자, 어형 변화, CEFR 같은 level 체계)는 payload를 읽는 bot 렌더러와 서버 level 규칙에서 처리해야 한다.

## ADR-069: cmd 바이너리 공통 셋업을 `internal/bootstrap`으로 모은다

- 날짜: 2026-10-10
- 상태: 승인됨. [ADR-059](ADR-059_architecture_simplification.md) §8.8 "cfg → options 매핑은 조립부가 한다"와 §8.9 드라이버 소유 표를 보강한다
- 배경:
  - `initDB`가 `cmd/` 5곳(server, seeding, admin 3개)에 복사돼 있었다. 그중 3곳은 `config.DBConfig.DSN()`을 쓰지 않고 DSN 문자열을 직접 조립했다. 설정 항목이 늘면 이 3곳만 어긋난다.
  - TTS client·S3 store·`AudioService` 조립(cfg → `TTSOptions`·`S3Options` 매핑)이 server와 `generate_listening_audio`에 똑같이 있었다.
  - 5곳 모두 `_ "github.com/lib/pq"`를 import했다. `repository`가 이미 `lib/pq`를 import하므로 driver 등록에는 필요 없었다.
  - 연결 생성은 쿼리를 다루는 `repository`의 역할이 아니다. import 경계상 sqlx·`lib/pq`를 쓸 수 있는 곳이 `repository`뿐이라는 이유로 그곳에 두면 역할이 섞인다.
- 결정:
  - 둘 이상의 `cmd/` 바이너리가 쓰는 셋업 코드는 `internal/bootstrap`에 둔다. 지금은 `OpenDB(config.DBConfig)`와 `NewAudioService(cfg, questions)` 두 개다.
  - 한 바이너리만 쓰는 셋업은 그 바이너리에 남긴다. server의 connection pool 설정, Redis 연결, logger, Telegram 조립이 그렇다. 바이너리마다 다른 정책(API key가 없을 때 server는 audio 없이 뜨고 admin 도구는 거부)도 호출부가 정한다.
  - `bootstrap`은 `config`·`external`·`repository`·`service`를 import할 수 있다. `internal/` 안에서는 아무도 `bootstrap`을 import하지 않는다. `lib/pq`·sqlx 소유 목록에 `bootstrap`을 추가한다(`internal/import_boundary_test.go`).
- 결과:
  - `cmd/`의 `initDB` 4개와 blank import 5개가 없어졌다. server의 `initDB`는 `OpenDB` 위에 pool 설정만 더한다.
  - `internal/` 패키지가 하나 늘었다. ADR-059 §8.1은 그 보강 기간에 패키지 수를 12개로 유지했지만, 영구 규칙은 아니다.
  - 새 admin 도구는 `bootstrap.OpenDB(cfg.DB)` 한 줄로 DB에 연결한다. DSN 형식이 바뀌어도 `config.DBConfig.DSN()` 한 곳만 고치면 된다.

## ADR-070: Telegram Bot API 호출 실패는 `TelegramClient` 한 곳에서 log한다

- 날짜: 2026-10-11
- 상태: 승인됨. [CONVENTIONS.md](../CONVENTIONS.md) Go §6 "하위 layer는 직접 log하지 않는다"의 예외로 둔다
- 배경:
  - bot flow에서 `TelegramClient`를 호출하는 곳이 111곳이다. 그중 105곳이 반환 error를 버린다. 결과 메시지 같은 전송은 보내고 잊는 방식이라 실패해도 할 일이 없기 때문이다.
  - 2026-10-11 Quiz 세션 중 결과 메시지(정답 표시 + [다음 문제] 버튼) 전송이 Telegram 429로 거부됐다. slog log에는 아무것도 남지 않았다. tgbotapi Debug stdout을 뒤져서야 원인이 나왔다. 사용자는 다음 문제로 넘어갈 버튼이 없어 진행이 멈췄다.
  - `TelegramClient` method는 `context.Context`를 받지 않는다. 그래서 boundary에서 log하려면 105곳을 모두 고쳐야 한다.
- 결정:
  - `TelegramClient`의 모든 API 호출이 `send`/`request` wrapper를 지나게 한다. 실패하면 WARN `telegram.api.failed`를 한 번 남긴다.
  - log field는 `method`, 대상(`chat_id` 또는 `callback_query_id`), `error`다. Telegram이 거부한 경우 `error_code`와 `retry_after_sec`를 더한다. 그래서 rate limit과 네트워크 오류를 구분할 수 있다.
  - error는 지금처럼 호출자에게도 돌려준다. 호출자의 처리 방식은 바꾸지 않는다.
  - Quiz 텍스트 답을 기다리는 문제가 없어서 일반 텍스트를 무시할 때 INFO `telegram.message.ignored`를 남긴다. "bot이 답이 없다"는 신고가 전송 실패인지 무시인지 log로 가릴 수 있게 하려는 것이다.
- 버린 대안:
  - 105곳 모두에서 error를 검사하고 log: `interaction_id`가 붙어 정확하다. 하지만 같은 코드가 105번 반복된다. 새 호출이 추가될 때 빠뜨리기도 쉽다.
  - 모든 method에 ctx 추가: `interaction_id`를 얻는 대신 105곳을 고쳐야 한다. 실패 log가 생긴 뒤 상관관계가 실제로 부족하다고 확인되면 다시 검토한다.
- 결과:
  - Bot API 실패가 전부 slog JSON log에 남는다. 새 호출도 `TelegramClient`를 거치면 자동으로 포함된다.
  - log에 `interaction_id`가 없다. update log와는 시간과 `chat_id`로 맞춘다.
  - 이미 error를 검사하고 직접 log하는 호출자 6곳은 같은 실패가 두 줄로 남을 수 있다.
  - 내용이 같은 edit(`message is not modified`) 같은 무해한 400도 WARN으로 남는다. 잡음이 많아지면 level을 조정한다.
  - 429 retry는 여기서 다루지 않는다. 이 log로 실패 빈도를 본 뒤 따로 결정한다.


## ADR-071: 진행 중 Quiz에서 대기 중이 아닌 텍스트는 이어하기로 받는다

- 날짜: 2026-10-11
- 상태: 승인됨
- 배경:
  - Quiz 텍스트 답을 받으면 bot은 "텍스트 입력 대기" 상태를 먼저 지운 뒤 결과 메시지([다음 문제] 버튼 포함)를 보낸다.
  - 결과 메시지 전송이 실패하면(2026-10-11 Telegram 429, [ADR-070](#adr-070-telegram-bot-api-호출-실패는-telegramclient-한-곳에서-log한다)) 사용자에게 누를 버튼이 없다. 같은 답을 다시 보내도 대기 중인 입력이 없어 무시된다. 그래서 진행이 막힌다.
  - 버튼 경로는 이미 복구된다. 답이 이미 처리된 버튼을 다시 누르면 다음 미답 문제로 보낸다(`redirectToNextUnansweredQuestion`). 메뉴 "학습 시작"도 진행 중 Quiz를 이어 준다. 하지만 막힌 사용자가 그 경로를 알 방법이 없다.
- 결정:
  - 일반 텍스트가 LLM 질문도 텍스트 답도 아니면, 그 사용자의 진행 중 Quiz(DB `in_progress`, 최신 1건)를 찾는다. 있으면 아래 순서로 보낸다.
    1. "진행 중인 퀴즈로 돌아갈게요" 안내. 직전 문제의 정오답을 함께 보여준다. 결과 메시지가 사라졌을 수 있기 때문이다. 정오답은 working set에서 읽는다.
    2. 다음 미답 문제. 메뉴 이어하기와 같은 `showQuestion`을 쓴다. 다음 문제가 텍스트 문제면 텍스트 입력 대기 상태도 다시 걸린다.
  - 진행 중 Quiz가 없으면 지금처럼 무시하고 `telegram.message.ignored`를 log한다.
  - working set이 만료됐으면 기존 "세션 상태가 만료되었습니다" 안내와 메인 메뉴를 보낸다.
- 버린 대안:
  - 결과 전송 성공 후에 텍스트 입력 대기 상태를 지우기: 결과 메시지 하나만 고친다. 다음 문제 전송 실패 같은 다른 경로는 그대로 막힌다.
  - 전송 retry(429는 `retry_after` 대기, 5xx는 backoff): 짧은 장애의 빈도는 줄인다. 이번처럼 2분 가는 장애는 못 막는다. 이 ADR과 별개로 따로 판단한다.
- 결과:
  - 진행 중 Quiz가 있으면 아무 텍스트나 보내도 현재 위치로 돌아온다. 결과 메시지가 사라져도 사용자 혼자 복구할 수 있다.
  - 대기 중이 아닌 텍스트가 올 때마다 DB 조회가 1번 일어난다. 진행 중 Quiz가 있으면 메시지 2개가 더 나간다. `sessions(user_id)` index가 없어 user 수가 늘면 조회 비용이 커진다. 대기 중이 아닌 텍스트일 때만 조회하므로 지금은 index를 추가하지 않는다.
  - 진행 중 Study 세션(버튼 전용)은 대상이 아니다. 같은 막힘이 Study에서 확인되면 다시 검토한다.


## ADR-072: 통계 진도는 `user_material_progress`를 읽을 때 집계한다

- 날짜: 2026-10-13
- 상태: 승인됨
- 배경:
  - 통계 메뉴에는 정답률만 있었다. 레벨별로 학습 자료를 얼마나 봤는지 알 방법이 없었다.
  - "본 자료"를 무엇으로 셀지 정해야 했다. 후보는 Study 세션에 배정된 자료(`session_materials`)와 Study 카드에서 학습 처리된 자료(`user_material_progress`)다.
- 결정:
  - 통계를 `정답률 | 진도` 탭으로 나눈다. `menu:stats`는 정답률 탭, `stats:progress`는 진도 탭이다. 같은 메시지를 edit해서 탭을 바꾼다.
  - 진도는 사용자 언어의 레벨별 `(한 번이라도 학습한 자료 수) / (전체 자료 수)`다. 학습 기준은 `user_material_progress.times_studied > 0`이다. SRS와 유지·제외 설정이 이미 쓰는 기준이다. 세션에 배정만 되고 카드를 넘기지 않은 자료는 세지 않는다.
  - 분모는 해당 레벨 자료 전체다. 학습 제외 자료도 포함한다. 비율은 내림해서 모두 학습했을 때만 100%가 된다.
  - 탭을 열 때마다 `materials` LEFT JOIN `user_material_progress` 한 번으로 집계한다. cache나 사용자별 counter는 두지 않는다.
- 버린 대안:
  - 사용자·레벨별 counter 테이블: 읽기는 O(1)이 된다. 대신 Study flush마다 counter를 같이 갱신해야 하고, seed로 자료가 늘면 분모를 다시 맞춰야 한다. 탭은 사용자가 누를 때만 열리므로 쓰기 경로를 복잡하게 할 이유가 없다.
  - Redis cache: 같은 이유로 무효화 지점만 늘어난다.
- 결과:
  - 비용은 catalog 크기에 비례한다. 진행 상태는 PK `(user_id, material_id)`로 찾는다. dev DB(ja 자료 2,242개)에서 실행 2.8ms였다. 사용자 수가 늘어도 요청당 비용은 그대로다.
  - catalog가 수만 개로 커지거나 진도를 push 메시지처럼 자주 계산하게 되면 counter나 cache를 다시 검토한다.
  - Quiz에서 연결 문항만 푼 자료는 진도에 들어가지 않는다.

## ADR-073: 한자 구성 해설은 단어 payload에 복사해 두고 버튼을 눌렀을 때만 보여 준다

- 날짜: 2026-10-19
- 상태: 승인됨. 해설 화면의 `같은 발음 요소`·`계열로 배운 단어` 줄과 `series` 데이터는 [ADR-074](#adr-074-한자-해설의-읽기는-같은-부품이-아니라-이-단어에서-그-한자를-어떻게-읽는지로-보여-준다)가 대체
- 배경:
  - 단어를 배우면서 한자 구성(의미 요소·발음 요소)도 익히게 하려 한다. 목표는 같은 발음 요소(예: 清·晴·精의 青)를 여러 단어에서 알아보고, 처음 보는 한자의 뜻과 음독을 추론하게 하는 것이다.
  - 해설은 한자의 속성이지만 Study 카드는 단어 단위다. 같은 한자가 여러 단어에 나온다(N5–N3 어휘 기준 時 10단어, 方 11단어).
  - 해설을 어디에 저장할지, 누가 언제 만들지, 카드에 어떻게 보여 줄지 정해야 했다.
- 결정:
  - 저장: vocabulary `materials.payload`에 선택 key `kanji_notes`를 둔다. 한자별 note 배열이고 구조는 `model.KanjiNote`다(`type`, `parts[{part, role: meaning|sound|form, gloss_ko, on}]`, `explanation_ko`, `series[{char, on}]`). DB schema, migration, seeder 코드는 바꾸지 않는다.
  - 같은 한자의 note는 그 한자를 쓰는 모든 단어에 똑같이 복사한다. 복사본이 하나라도 다르면 `cmd/seeding` 테스트가 실패한다.
  - 생성: 런타임이 아니라 미리 한 번 만든다. 어휘에 쓰인 한자 1,421자를 대상으로 Claude(opus)가 생성하고 900자에 note를 달았다. 음독은 KANJIDIC2, 부품은 IDS 분해와 자동 대조했다. 공유 음독과 계열 예시의 음독은 JMdict 일반 어휘에서 실제로 쓰이는지 확인해, 사전에만 있는 음독은 뺐다. 경고 항목과 표본은 별도 검수했다. 단순 상형·지사자와 기원이 불확실한 한자는 넣지 않는다. 절차와 수치는 [workthrough](../workthrough/2610/2610190043_kanji_notes.md)에 있다.
  - 표시: 카드 본문은 그대로 두고 `🈁 한자 해설` 버튼만 붙인다. 버튼은 `kanji_notes`가 1개 이상일 때만 생긴다. 누르면 같은 메시지가 해설로 바뀌고 `← 학습 카드로`로 돌아온다(`study:{session_id}:kanji:{material_id}`). 학습 처리, Quiz, SRS, 세션 생성은 바뀌지 않는다.
  - 해설 화면에는 사용자가 이미 학습한 단어 중 같은 발음 요소를 다른 한자로 쓰는 단어를 발음 요소당 최대 3개 보여 준다. 읽기는 붙이지 않는다. `user_material_progress`(`times_studied > 0`)에서 시작해 payload JSONB를 펼쳐 찾는다. 조회가 실패해도 해설은 보여 준다.
- 버린 대안:
  - 한자 단위 테이블: 중복은 없다. 대신 migration, repository, seeder 로더, 렌더러 의존성이 모두 늘어난다. 지금 기능은 한자를 FK로 참조하지 않는다.
  - 한자 사전 JSON을 서버 바이너리에 embed: 학습 데이터가 DB와 바이너리 두 곳으로 나뉜다.
  - 버튼을 누를 때 LLM으로 생성하고 저장(lazy): 검수 전 해설이 바로 노출된다. payload에 저장하면 재seed 때 덮어써져 결국 별도 테이블이 필요하다.
  - 카드 본문에 항상 표시: 카드가 길어지고 해설이 필요 없는 순간에도 읽게 된다.
- 결과:
  - 해설 추가·수정은 seed JSON 수정 후 재seed다. 진행 중 세션은 Redis snapshot을 쓰므로 새 세션부터 보인다.
  - seed JSON이 커진다. 어휘 2,066개에 note 2,524개가 복사되어 n3/n4/n5.json이 약 16만 줄에서 21만 줄이 됐다.
  - 배운 단어 조회 비용은 사용자 한 명의 학습 이력 크기에 비례하고 사용자 수와 무관하다. payload index는 두지 않는다.
  - 한자별 SRS·숙련도처럼 한자를 직접 참조하는 기능이 생기거나 note 수정이 잦아 복사본 관리가 부담되면 한자 단위 테이블로 옮긴다.
  - 구성 분석의 정확성은 자동 검증(음독·부품)과 검수 범위까지만 보장한다.

## ADR-074: 한자 해설의 읽기는 같은 부품이 아니라 이 단어에서 그 한자를 어떻게 읽는지로 보여 준다

- 날짜: 2026-10-19
- 상태: 승인됨
- 배경:
  - ADR-073의 해설은 한자 단위라서 훈독 단어에도 발음 요소의 음독을 그대로 보여 줬다. 伺う(うかがう)에 司(シ), 伸ばす(のばす)에 申(シン)이 붙어 단어 읽기와 무관한 음으로 읽혔다(2026-10-19 21시 Study 세션).
  - seed 기준으로 발음 요소가 있는 해설 1,843개 중 단어 kana에 그 음이 나오는 것은 646개(35%)다. 발음 요소의 음과 그 한자의 음독이 같은 것도 58%뿐이다.
  - 학습 목표는 "한 한자의 음독을 알면 그 한자가 들어간 다른 단어를 읽을 수 있다"이다. 한자어에서 그 한자의 음독이 단어 읽기에 나오는 비율은 90%(580/640)이고, 음독으로 쓰인 한자 1,018자 중 959자(94%)는 우리 어휘 안에서 음독이 한 종류다. 발음 요소 계열(같은 부품 → 다른 한자의 음)은 이 목표와 다른 경로이고 덜 정확하다.
- 결정:
  - vocabulary `materials.payload`에 선택 key `kanji_readings`를 둔다. 단어 순서대로 `{text, reading, type: on|kun|jukuji, on[]}`(`model.KanjiReading`)이다. `reading`은 이 단어에서 그 한자의 히라가나 읽기다. `on`은 그 한자가 JMdict 일반 어휘에서 쓰는 음독으로, 많이 쓰는 순서로 최대 3개에 이 단어의 음독을 더한다. 今日(きょう)처럼 읽기가 묶음 전체에 붙는 熟字訓은 `text`가 묶음이고 `on`이 없다.
  - 같은 한자의 `on`은 모든 단어에서 같다. 다르면 `cmd/seeding`의 `TestKanjiReadings`가 실패한다(ADR-073의 note 복사 규칙과 같은 방식).
  - 생성은 `scripts/gen_kanji_readings.py`가 자동으로 한다. JmdictFurigana로 단어 읽기를 한자별로 나누고, KANJIDIC2 음독 목록과 비교해(연탁·촉음 허용) 음독·훈독을 판정한다. 수작업은 정렬 보정 9건(`MANUAL`: 사전 정렬이 안 되는 6단어와 日本·日本語)과 판정 보정 5건(`TYPE_OVERRIDE`: 差す·放る는 훈독, 日本·文字는 줄어든 음독)뿐이다. 한자가 있는 어휘 2,961개 전부에 붙었다(음독 2,990 · 훈독 1,575 · 熟字訓 44).
  - 해설 화면은 한자마다 note가 있으면 제목·부품·설명을 보여 주고, 이어서 `이 단어에선: の (훈독)`과 `이 한자의 음독: シン`을 붙인다. note가 없는 한자(事·無 등)도 읽기 줄로 보여 주므로, 버튼은 `kanji_notes`나 `kanji_readings`가 있으면 생긴다.
  - ADR-073의 `같은 발음 요소` 줄과 `계열로 배운 단어` 줄을 지운다. note의 `series` 데이터, `KanjiNoteService`, `MaterialRepository.ListStudiedSoundSeriesWords`도 함께 지운다.
  - note의 부품 표시와 설명 문장은 그대로 둔다. 설명 속 음독(고유 해설 900개 중 751개)은 새 음독 줄과 같은 내용이라 모순되지 않는다.
- 버린 대안:
  - 렌더링할 때 단어 kana에 발음 요소의 음이 있는지만 비교: 데이터 작업은 없지만 "쓰임 / 안 쓰임"만 말할 수 있다. 違い(ちがい)←韋(イ)처럼 훈독 kana에 음이 우연히 들어 있으면 틀린다.
  - 해설에서 읽기 정보를 아예 빼기: 혼란은 사라지지만 "한자 → 음독" 연결이라는 학습 목표도 사라진다.
  - "같은 한자를 음독으로 읽은 배운 단어" 줄: 화면 텍스트가 길어지고, 반복해서 보면 자연히 알게 된다고 판단했다(사용자 결정).
  - 단어별 음독·훈독 판정을 LLM으로 생성: 사전 정렬로 98%가 자동 판정되므로 필요 없다.
- 결과:
  - 새 어휘를 추가하면 `scripts/gen_kanji_readings.py`를 다시 돌린다. 정렬 안 되는 단어는 `MANUAL`에 넣는다. 출력은 결정적이라 입력이 같으면 파일이 바뀌지 않는다.
  - seed JSON이 약 21.2만 줄에서 25.0만 줄로 늘었다(읽기 추가, `series` 제거).
  - EDRDG 데이터(CC BY-SA 4.0)에서 나온 값을 저장하므로 `NOTICE`에 출처를 둔다.
  - 판정 정확도는 JmdictFurigana 정렬과 KANJIDIC2 음독 목록 수준이다. 훈독이 음독 형태와 우연히 같으면 음독으로 판정될 수 있다. 발견하면 `TYPE_OVERRIDE`에 넣는다.

## ADR-075: 학습 팁(Tips) 기능을 폐기하고 LLM 질문 기록 저장만 남긴다

- 날짜: 2026-10-20
- 상태: 승인됨 (ADR-015 폐기)
- 배경:
  - ADR-015는 손글씨 채점을 기다리는 몇 초 동안 Mini App에 팁을 15초 간격으로 보여 주려고 도입했다. 사용자가 원래 원한 것은 대기 시간에 한자 파츠 분석을 보며 익히는 것이었다.
  - 실제로 쌓인 팁 102개는 모두 가나 범주(촉음·탁점·장음·획순 등)다. N4 팁 51개도 가나 이야기라서 N4 단어·문법을 공부하는 현재 사용자에게 맞지 않는다. 같은 내용을 문장만 바꾼 것이 많다(N5 촉음 18개가 거의 "1박자 멈춤"). 틀린 문제와도 무관한 무작위 노출이다.
  - 마지막 팁 생성은 2026-09-10이다. 이후 scheduler는 매일 "bucket full"만 기록했다. 마지막 팁 조회는 2026-09-27이다. 손글씨 문제가 N5에만 있고 하위 레벨 due가 뽑히지 않는 문제([quiz_lower_level_due_starvation](../todos/quiz_lower_level_due_starvation.md)) 때문에 9-28 이후 손글씨가 출제되지 않았다.
  - 한자 파츠 학습은 ADR-073·074의 한자 구성 해설이 단어 단위로 맡고 있다.
- 결정:
  - 팁 표시와 생성을 삭제한다. `TipService`·`tipGenerator`, `GenerateTips`, scheduler 팁 top-up, `GET /api/miniapp/tips`, Mini App 팁 카드, `model.Tip`·`TipCategory`가 대상이다. Mini App은 채점 중 spinner만 보여 준다.
  - `/llm` 질문·답변 저장(`tip_candidates`, ADR-028)은 유지한다. 최근 2주에도 37건이 쌓였고, 문제 오류 신고처럼 콘텐츠 점검에 쓸 수 있는 기록이며, 지우면 복구할 수 없다. `LLMQuestionService`는 `TipRepository.CreateCandidate`를 직접 받는다.
  - `tips` 테이블과 데이터는 남긴다. 코드가 읽지 않으므로 동작에 영향이 없고, DROP migration은 필요해질 때 따로 한다.
- 결과:
  - 채점 대기 화면이 가나 팁 대신 spinner만 보여 준다. 팁 생성 LLM 호출 경로가 사라진다.
  - `tip_candidates`·`TipCandidate`·`TipRepository` 이름은 폐기된 기능을 가리키지만 바꾸지 않는다. 이름 변경은 migration이 필요하므로 별도 작업으로 둔다.
  - 대기 시간 학습 콘텐츠가 다시 필요하면 한자 구성 해설(ADR-073)을 채점 대기 화면에 재사용하는 방향으로 다시 결정한다.

## ADR-076: 한자 해설 제목에 한국 훈음을 붙이고 단어별 읽기 항목에 복사해 둔다

- 날짜: 2026-10-21
- 상태: 승인됨
- 배경:
  - 해설 화면은 한자의 구성(부품·설명)과 읽기만 보여 줬다. 그래서 그 한자가 무슨 뜻인지부터 알 수 없었다(사용자 피드백). note가 없는 한자(事·無 등)는 글자와 읽기 줄만 나왔다.
  - 한국 학습자는 한자를 `펼 신` 같은 훈음으로 익힌다. 훈음 한 줄로 뜻과 한국 한자음을 함께 알 수 있다.
- 결정:
  - 해설 블록 제목을 `伸 펼 신 · 형성문자`로 바꾼다. note가 없는 한자도 `事 일 사`로 나온다. 熟字訓 묶음은 `今日 今 이제 금 · 日 날 일`처럼 글자별 훈음을 잇는다.
  - 저장: `kanji_readings` 항목에 `hun_eum`을 둔다. 읽기 항목은 단어의 모든 한자를 덮고, 이미 `on` 같은 한자 단위 값을 복사해 두고 있다. 같은 한자의 `hun_eum`이 단어마다 다르거나 비어 있으면 `TestKanjiReadings`가 실패한다.
  - 원본: `scripts/kanji_hun_eum.tsv`(한자 1,421자)를 `gen_kanji_readings.py`가 seed에 복사한다. 고칠 때는 TSV를 수정하고 script를 다시 돌린 뒤 재seed한다.
  - 값 규칙:
    - 기본은 한국 사전의 대표 훈음 하나다. 두음법칙을 적용하지 않은 본음을 쓰고, 신자체는 정자 기준이다(学 → 배울 학).
    - 일본어에서 두 뜻이 서로 다른 한국 음으로 쓰일 때만 두 개를 쓴다(楽 → 즐길 락, 노래 악). 이런 한자는 9자다.
    - 훈이 일본어 뜻과 크게 달라 오해를 부르면 일본어 뜻을 붙인다. 咲 → 웃을 소 (일본어: 피다)처럼 쓰고, 39자다.
    - 한국 훈음이 없는 일본 국자는 `일본 한자 (일본어: …)`로 쓴다. 込·枠 2자다.
  - 생성: Claude(opus)가 단어 예시를 보고 초안을 만들었다. 초안을 libhangul 한자 사전(BSD)과 KANJIDIC2 한국 음에 자동 대조했다. 1,432개 항목 중 1,114개가 libhangul 항목과 정확히 같았고, 음 불일치는 2개였다(繰 조로 수정, 掴 괵은 사전 키로 확인). opus가 한 번 더 전수 검수해 위 규칙으로 통일했고(199자 수정), 표준 대표훈에서 벗어난 4자(対·単·制·帰)는 되돌렸다.
- 버린 대안:
  - libhangul 사전만으로 자동 생성: 결정적이고 출처가 분명하다. 하지만 1,421자 중 218자(学·気·会 등 신자체)가 비어 있다. 첫 항목이 대표훈이 아닌 경우도 있다(行 → 시장 항).
  - `kanji_notes`에 넣기: note가 있는 900자만 덮는다.
  - 새 payload key: 렌더러가 세 목록을 합쳐야 하고, 한자 단위 값의 일관성 검사가 두 곳으로 나뉜다.
  - 한자 단위 테이블: ADR-073의 이전 조건(한자를 직접 참조하는 기능)이 아직 없다. 한자 속성이 note·음독·훈음 셋으로 늘었지만, 복사본 일관성은 test가 지킨다.
- 결과:
  - 재seed한 뒤 새 세션부터 보인다. 진행 중 세션은 Redis snapshot을 쓰기 때문이다.
  - seed JSON이 약 25.0만 줄에서 25.5만 줄로 늘었다.
  - 훈음의 정확성은 사전 대조와 검수를 거친 범위까지만 보장한다. 이상한 값을 발견하면 TSV를 고친다.

## ADR-077: 가나(히라가나·가타카나)는 N5 아래 레벨 `NZ`로 분리한다

- 날짜: 2026-10-21
- 상태: 승인됨
- 배경:
  - 가나 material 208개와 문항 622개(읽기·떠올리기 416, 손글씨 206)가 N5로 seed되어 있었다. 가나는 JLPT 범위가 아니라 그 앞 단계인데, N5 진도와 출제 범위에 섞여 있었다.
  - 인접 레벨 규칙(ADR-046) 때문에 N4 사용자에게도 N5인 가나가 계속 나왔다. 복습 due 193건(가나 124, 손글씨 69)이 쌓여 있었다.
- 결정:
  - 가나를 `data/ja/nz.json`으로 옮긴다. 레벨은 파일 이름에서 정해지므로(ADR-066) 코드 변경 없이 `NZ`가 된다.
  - 일본어 레벨 순서를 `NZ → N5 → N4 → N3 → N2 → N1`로 둔다. 인접 규칙은 그대로라서 NZ 사용자는 [NZ, N5], N5 사용자는 [NZ, N5, N4] 범위를 받는다. N4 이상은 가나를 신규로도 복습으로도 받지 않는다.
  - 통계의 레벨별 진도도 같은 순서를 쓰므로 NZ가 맨 앞에 나온다.
  - 기존 DB 행은 `material_key`·`question_key`가 그대로이므로, 가나 행의 `proficiency_level`만 `NZ`로 바꾼다. id와 학습 기록(`user_question_progress`)은 유지된다.
- 버린 대안:
  - NZ를 모든 일본어 범위에 항상 포함: 출제는 지금처럼 이어진다. 하지만 N3 이상도 가나가 슬롯을 계속 차지하고, 인접 규칙에 예외가 하나 생긴다.
  - NZ를 레벨 순서에 넣지 않기: 어떤 사용자에게도 가나가 나오지 않는다.
- 결과:
  - 지금 사용자(N4)의 복습 큐에서 가나·손글씨 193건이 빠졌다. 학습 기록은 남아 있어서, 레벨을 N5 이하로 바꾸면 다시 나온다.
  - N4 이상 사용자가 가나·손글씨를 연습하려면 레벨 범위와 따로 고르는 경로가 필요하다. 세션 비율 preset TODO(`user_selectable_session_mix_presets.md`)에서 다룬다.

## ADR-078: 저녁 Quiz를 아침과 같은 17문항(복습 6)으로 늘린다

- 날짜: 2026-10-21
- 상태: 승인됨
- 배경:
  - Study는 하루 신규 27개(아침 16, 저녁 11, ADR-064)인데 Quiz 신규는 하루 약 17문항(아침 11, 저녁 6)이었다.
  - 최근 14일 동안 학습한 N4 단어 306개 중 Quiz로 한 번이라도 확인한 단어는 160개(52%)였다. 학습한 N3 문법 14개는 Quiz에 한 번도 나오지 않았다. 떠올려 보는 연습 없이 지나가는 자료가 절반이었다.
  - 12월 JLPT N3까지 Study 신규를 40개로 늘리는 안도 검토했다. 하지만 Quiz가 지금도 못 따라가므로 신규를 늘리기 전에 확인 용량부터 맞춘다.
- 결정:
  - 저녁 Quiz를 12문항(복습 8, clamp 후 6) → 17문항(복습 6)으로 바꾼다. 아침과 같은 구성이라 신규 약 11문항이 나오고 하루 Quiz 신규는 약 22문항이 된다.
  - Study 할당량(27/일)은 그대로 둔다.
  - 사용자 부담을 고려해 Study와 1:1(+10문항)이 아니라 +5문항으로 제한한다.
- 버린 대안:
  - Study 신규 40/일: 확인되지 않는 자료가 더 늘어난다.
  - 저녁 22문항(신규 Study와 1:1): 하루 풀이량이 약 100문항까지 늘어 부담이 크다.
  - Study를 20/일로 감축: 확인 비율은 맞지만 시험 전 N3 단어 범위가 약 500개로 줄어든다.
- 결과:
  - 하루 Quiz 풀이량이 약 5문항 늘어난다(복습 포함 약 65~95문항).
  - 현재 레벨 최소 비중은 `ceil(17×4/5)` = 14문항으로 자동 계산된다(ADR-050).
  - 2주 뒤 "학습 후 Quiz로 확인된 비율"을 다시 보고 추가 조정을 판단한다.
