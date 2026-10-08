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
  - B2 (다음 단계, 미착수): 일본어 전용 builder(kana, vocab kana/kanji 문항, `ScriptLabel`, 탁점 hint)를 `cmd/seeding/ja`로 분리한다.
  - 하지 않기로 한 것: vocab schema를 `word/reading` 등으로 일반화하거나 언어 plugin interface를 두는 일은 두 번째 언어가 실제로 들어올 때 한다. 구현체가 하나뿐인 interface는 설계가 맞는지 검증할 수 없다. 또 payload 키를 바꾸면 DB 데이터 migration과 bot 렌더러 수정까지 번진다.
- 결과:
  - 새 언어를 추가할 때 데이터 위치(`data/<code>/`)와 registry 항목은 정해졌다. 다만 그 언어용 schema·builder와 서버 쪽 레벨 규칙·Skill 분류는 여전히 따로 만들어야 한다.
  - 변경 전후 seed 출력(material 2,242개, question 5,318개)이 바이트 단위로 같음을 확인했다. 기존 DB 재seed가 필요 없다.
  - 실행 명령이 `go run ./cmd/ja/seeder` → `go run ./cmd/seeding`으로 바뀐다.

