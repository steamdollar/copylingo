# JLPT N5/N4 어휘 보충

- 날짜: 2026-10-07
- 계기: 학습 자료의 단어 목록이 JLPT 레벨별 필수 어휘와 얼마나 맞는지 점검한 결과, 핵심어 누락이 많았음
- 동작 변화: N5·N4 vocabulary material과 문제가 늘어남. 늘어난 문제 수 때문에 드러난 `QuestionRepository.UpsertSeedBatch` 파라미터 한도 버그를 수정했고, 로컬 DB에 seeder로 반영함

## 기준 소스

JLPT는 2010년 개정 이후 『出題基準』(어휘 리스트)를 비공개로 하고 있다 ([jlpt.jp FAQ](https://www.jlpt.jp/faq/)). 공식에 가장 가까운 소스는 다음 순서다.

| 소스 | 성격 | 이번 사용 여부 |
|---|---|---|
| 旧『日本語能力試験出題基準【改訂版】』(2002) | 주최 측이 발행한 유일한 어휘 리스트. 종이책, 절판 | 미사용 |
| Jonathan Waller(tanos) 리스트 ([open-anki-jlpt-decks](https://github.com/jamsinclair/open-anki-jlpt-decks)) | 구 출제기준을 재구성한 비공식 리스트 | **기준으로 사용** |
| JEV 日本語教育語彙表 | 연구용, 재배포 금지 | 라이선스 때문에 미사용 |

Waller 리스트의 N5 718개와 N4 668개는 공식 비교표([comparison01.pdf](https://www.jlpt.jp/about/pdf/comparison01.pdf))의 구 4급 800어 정도, 구 3급 누적 1500어 정도와 규모가 맞는다. 그래서 N5/N4에 한해 기준으로 쓴다. N3 경계는 어느 리스트든 추정치다.

## 보충 전 진단

| | 들어 있는 단어 중 해당 레벨 비율 | 레벨 필수어 커버리지 |
|---|---|---|
| n5_vocab (540) | 72% N5 (나머지는 인사말 분류 차이·합성어·외래어) | 61% (438/718) |
| n4_vocab (600) | 36% N4, 52% N3 이상 | 40% (268/668) |

## 변경 내역

| 대상 | 내용 |
|---|---|
| `cmd/ja/catalog/data/n5_vocab.json` | 249개 추가 (`n5_word_541`~`789`). 문제는 seeder가 자동 생성 |
| `cmd/ja/catalog/data/n4_vocab.json` | 365개 추가 (`n4_word_0601`~`0965`) |
| `cmd/ja/catalog/data/n4_question_seeds.json` | 새 단어마다 seed 1개, 총 365개. 기존 vocab seed 바로 뒤에 삽입 |
| `internal/repository/question_repo.go` (+test) | `UpsertSeedBatch`를 PostgreSQL bind parameter 한도(65535)를 넘지 않는 크기로 나눠 실행하고, `WithinTx` 하나로 묶어 전부 반영되거나 전부 취소되게 함. 문제 5318개 × 15 = 79770 파라미터라 한 번에 보내면 실패했음 |
| `cmd/ja/catalog/{datasets,materials}_test.go`, `cmd/ja/seeder/main_test.go` | N5 개수 고정값 갱신: 단어 540→789, 문제 2051→2966, kanji_recall 431→599. goparams 적용으로 diff가 커 보임 |

## 판단

- **큐레이션**: 다음은 제외했다.
  - `～円` 같은 접사
  - 기존 단어의 표기 변형 (茶碗/お茶碗, すぐに/すぐ, 下りる/降りる 등)
  - 대체된 단어 (ワープロ, 看護婦, テープレコーダー)
  - 背広·汽車 같은 구식이지만 공식인 단어는 유지했다.
- **표기**: 현대 일반 표기를 따랐다. 보통 가나로 쓰는 단어는 `kanji = kana`로 했다. 상용외 한자나 ateji로 쓰는 단어(飴, 醤油, 可愛い, 温い)도 kana로 둬서 seeder가 어려운 한자 recall 문제를 만들지 않게 했다.
- **N4 seed 생성 방식 (사용자 결정: 혼합)**
  - 한자어 268개: 스크립트로 `vocab_kanji_reading`과 `vocab_orthography`를 번갈아 결정론적으로 생성했다.
  - 가나 전용어 98개: sonnet이 `vocab_context`와 `vocab_usage`를 번갈아 작성했다.
  - 결과적으로 전체 vocab seed에서 읽기·표기 유형 비중이 40%에서 약 55%로 늘었다.
- **읽기가 여러 개인 한자**: 止める(とめる/やめる), 開く(あく/ひらく), 空く(あく/すく)는 kanji_reading 문항으로 내면 정답이 둘이 된다. 그래서 뜻을 함께 제시하는 orthography로 바꿨다.

## 작성 문항 검수 (opus)

sonnet이 작성한 context/usage 97문항을 opus가 독립적으로 검수했고, 74문항을 다시 썼다.

| 사유 | 문항 수 |
|---|---|
| 자명한 오답 | 54 |
| 복수 정답 | 7 |
| 부자연스러운 일본어 / 수준 초과 | 5 |
| 해설 오류 | 4 |
| 기타 | 4 |

N4 `あ`(감탄사)는 좋은 문항을 만들기 어려워 단어와 seed를 함께 삭제했다.

## 남은 확인 사항

- 형식명사 よう(0896), こと(0717), はず(0764), ため(0827)의 문항은 실질적으로 문법을 묻는다. grammar 유형으로 옮길지 검토가 필요하다.
- usage_0887 アジア: 고유명사라 용법 문항으로 만들 여지가 적다. 삭제 후보다.
- 다시 쓴 문항은 원어민 검수를 받지 않았다.
- 기존 데이터에 원래 있던 중복: `月(つき)` n5_word_038/149, `掛ける(かける)` n5_word_464/497. DB material과 학습 이력에 미치는 영향을 확인한 뒤 정리해야 한다.
- seeder는 question upsert가 실패해도 `log.Printf` 후 return하므로 exit 0으로 끝난다. 실패를 알아채기 어렵다.

## 검증

- `go test ./cmd/ja/...`: ok
- `make test`: exit 0 (repository 수정 후 재실행)
- `go run ./cmd/ja/seeder`: material 2242개, question 5318개 upsert 성공. DB의 N4 vocabulary 문제 965개가 단어 수와 일치
- 데이터 검증: id/source_id 유일, 모든 신규 N4 단어에 seed 정확히 1개, multiple_choice 정답이 options 안에 있음, n5+n4 간 신규 (kana, kanji) 중복 없음
