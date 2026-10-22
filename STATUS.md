# CopyLingo 현재 상태

> 에이전트는 새 세션 시작 시 이 파일을 읽고 작업을 시작합니다.

---

## 🔨 진행 중

- (없음)

---

## ⏭️ 다음

- (없음)

---

## 🚧 블로커

- (없음)

---

## TODO

> 각 항목은 `docs/todos/<file>.md`에 자기완결적 문서로 분리되어 있다. 작성/실행/완료 처리 규칙은 `AGENTS.md` §3 Case C 참조.

- [ ] 구조 검토 후속: service·bot 파일 prefix rename·테스트 재배치(U3) → 책임 이동(U4, SRS 단일화 등 일부 Case A 선결) + callback 이중 응답(실기기 확인 대기). 하위 패키지 분리는 하지 않음 see [docs/todos/structure_review_followups.md](docs/todos/structure_review_followups.md)

- [ ] 사용자 선택형 세션 문제 조합 preset — Daily Session 생성 전에 Vocabulary/Kana/Handwriting 비율 preset을 선택할 수 있도록 설계 및 구현. **(Case A 선결: preset 비율/변경 UX/SRS 충돌 우선순위/vocab fallback 미결)** see [docs/todos/user_selectable_session_mix_presets.md](docs/todos/user_selectable_session_mix_presets.md)

- [ ] 정규 Quiz 하위 레벨 due 복습 starvation 버그 — 현재 레벨 due가 세션 크기+2개를 넘으면 N5 due가 한 문제도 안 나옴(9/29 이후 0개, N5 due 512개 적체). 하위 레벨 전용 due 조회로 수정 see [docs/todos/quiz_lower_level_due_starvation.md](docs/todos/quiz_lower_level_due_starvation.md)

- [ ] Cloudflare Tunnel(cloudflared/trycloudflare) Korea-block 노출 대응 — 손글씨 Mini App ingress가 Cloudflare 의존이라 한국 재차단 시 통째 중단 위험(현시점 도달은 정상). **(Case A 선결: 자체 도메인+named tunnel vs 비-CF ingress vs accept+monitor 미결)** see [docs/todos/cloudflare_korea_tunnel_risk.md](docs/todos/cloudflare_korea_tunnel_risk.md)

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
| 2026-10-22 | N3 청해 20문항 추가: 새 자료를 만들지 않고 기존 N3 문법 material 20개(학습한 16 + 다음 4)에 연결해 대본이 그 문법을 쓰게 함. N3 난이도(5~7턴 대화·200자 이상 독백, 음성 속 오답, 계획 변경 함정)로 작성. scheduler 음성 top-up을 현재 레벨에서 Study·Quiz 범위 전체(N4 사용자는 N5·N4·N3)로 넓혀, 인접 레벨 청해가 음성 없이 영영 출제되지 않던 문제 해결. seed DB 반영 (ADR-079) | - |
| 2026-10-21 | 저녁 Quiz 12 → 17문항(복습 6)으로 아침과 통일. Quiz 신규가 Study를 못 따라가던 문제(최근 14일 학습 N4 단어 중 52%만 Quiz 확인) 완화, 하루 Quiz 신규 약 17 → 22 (ADR-078) | - |
| 2026-10-21 | 가나(히라가나·가타카나) material 208·문항 622을 N5에서 새 레벨 `NZ`(`data/ja/nz.json`)로 분리. 레벨 순서 `NZ→N5→…→N1`, N4 이상 범위에서 제외. DB 행 level만 NZ로 갱신해 N4 복습 큐의 가나 due 193건 제외 (ADR-077) | - |
| 2026-10-21 | 한자 해설 제목에 한국 훈음 추가: `伸 펼 신 · 형성문자`, note 없는 한자도 `事 일 사`. 어휘 한자 1,421자의 훈음을 `scripts/kanji_hun_eum.tsv`에 두고 `gen_kanji_readings.py`가 `kanji_readings[].hun_eum`으로 복사해 재seed. opus 초안을 libhangul·KANJIDIC2와 자동 대조(1,114/1,432 사전 일치)하고 전수 검수, 일본어 뜻과 다른 39자는 `(일본어: …)` 보충 (ADR-076) | - |
| 2026-10-21 | N4 vocab_usage 오답 반복 틀 제거: 「日記に今日の出来事を〇〇」「自分の意見を〇〇」「冗談を〇〇して、みんなを笑わせます」「朝ごはんを〇〇」 등 같은 문장 틀에 단어만 바꿔 넣은 오답이 있던 48문항의 오답 3개를 상황이 서로 다른 오용 문장으로 다시 쓰고 해설의 오답 부분도 새로 작성(정답·key 유지, reviewer 검수로 복수 정답 의심 9개 교체)한 뒤 DB 반영 | - |
| 2026-10-19 | 한자 해설을 단어 읽기 기준으로 변경: 단어별 한자 읽기 `kanji_readings`(음독·훈독·熟字訓 + 그 한자의 일반 음독)를 어휘 2,961개에 자동 생성(`scripts/gen_kanji_readings.py`, JmdictFurigana·KANJIDIC2·JMdict, `NOTICE`에 출처), 해설 화면에 `이 단어에선`·`이 한자의 음독` 줄 추가와 note 없는 한자도 표시, `같은 발음 요소`·`계열로 배운 단어` 줄과 관련 service·repository 삭제 (ADR-074) | [2610192358](docs/workthrough/2610/2610192358_kanji_readings.md) |
| 2026-10-19 | 복수 정답 2차: 전수 검사가 안 됐던 선택형(vocab_context·vocab_usage·vocab_paraphrase·청해·N5 문법 의미형)과 남은 문법 후보를 검사해 84문항 수정(문법 19, vocab_context 32, vocab_usage 32는 오답 3개 재작성, 청해 1)하고 해설도 새 문항에 맞춰 다시 쓴 뒤 DB 반영. 어순 배열 38개·한자 읽기 입력형 약 13개는 처리 방식 결정 대기 | - |
| 2026-10-19 | N3·N4·N5 seed의 일본어 문장 1,948개를 비문 검수해 비문·부자연 17개, 정답 오류 6개(독해 정답 key가 본문과 모순·정답 선택지 없음 5, paraphrase 오답 정답 1)를 고치고 DB 반영. 복수 정답 의심 문항은 미처리 | - |
| 2026-10-19 | 빈약했던 N4·N5 문장형 Quiz 해설 704개(vocab_context·grammar_form·grammar_text·sentence_composition)를 `문장`(한자 뒤 괄호 히라가나 읽기)·`해석`·`해설`·`오답`(선택지별 읽기·뜻·이유) 형식으로 다시 쓰고 DB 반영. 배열형은 `정답`·`순서`·`해설` 형식. N3·청해·독해·단어 단독 문항은 그대로 | - |
| 2026-10-19 | 복수 정답이던 seed 문항 7개 수정(N4 문법 4·N5 문법 2·N4 覚ます 용법 1): 문맥을 추가하거나 정답이 될 수 있는 선택지를 교체하고 DB에 반영, 잘못 채점돼 깎인 SRS ease를 2.5로 복구(5행). 정답 키가 틀린 N5 `読ん__します` 문항(たり→だり)도 수정. 이어서 문법 빈칸 476문항을 전수 검사해 복수 정답 93개(N5 25·N4 68, N3 0)를 오답 선택지 교체·문맥 추가로 수정하고 DB 반영 | - |
| 2026-10-19 | Study 단어 카드에 `🈁 한자 해설` 버튼 추가: vocabulary payload의 `kanji_notes`(한자 900자 note, 어휘 2,066개)를 누를 때만 보여 주고, 이미 학습한 단어 중 같은 발음 요소를 쓰는 단어를 함께 표시. 카드 본문·Quiz·SRS·세션 생성은 그대로 (ADR-073) | [2610190043](docs/workthrough/2610/2610190043_kanji_notes.md) |
| 2026-10-13 | JLPT N3 seed 추가: Waller 리스트 기준 단어 1,766개(한자어 읽기·표기 문항은 스크립트, 가나 단어 문항은 sonnet 작성·opus 전수 검수)와 문법 98개(문항 3종), N3 단어는 흔한 단어부터 나오도록 `difficulty`에 빈도 반영 | [2610132130](docs/workthrough/2610/2610132130_jlpt_n3_seed.md) |
| 2026-10-13 | 통계를 `정답률 \| 진도` 탭으로 나누고, 진도 탭에 레벨별로 Study에서 한 번이라도 학습한 자료 비율 표시 (ADR-072) | - |
| 2026-10-11 | 진행 중 Quiz에서 대기 중이 아닌 텍스트를 받으면 직전 문제 정오답 안내 + 다음 미답 문제 재전송(결과 메시지 유실 시 dead end 해소, ADR-071) | [2610111020](docs/workthrough/2610/2610111020_telegram_429_silent_quiz_stall.md) |
| 2026-10-11 | Telegram Bot API 호출 실패를 `TelegramClient`에서 WARN `telegram.api.failed`(method·chat_id·error_code·retry_after_sec)로 log, 대기 중인 텍스트 입력 없이 무시된 메시지를 INFO `telegram.message.ignored`로 log (ADR-070) | [2610111020](docs/workthrough/2610/2610111020_telegram_429_silent_quiz_stall.md) |
| 2026-10-10 | seeder 언어를 필수 위치 인자로 변경(`go run ./cmd/seeding ja`, 인자 개수가 틀리거나 없는 언어면 사용법과 언어 목록을 보여주고 exit 2), `records.go`·`records_test.go`를 `main.go`·`main_test.go`로 합침, seeder 함수 10개를 6개로 합치고 일어날 수 없는 guard(row ID 0, embed 읽기 에러)와 level별 검사 제거, 쓰지 않는 `cmd/admin/build_study_sessions` 삭제(scheduler `BuildForSlot`·bot 즉시 study가 대체), `reset_learning_data`를 TRUNCATE 상수 하나와 `main`으로 합침 | - |
| 2026-10-10 | Quiz 문제 헤더에 문제별 레벨 표시(`📝 문제 3/17 · N4 🔄`), level 없는 문제는 생략 | - |
| 2026-10-10 | `cmd/` 5곳에 복사된 `initDB`와 server·admin의 audio 조립을 `internal/bootstrap`(`OpenDB`·`NewAudioService`)으로 모음, seeder 로더를 함수로 바꾸고 테스트와 겹치는 런타임 검사 제거, question upsert 실패 시 exit 0 버그 수정 (ADR-069, ADR-068 보강) | - |
| 2026-10-08 | seed `catalog`·`data` 패키지를 `cmd/seeding`에 합치고, seed JSON을 `model.Material`·`model.Question`으로 바로 읽도록 record 타입 정리, 언어·level은 파일 경로에서 결정, `UpsertBatch`가 row ID를 돌려줘 ID 재조회 제거 (ADR-068, seed 출력 동일) | - |
| 2026-10-08 | N5 seed 데이터를 record로 전환해 seeder를 경로 하나로 통일하고 seed 때 문항 생성을 제거, level당 JSON 하나(`data/ja/n4.json`, `n5.json`)로 정리, 앞으로 문항 유형은 JLPT 체계를 따름 (ADR-067, seed 출력 동일) | - |
| 2026-10-08 | N4 seed 데이터를 DB row 모양의 공통 record로 통일(문항을 material 안에 넣은 `n4/records.json` 하나)하고 문항 유형과 무관한 seeding 경로 추가, 기존 question_key 보존 (ADR-066) | - |
| 2026-10-08 | Seed 도구를 `cmd/ja` → `cmd/seeding`으로 이동, JSON을 `data/<언어>/`로 나누고 registry·`-language` 플래그에 언어 차원 추가 (ADR-065 B1, seed 출력 동일) | - |
| 2026-10-08 | Study 플랜 신규 어휘 중심 재조정(ADR-064): 하루 신규 단어 12→24, 재학습·독해 재독 축소, `/study n`이 아침 플랜 비율을 따름 | - |
| 2026-10-07 | JLPT N5/N4 어휘 보충: Waller(구 출제기준 재구성) 리스트 기준으로 N5 249개, N4 365개(+seed 365개, 작성 문항 opus 검수) 추가, question seed upsert 파라미터 한도 버그 수정 | [workthrough](docs/workthrough/2610/2610072307_jlpt_vocab_supplement.md) |
| 2026-10-05 | 죽은 코드 삭제: 호출자 없는 repository 메서드 10개·읽지 않는 Redis 키·미사용/중복 interface·grader 테스트 전용 메서드, scheduler 콘텐츠 수집 연결 제거(pipeline 코드는 보존, ADR-057 보강) | - |
| 2026-10-05 | Quiz 답안 제출(선택지·어순·주관식)에 세션 소유자 검사 추가, callback 정수 파싱 실패를 0번 선택지로 처리하던 문제 수정 | [2610051431_quiz_answer_owner_check.md](docs/workthrough/2610/2610051431_quiz_answer_owner_check.md) |
| 2026-10-05 | `go list` 기반 import 경계 테스트로 internal 패키지 간 import와 드라이버 소유를 고정 — 아키텍처 단순화 2·3단계 완료 (ADR-059 §8 E) | - |
| 2026-10-05 | `Services` 묶음 삭제, 외부 클라이언트·서비스 생성을 cmd/server로 이동, `app` Run/Close 생명주기 (ADR-059 §8 D) | - |
| 2026-10-04 | 기능별 Flow 분리, `*Bot` 역참조 제거, scheduler·Mini App에 Flow의 좁은 계약 주입 (ADR-059 §8 C) | - |
| 2026-09-30 | Quiz·Study를 `SessionService` Tier1로 통합하고 Tier2를 unexported로 전환 (ADR-059 §8 B) | - |
| 2026-09-30 | Bot의 Telegram API 호출을 `telegramClient`로 분리, Flow가 전송 시 `*Bot`을 거치지 않도록 정리 (ADR-059 §8 A) | - |
| 2026-09-27 | Quiz·Study Redis 진행 상태의 `workingSetStore` 포장 제거, 저장소 직접 호출 및 손상 상태 검사 통합 (ADR-062) | - |
| 2026-09-27 | Study 세션 생성의 트랜잭션 경계를 서비스 `WithinTx`로 이동하고 세션·자료 INSERT 분리 (ADR-061) | - |
| 2026-09-27 | Study 세션·자료 연결 INSERT를 단일 PostgreSQL 트랜잭션으로 묶어 실패 시 세션 행 롤백 | - |
| 2026-09-27 | Study 세션 자료 연결 저장을 SessionRepository로 통합하고 분리 저장소·주입 제거 | - |
| 2026-09-26 | 아키텍처 단순화 설계와 1단계 완료: Redis 저장 구현 분리·기능별 계약 주입·기존 데이터 호환 유지 (ADR-059·060, 9/27 파일·저장 타입·생성자 정리와 Quiz/Study 이름 구분, 2·3단계 미착수) | - |
| 2026-09-26 | 서버 시작·콘텐츠 저장의 서비스 경계 정리 및 서버 파일 분리, 미사용 파이프라인 초기화 제거·자동 수집 비활성 유지 (ADR-057·058) | - |
| 2026-09-26 | 신규 청해 대화 음성에 A/B 두 화자 적용, 기존 음성 유지 (ADR-056) | - |
| 2026-09-26 | 미사용 설정·환경변수 및 설정 파일 정리, TTS 활성화 설정 제거·LLM 설정 통합 | - |
| 2026-09-26 | 사용자별 30분 cron만 유지, 전역 발송·자동 수집 cron과 상한 설정 제거, 미완료 세션 3개 재알림 규칙 유지 (ADR-052·053) | - |
| 2026-09-25 | Study·Quiz 자료 유지 복습·제외 메뉴와 목록/복원, 공통 기한·자료별 유지 문제 제한·결과별 간격 전환, 소유권 검증과 기존 세션 보존 (ADR-051) | - |
