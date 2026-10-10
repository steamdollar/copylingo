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

- [ ] Cloudflare Tunnel(cloudflared/trycloudflare) Korea-block 노출 대응 — 손글씨 Mini App ingress가 Cloudflare 의존이라 한국 재차단 시 통째 중단 위험(현시점 도달은 정상). **(Case A 선결: 자체 도메인+named tunnel vs 비-CF ingress vs accept+monitor 미결)** see [docs/todos/cloudflare_korea_tunnel_risk.md](docs/todos/cloudflare_korea_tunnel_risk.md)

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
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
| 2026-09-20 | JLPT N4 vocab_paraphrase(유의 표현) 부실 해설 전수 개편 (120문항 문장 해석·정답 근거·4개 선지 개별 어휘 뜻 보강 및 DB 반영) | - |
| 2026-09-19 | 정규 Quiz 현재 레벨 최소 80%·최근 Study 자료 분산 우선, 독해·청해 예약 및 추가 복습 현재 레벨 우선 (ADR-050) | - |
| 2026-09-19 | Study 복습 부족분을 현재 레벨 우선 신규 단어로 보충해 아침 20·저녁 24개 유지, 신규 문법·독해 상한과 SRS 간격 보존 (ADR-049) | - |
| 2026-09-19 | 동적 개인화 푸시(ADR-048) 활성화 시 레거시 정적 Study Cron 등록 원천 차단 및 config.yaml 정리 | - |
| 2026-09-19 | 세션별 객관식·청해 보기(Options) 결정론적 셔플링(`sessionID:questionID` Seed 기반 멱등성 보장 및 위치 암기 방지) | [2609192027_deterministic_question_option_shuffle.md](docs/workthrough/2609/2609192027_deterministic_question_option_shuffle.md) |
| 2026-09-18 | JLPT N4 vocab_usage(용법) 동어 반복 결함 수정 (120문항 4개 선지 전수 목표 단어 포함 및 DB 반영) | - |
| 2026-09-14 | 사용자별 30분 단위 개인화 푸시 스케줄링(Study/Quiz 4슬롯), 다중 타임존 partial index, Worker Pool+Rate Limiter, Redis 멱등성 락 및 텔레그램 설정 UI (ADR-048) | - |
| 2026-09-13 | Study 아침 20·저녁 24개 및 유형별 신규량 보장, 현재 레벨 신규 우선, due 보충·수동 limit 배분, cron 설정 정합성 (ADR-047) | - |
