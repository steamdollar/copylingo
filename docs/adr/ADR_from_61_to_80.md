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

## ADR-063: Study Redis 진행 상태에서 version 필드를 제거한다

- 날짜: 2026-09-28
- 상태: 승인됨
- 배경: `StudyActiveSessionState`의 `Version`은 현재 값 `1`만 기록하며, Redis의 Study 키에는 별도 세대 구분이 없다. JSON 해석과 요청한 session ID 일치 여부만으로 잘못된 상태를 걸러낼 수 있다.
- 결정: Study 상태의 `Version` 필드, 상수, 저장 시 대입과 version 검사를 제거한다. JSON 오류와 session ID 불일치 검사는 유지한다. Quiz 상태의 독립적인 version 검사는 변경하지 않는다.
- 결과: 기존 Redis JSON에 남은 `version` 값은 Go JSON 해석 시 무시되므로 마이그레이션 없이 읽을 수 있다. 앞으로 Study 상태 형식이 호환되지 않게 바뀌면 version 필드에 의존하지 말고 필요한 시점에 Redis 키 세대 구분이나 명시적 무효화를 추가한다.

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
    - 언어 코드는 `catalog.Japanese` 상수 하나로 모은다. material/question key 접두어와 `Language` 필드는 이 상수에서 만든다. 값은 그대로라서 DB의 `material_key`·`question_key`는 바뀌지 않는다.
    - seeder에 `-language` 플래그(기본 `ja`)를 둔다. 이름은 기존 `generate_listening_audio`의 `-language`에 맞췄다. 등록되지 않은 언어면 바로 종료한다.
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
  - 데이터 검사를 record 공통 규칙으로 옮겼다. category별 payload 필수 필드, 선택형 문항의 서로 다른 선지 4개, 난이도 범위(DB CHECK와 같은 1~10), level별 item type 고정이다. 로더는 모르는 JSON 필드를 거부한다.
  - 선지는 문항마다 고정이고, 런타임에는 순서만 섞인다. 매번 다른 선지가 필요하면 후보 풀을 데이터에 두고 런타임에 3개를 뽑는 방식을 별도 결정으로 다룬다.
  - 파일 하나가 너무 커지면 나눌 때 registry와 로더를 같이 고쳐야 한다(지금 N5 2.4MB, N4 1.8MB).
