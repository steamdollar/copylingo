# N5 독해 seed 10→40 확장 (0011~0040, Case C)

- **날짜**: 2026-07-23
- **범위**: ADR-036 MVP의 original 지문 10개(`n5_reading_0001`~`0010`)에 30개를 추가해 총 40개로 확장. 데이터 개수만 증가하고 seeder/repo/session 로직은 ADR-036에서 개수-불변으로 이미 구현되어 코드 변경은 데이터 파일 + integrity 기대치 1줄뿐.
- **위임**: 사용자 승인(고정 결정, `docs/todos/n5_reading_expand_30.md`)에 따라 **Sonnet subagent 3배치가 생성**하고 **Opus(main)가 전수 검수**했다. haiku는 전문 かな 읽기의 숫자·특수 읽기 오류율 때문에 금지.

## 생성 (Sonnet subagent × 3배치)

배치 간 지문/prompt 충돌을 막기 위해 장르 풀을 겹치지 않게 분할하고 병렬 생성했다.

- **Batch A (0011~0020)**: 엽서·초대메일·전철 안내방송·병원 접수·레시피·펫 일기·심부름 메모·주말 일기·버스 승차장·우체국 안내
- **Batch B (0021~0030)**: 운동회·문화제·부활동 모집·시험 안내·소풍·급식·청소당번·전학생 자기소개·도서실 대출·자리바꾸기
- **Batch C (0031~0040)**: 알바 모집·쓰레기 배출·이사 인사·취미 소개·여름방학 편지·주말 날씨·빵집 영업·전화 전언·여행 계획·수영교실

## Opus 전수 검수 + 수정 4건

전 항목에 대해 (1) reading = passage 전문 ひらがな 전사(숫자 counter 읽기 중점), (2) 정답 유일성 + 오답 3개의 명백한 오류, (3) N5 문법/어휘 범위, (4) explanation 한국어 근거 인용, (5) integrity 제약을 검수했다. 숫자 읽기(くじ/よじ/しちじ/はつか/ふつか/にほん/みっか 등)는 모두 정확했다. 발견·수정 4건:

1. **0021 `reading`**: passage 「前で」를 「まえに」로 전사 → 전사 불일치 실오류. `まえで`로 수정.
2. **0031 `reading`**: passage 카타카나 「アルバイト」를 「あるばいと」로 히라가나화 → 기존 코퍼스(コーヒー/パン 카타카나 유지) 관례와 불일치. `アルバイト`로 복원.
3. **0020 key_vocabulary**: `ポスト` reading이 `ぽすと`(히라가나) → `ポスト`로 통일.
4. **0040 options**: 0023(サッカー部)과 options 배열·정답이 완전 동일 → 정답 패턴 중복 완화 위해 distractor 변경(`月曜日と金曜日`/`水曜日と土曜日`/`日曜日だけ`).

difficulty 분포(전체 40): 1=16, 2=19, 3=5 (목표 대략 10:15:5 근사).

## 변경

- `cmd/ja/catalog/data/n5_reading.json`: `n5_reading_0011`~`0040` 30개 추가(총 40개).
- `cmd/ja/catalog/datasets_test.go`: `TestN5Reading_Integrity` 기대 개수 10→40.
- `docs/adr/ADR_from_21_to_40.md`: ADR-036에 후속(2026-07-23) 확장 노트 추가.
- `STATUS.md`: in-progress 항목 제거 + 완료 행 추가. `docs/todos/n5_reading_expand_30.md` 삭제.

## 검증

- `jq` 병합 후 integrity 전수 스크립트 통과: id 0001~0040 연번, 지문/prompt 전역 중복 0, options 4개·중복 0·correct_answer ∈ options, HTML 특수문자 0, key_vocabulary 완전, difficulty 1~3.
- `go test ./cmd/ja/catalog` 및 `make test`(`go test ./...`) 전체 통과.
- `go run ./cmd/ja/seeder` 멱등 실행: `reading=40`.
- DB 확인: `questions(category='reading')` = 40, `material_id IS NULL` = 0, `materials(category='reading')` = 40, distinct material_id = 40.

## 미실행 / 잔여

- **restart-app 생략**: live app은 DB에서 문제를 읽으므로 seeder 반영만으로 서빙된다. tmux 세션 방해를 피해 재시작하지 않았다(embed는 seeder 경로에서만 필요).
- **Telegram 수동 검수**: 기존 10개 포함 40개 실제 노출 확인은 사용자 몫으로 잔여.

## 범위 밖 (후속 과제)

`reading_mid`/`information_retrieval` skill 추가(공식 분포 검증 선행), 50개 초과 확장, 지문당 다문항.
