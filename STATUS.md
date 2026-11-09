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

- [ ] 과잉 추상화·테스트 가치 검토 후속(E~I, A~D 완료): service Tier2 하위 interface·nil 분기 제거, 가치 낮은 테스트 삭제, scheduler·restart recovery 테스트 공백. **(D는 ADR-059 §8.7 수정, G는 ADR-057 재논의 선결)** see [docs/todos/abstraction_review_followups.md](docs/todos/abstraction_review_followups.md)

- [ ] 사용자 선택형 세션 문제 조합 preset — Daily Session 생성 전에 Vocabulary/Kana/Handwriting 비율 preset을 선택할 수 있도록 설계 및 구현. **(Case A 선결: preset 비율/변경 UX/SRS 충돌 우선순위/vocab fallback 미결)** see [docs/todos/user_selectable_session_mix_presets.md](docs/todos/user_selectable_session_mix_presets.md)

- [ ] 정규 Quiz 하위 레벨 due 복습 starvation 버그 — 현재 레벨 due가 세션 크기+2개를 넘으면 N5 due가 한 문제도 안 나옴(9/29 이후 0개, N5 due 512개 적체). 하위 레벨 전용 due 조회로 수정 see [docs/todos/quiz_lower_level_due_starvation.md](docs/todos/quiz_lower_level_due_starvation.md)

- [ ] scheduler 발송 복구 후보안: 끊기거나 놓친 slot 발송을 이어 보내는 방법. A(DB 상태 기록 + 재개 + admin, 무거움) / B(Redis claim 유지 + 실패 시 해제 + lookback, 경량) / C(현행 유지). **(Case A 선결: 안 채택 미결)** see [docs/todos/scheduler_dispatch_recovery.md](docs/todos/scheduler_dispatch_recovery.md)

- [ ] Cloudflare Tunnel(cloudflared/trycloudflare) Korea-block 노출 대응 — 손글씨 Mini App ingress가 Cloudflare 의존이라 한국 재차단 시 통째 중단 위험(현시점 도달은 정상). **(Case A 선결: 자체 도메인+named tunnel vs 비-CF ingress vs accept+monitor 미결)** see [docs/todos/cloudflare_korea_tunnel_risk.md](docs/todos/cloudflare_korea_tunnel_risk.md)

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
| 2026-10-27 | 청해 음성 생성기(`AudioGenerator`)를 `internal/service`에서 유일한 사용처인 `cmd/admin/generate_listening_audio`로 이동(unexported), 동작 변화 없음 (ADR-082 후속) | - |
| 2026-10-27 | 조립부가 채우는 의존의 nil 방어 분기 37곳(service·bot·scheduler·Mini App·청해 음성)과 `Err{Quiz,Study}ActiveSessionDependencyMissing` 삭제, 테스트는 not-found fake로 대체 (ADR-084, 검토 TODO D) | - |
| 2026-10-27 | session builder 테스트 정리: 8인자 closure fake 4종을 in-memory `builderPool`·`builderStores`로 합쳐 3파일 2,669줄 → 1,479줄, 시나리오 유지 (검토 TODO B 완료) | - |
| 2026-10-27 | builder·study session 생성자가 `SessionRepo`를 직접 받음: 좁힌 재선언 interface `sessionStore`·`studySessionStore`와 builder fake의 죽은 메서드 3개 삭제 (검토 TODO B 일부) | - |
| 2026-10-27 | `srsService` 삭제: SM-2는 순수 함수 `scheduleAnswer`, 복습 문제·개수는 builder와 `SessionService`가 `QuestionRepo`를 직접 조회. `srsScheduler`·`questionQuerier`·`questionFetcher` interface 삭제 (검토 TODO B 일부) | - |
| 2026-10-27 | 손글씨 PNG 렌더러를 service에서 miniapp handler로 이동: `StrokeRenderer` interface와 렌더러 구조체를 상수 기반 함수 `RenderStrokesPNG` 하나로 축소, service는 stroke 대신 렌더링된 이미지만 받음. 렌더링이 소유권 검사보다 앞으로 옴(수 ms, LLM 호출은 여전히 검사 후) (검토 TODO B 일부) | - |
| 2026-10-27 | grader 계층 삭제: `graderService`와 그 interface를 지우고, 채점은 제출 메서드에서 하며 `RecordAnswer`는 `SessionService`가 직접 호출 (검토 TODO C, U4 grader 항목) | - |
| 2026-10-27 | LLM API key 필수화: key가 없으면 서버가 시작하지 않음. 동작하지 않던 key guard·`ErrAIConfigMissing`/`ErrAIUnavailable` 체인과, 문항이 0건이던 subjective 타입(LLM 의미 채점·오답 fallback·AI 피드백)을 삭제 (ADR-083, 검토 TODO A) | - |
| 2026-10-26 | 청해 음성 생성(TTS)을 서버에서 제거: scheduler의 30분 음성 top-up을 지우고 `cmd/admin/generate_listening_audio`(이제 `-level` 없이 언어 전체 레벨 처리)만 TTS를 호출. `AudioService`는 클립 읽기·file_id 저장만 남기고 생성은 `AudioGenerator`로 분리. 서버 음성 전송이 LLM key와 무관해짐. 새 청해 seed 후 도구 실행 필요(README). 음성을 새로 만들 때 Telegram `audio_file_id`도 지워, 대본이 바뀐 문항에 옛 음성이 재전송되던 버그 수정 (ADR-082) | - |
| 2026-10-19 | 한자 해설을 단어 읽기 기준으로 변경: 단어별 한자 읽기 `kanji_readings`(음독·훈독·熟字訓 + 그 한자의 일반 음독)를 어휘 2,961개에 자동 생성(`scripts/gen_kanji_readings.py`, JmdictFurigana·KANJIDIC2·JMdict, `NOTICE`에 출처), 해설 화면에 `이 단어에선`·`이 한자의 음독` 줄 추가와 note 없는 한자도 표시, `같은 발음 요소`·`계열로 배운 단어` 줄과 관련 service·repository 삭제 (ADR-074) | [2610192358](docs/workthrough/2610/2610192358_kanji_readings.md) |
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
| 2026-10-07 | JLPT N5/N4 어휘 보충: Waller(구 출제기준 재구성) 리스트 기준으로 N5 249개, N4 365개(+seed 365개, 작성 문항 opus 검수) 추가, question seed upsert 파라미터 한도 버그 수정 | [workthrough](docs/workthrough/2610/2610072307_jlpt_vocab_supplement.md) |
| 2026-10-05 | 죽은 코드 삭제: 호출자 없는 repository 메서드 10개·읽지 않는 Redis 키·미사용/중복 interface·grader 테스트 전용 메서드, scheduler 콘텐츠 수집 연결 제거(pipeline 코드는 보존, ADR-057 보강) | - |
| 2026-10-05 | Quiz 답안 제출(선택지·어순·주관식)에 세션 소유자 검사 추가, callback 정수 파싱 실패를 0번 선택지로 처리하던 문제 수정 | [2610051431_quiz_answer_owner_check.md](docs/workthrough/2610/2610051431_quiz_answer_owner_check.md) |
| 2026-10-05 | `go list` 기반 import 경계 테스트로 internal 패키지 간 import와 드라이버 소유를 고정 — 아키텍처 단순화 2·3단계 완료 (ADR-059 §8 E) | - |
| 2026-10-05 | `Services` 묶음 삭제, 외부 클라이언트·서비스 생성을 cmd/server로 이동, `app` Run/Close 생명주기 (ADR-059 §8 D) | - |
| 2026-10-04 | 기능별 Flow 분리, `*Bot` 역참조 제거, scheduler·Mini App에 Flow의 좁은 계약 주입 (ADR-059 §8 C) | - |
