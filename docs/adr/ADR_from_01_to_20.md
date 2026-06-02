# CopyLingo 의사결정 기록 (ADR)

> Architecture Decision Records — 프로젝트에서 내린 주요 기술적 의사결정을 기록합니다.

---

## ADR-001: 텔레그램 봇을 프론트엔드로 사용

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 개인 학습용 앱이므로 별도 모바일/웹 프론트를 만드는 건 과잉. 텔레그램은 이미 사용 중인 메신저이고, Bot API가 잘 정비되어 있음.
- **결정**: Telegram Bot API (Inline Keyboard 기반) 사용
- **장점**: 개발 비용 최소, 알림 자연스러움, 크로스플랫폼
- **단점**: UI 자유도 제한, 복잡한 인터랙션 어려움
- **대안**: PWA, Flutter — 개인 사용 대비 과잉 판단

---

## ADR-002: PostgreSQL + Redis 조합

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 개인 사용이라 SQLite도 가능하나, SRS 스케줄 쿼리(`next_review_at <= NOW()` + JOIN)와 카테고리별 정답률 집계 등 분석 쿼리에 PostgreSQL이 유리.
- **결정**: PostgreSQL (메인 DB) + Redis (세션 캐시, 응답 시간 측정)
- **장점**: 강력한 쿼리 성능, JSONB 지원, 확장성
- **단점**: SQLite 대비 인프라 비용
- **대안**: SQLite — 분석 쿼리 한계, 동시성 제한

---

## ADR-003: SM-2 간격 반복 알고리즘

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 복습 시스템 요구. Anki에서 검증된 SM-2가 구현 난이도와 효과 균형 좋음.
- **결정**: SM-2 알고리즘 직접 구현
- **장점**: 간단, 검증됨, 커스터마이징 용이
- **대안**: SM-5, FSRS — 더 복잡하지만 필요 시 업그레이드 가능

---

## ADR-004: AI 모델 선정 (문제 생성)

- **날짜**: 2026-03-11 (최종 수정: 2026-03-22)
- **상태**: 수정됨
- **맥락**: 수집된 뉴스/시험자료에서 JLPT 수준별 문제를 자동 생성해야 함. 수동 제작은 비현실적.
- **결정 (v1)**: GPT-4o-mini → 월 $2~5 예상
- **결정 (v2, 현재)**: **Gemini 3.0 Flash** → 월 무료 (1,500 RPD 내 운용)
- **변경 사유**: Gemini 무료 티어로 비용 0 달성, OpenAI 호환 엔드포인트 제공으로 코드 변경 최소화
- **장점**: 비용 0, 다양한 문제 유형 생성, 한국어/일본어 성능 우수
- **단점**: API 의존성, RPD 제한 (1,500/일)
- **대안**: GPT-4o-mini (유료), 로컬 LLM (품질 트레이드오프)

---

## ADR-005: TTS 사전 생성 + 캐싱

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 일본어 발음 학습에 TTS 필요하나, 실시간 API 호출 시 0.5~2초 지연으로 UX 저하.
- **결정**: 문제 생성 파이프라인에서 TTS 사전 생성 → 파일 캐싱 → Telegram voice message 전송
- **장점**: 지연 0초, 동일 문제 재출제 시 API 재호출 불필요
- **단점**: 디스크 사용량 증가 (관리 가능 수준)

---

## ADR-006: 하트(생명) 시스템 제외

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 듀오링고의 하트 시스템은 무료 유저 제한 + 과금 유도 목적. 개인 학습에 동기부여보다 학습 흐름 방해.
- **결정**: 하트 시스템 제외, 무제한 풀기
- **장점**: 학습 흐름 유지, 부담 없는 반복
- **대안**: 유지 — 개인 사용에 맞지 않다고 판단

---

## ADR-007: 세션 푸시 방식 (Pull vs Push)

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 사용자가 매번 수동으로 학습을 시작하는 것보다, 정해진 시간에 자동 전송되는 것이 학습 습관 형성에 유리.
- **결정**: Push 방식 (크론 스케줄러 → 텔레그램 메시지 전송) + 수동 Pull도 가능 (/menu → 학습하기)
- **장점**: 학습 습관 강제, 알림 효과
- **단점**: 바쁠 때 귀찮을 수 있음 → 설정에서 시간 조정 가능

---

## ADR-008: 콘텐츠 비율 4:6 (뉴스:시험대비)

- **날짜**: 2026-03-11
- **상태**: 채택됨
- **맥락**: 실용적 일본어 + 시험 합격이라는 이중 목표. 사용자가 뉴스 40%, 시험 대비 60% 비율 제안.
- **결정**: 수집 및 문제 생성 시 뉴스 40%, 시험 대비 60% 비율 유지
- **조정**: 레벨별로 비율 조정 가능 (초급은 시험대비 비중 높게, 고급은 뉴스 비중 높게)

---

## ADR-009: 다국어 지원 스키마

- **날짜**: 2026-03-22
- **상태**: 채택됨
- **맥락**: 초기 설계는 일본어(JLPT) 전용이었으나, 그리스어 등 추가 언어 학습 가능성 대비 확장성 필요.
- **결정**: `jlpt_level` → `language` + `proficiency_level` 2개 필드로 분리
  - `language`: ISO 639-1 코드 ('ja', 'el', 'en' 등)
  - `proficiency_level`: 언어별 레벨 체계 (JLPT: N5-N1, CEFR: A1-C2)
- **영향 범위**: users, contents, questions 테이블 및 관련 model/repository/service 전체
- **다중 언어 학습**: 현재는 사용자당 1언어. 여러 언어 학습 시 user 레코드 별도 생성 (추후 필요 시 복합키 확장 가능)
- **장점**: 추가 언어 학습 시 스키마 변경 없이 확장
- **단점**: 기존 코드 대비 약간의 복잡도 증가
- **대안**: 언어별 별도 테이블 — 코드 중복, 유지보수 부담

---

## ADR-010: 손글씨 가나 문항은 Telegram Mini App + Binary Grading으로 구현

- **날짜**: 2026-04-23
- **상태**: 채택됨
- **맥락**: 히라가나/가타카나 학습에서 사용자가 모바일 화면에 직접 글자를 써 보는 연습이 필요하다. 그러나 Telegram Bot 채팅 UI 자체는 손글씨 입력 컴포넌트를 제공하지 않고, 일반 메시지로는 `text`, `photo`, `document` 등만 받을 수 있다. 종이에 쓰고 사진을 찍어 올리는 방식은 UX가 나쁘고 반복 학습에 부적합하다.
- **결정**:
  - 손글씨 입력 UI는 **Telegram Mini App**으로 제공한다.
  - Bot은 세션 오케스트레이션을 유지하고, 손글씨 문항에서만 `web_app` 버튼으로 Mini App을 연다.
  - Mini App은 완성된 원본 이미지를 매번 업로드하는 대신, 우선 **stroke data**(좌표, pen down/up, 시간 정보)를 서버로 전송한다.
  - 서버는 stroke data를 정규화한 뒤 필요한 경우에만 소형 raster 이미지로 렌더링한다.
  - 채점은 일반 OCR이 아니라, 이미 정답을 알고 있는 문항 특성을 활용한 **Binary Grading**으로 처리한다.
  - 즉, 모델에게 "`이 손글씨가 정답 문자 X를 충분히 올바르게 쓴 것으로 볼 수 있는가?`"를 판단하게 한다.
  - **Gemini multimodal** 호출은 기본 경로로 사용하되, 프롬프트는 자유 해석형 OCR이 아닌 정답 검증형으로 제한한다.
  - 추후 비용 최적화를 위해 heuristic/local check를 1차로 두고, 확신이 낮을 때만 Gemini를 호출하는 fallback 구조로 확장한다.
- **장점**:
  - Telegram 내부에서 자연스러운 손글씨 UX 제공
  - 채팅 입력창 제약을 우회하면서도 별도 앱 개발 없이 구현 가능
  - stroke-first 전송으로 payload와 재채점 비용 절감
  - OCR 문제를 open-ended recognition이 아닌 verification problem으로 축소하여 정확도와 비용 효율 개선
  - 기존 Bot 세션 구조를 유지하므로 회귀 범위 축소
- **단점**:
  - Bot 단독 구조 대비 Mini App 프론트엔드 및 서버 검증 로직이 추가됨
  - Bot 상태와 Mini App 상태 간 동기화가 필요함
  - 초기 버전에서는 Gemini multimodal 의존성이 남아 있음
- **대안**:
  - 종이 또는 외부 앱에 쓴 뒤 사진 업로드: 구현은 단순하지만 UX가 좋지 않아 기각
  - 세션 전체를 Mini App으로 이전: UX 일관성은 좋지만 현재 구조 대비 변경 범위가 과도하여 기각
  - 원본 PNG를 매 시도마다 Gemini에 직접 전달: 초기 구현은 쉬우나 네트워크/비용 측면에서 비효율적이라 기각

---

## ADR-011: 손글씨 Mini App 제출은 HTTPS 공개 ingress + 서버 측 검증으로 처리

- **날짜**: 2026-05-07
- **상태**: 채택됨
- **맥락**:
  - 손글씨 문항은 Telegram Mini App 내부 canvas에서 입력된다.
  - 휴대폰 Telegram 앱에서 열린 Mini App은 개발 머신의 `localhost:8080`에 접근할 수 없다.
  - Telegram Mini App은 실사용 경로에서 HTTPS 공개 URL이 필요하며, Bot이 생성하는 `web_app` URL의 host는 BotFather에 등록된 Mini App/Web App 도메인과 일치해야 한다.
  - 서버는 이미 Gin HTTP 서버를 `:8080`에서 실행하고, 손글씨 제출 API를 `POST /api/miniapp/handwriting/submit`으로 제공한다.
- **결정**:
  - 손글씨 제출은 Bot Callback Data가 아니라 Mini App의 HTTP `POST`로 받는다.
  - Mini App은 원본 이미지 파일을 직접 업로드하지 않고, `init_data`, `session_id`, `question_id`, `strokes`를 JSON으로 전송한다.
  - 서버는 Telegram `init_data`를 검증한 뒤, 세션 소유자와 제출 사용자가 일치하는지 확인한다.
  - 서버는 제출된 `question_id`가 해당 세션에 포함되어 있고, 문항 타입이 `kana_handwriting`이며, 아직 답변되지 않았는지 확인한다.
  - 서버는 stroke data를 PNG로 렌더링한 뒤, 정답이 이미 알려진 문항이라는 전제를 활용해 LLM multimodal 채점에 전달한다.
  - 로컬/개발 환경의 공개 HTTPS ingress는 Cloudflare Tunnel(`cloudflared tunnel --url http://localhost:8080`)을 우선 사용한다.
  - 운영 환경에서는 Cloudflare Tunnel 임시 URL보다 고정 도메인 + HTTPS reverse proxy 또는 named tunnel 구성을 사용한다.
- **장점**:
  - Telegram 채팅 UI의 입력 한계를 우회하면서도 Bot 세션 흐름을 유지한다.
  - raw image upload보다 stroke-first payload가 작고, 서버에서 렌더링 크기를 통제할 수 있다.
  - Telegram `init_data` 검증과 세션 소유권 검증을 서버에서 수행하므로, Mini App 클라이언트를 신뢰하지 않아도 된다.
  - 개발 단계에서 OS firewall/NAT/router 포트를 직접 열지 않고 HTTPS 공개 URL을 만들 수 있다.
  - HTTPS endpoint가 고정되면 BotFather 도메인 검증, Mini App URL 생성, submit API 호출 경로가 단순해진다.
- **단점**:
  - Mini App, public ingress, BotFather 도메인 설정이라는 운영 요소가 추가된다.
  - Cloudflare Tunnel 임시 URL은 재실행 시 바뀔 수 있어 `COPYLINGO_SERVER_PUBLIC_BASE_URL`과 BotFather 설정을 다시 맞춰야 한다.
  - tunnel을 켜 둔 동안에는 로컬 `:8080` HTTP surface가 인터넷에서 접근 가능해진다.
  - 현재 Docker Compose는 PostgreSQL/Redis도 host port로 publish하고 있으므로, public 서버 배포 전 별도 hardening이 필요하다.
- **대안**:
  - OS/router에서 `8080`을 직접 공개: HTTPS, 인증서, NAT, firewall 관리 부담이 커서 개발용으로 기각
  - Telegram 채팅에 사진 업로드: 구현은 단순하지만 반복 학습 UX가 나빠 기각
  - 모든 학습 세션을 Web App으로 이전: 현재 Bot 중심 구조 대비 변경 범위가 커서 기각
  - 서버 없이 클라이언트에서 채점: 정답/채점 기준 노출 및 조작 가능성이 커서 기각

---

## ADR-012: Bot 세션 메시지 렌더링은 nullable editMessageID로 분기

- **날짜**: 2026-05-08
- **상태**: 채택됨
- **맥락**:
  - `SessionFlow.showQuestion`은 기존 Telegram 메시지를 수정할지, 새 메시지를 보낼지 결정해야 한다.
  - 기존 구현은 `messageID int`에 실제 Telegram 메시지 ID와 `0` sentinel을 함께 담았다.
  - `messageID > 0`은 기존 메시지 edit, `messageID == 0`은 새 메시지 send라는 암묵 규약이었으나, `0`이 실제 엔티티 ID처럼 읽혀 흐름 이해가 어려웠다.
  - 손글씨 Mini App 문항은 Web App 버튼이 붙은 메시지를 별도로 남기고, 제출 후 다음 문제는 새 Telegram 메시지로 보내야 한다.
- **결정**:
  - 세션 플로우에서 메시지 렌더링 분기는 `editMessageID *int`로 표현한다.
  - `editMessageID != nil`이면 해당 Telegram 메시지를 수정한다.
  - `editMessageID == nil`이면 편집할 봇 메시지가 없거나 새 메시지 UX가 필요한 것으로 보고 새 Telegram 메시지를 보낸다.
  - 손글씨 Mini App의 "제출 후 다음 문제" 흐름은 기존 메시지의 버튼만 제거하고, 다음 문제를 새 메시지로 렌더링한다.
  - 객관식 callback처럼 버튼이 붙은 봇 메시지가 명확한 경우에는 `editMessageID`를 전달해 기존 메시지를 피드백으로 수정한다.
- **장점**:
  - `0` sentinel의 이중 의미를 제거해 코드 독해성이 좋아진다.
  - Telegram 메시지 ID와 렌더링 모드가 더 명확히 구분된다.
  - 손글씨 Mini App 왕복 흐름에서 메시지 히스토리를 보존하는 의도가 코드에 드러난다.
- **단점**:
  - 호출부에서 로컬 변수 주소를 넘기는 작은 보일러플레이트가 생긴다.
  - `nil` 의미를 이해해야 하므로 함수 시그니처와 주석을 함께 유지해야 한다.
- **대안**:
  - `messageID int` + `0` sentinel 유지: 구현은 단순하지만 의미가 불명확해 기각
  - `QuestionRenderMode` enum 추가: 가장 명시적이지만 현재 분기 규모에는 과한 구조라 보류
  - `sendNew bool` 인자 추가: bool과 message ID 조합이 불일치할 수 있어 기각

---

## ADR-013: 활성 세션 상태는 Redis 작업영역 + 세션 종료 시 DB 일괄 flush

- **날짜**: 2026-05-09
- **상태**: 채택됨 (구현 미진행 — Phase 분리 예정)
- **맥락**:
  - 현재 답안 처리 hot path는 문제 1개당 DB UPDATE 4개를 동기적으로 실행한다:
    1. `session_questions.user_answer, is_correct`
    2. `questions.times_served +1` (`IncrementTimesServed`)
    3. `questions.times_correct +1` (`IncrementTimesCorrect`)
    4. `questions.{ease_factor, interval_days, repetitions, next_review_at, ...}` (`UpdateSRS`)
  - 2~4번은 동일 row(`questions` PK)를 같은 트랜잭션 시점에 3번 때리는 구조로, 규모와 무관하게 잘못 짜인 부분이다.
  - 또한 세션 진행 중 `GetBySession()`이 매 답안 처리마다 DB를 read하여 mid-session 상태를 재조회하고 있다.
  - 본 프로젝트의 설계 평가 기준은 "수만~수십만 사용자 가정"이며 (CLAUDE.md/AGENTS.md의 "프로젝트 성격 및 설계 기준" 참조), 그 규모에서 위 플로우는 hot path latency, row-level 락 경합, 통계/도메인 데이터의 일관성 요구 분리 부재 등 명확한 약점이 있다.
- **결정**:
  - **활성 세션 상태(current question idx, 누적 답안, mid-session 진행 정보)는 Redis에 working state로 유지**한다.
  - 답안 hot path에서는 **DB write를 발생시키지 않는다.** Redis만 갱신하고 사용자에게 즉시 응답한다.
  - mid-session read(`GetBySession` 등)도 Redis hit으로 흡수한다.
  - **세션 종료 시점(`finishSession`)에 단일 DB 트랜잭션으로 일괄 flush**한다:
    - `sessions` UPDATE (status, completed_at, correct_count)
    - `session_questions` bulk UPDATE (1세션 분량, 오전 15 / 오후 10)
    - `questions` bulk UPDATE (SRS 필드 + times_served/times_correct 카운터를 **하나의 UPDATE로 합쳐서**)
  - flush 트랜잭션은 retry. 영구 실패 시 해당 세션은 손실 처리하고, 사용자가 다시 풀게 한다.
  - 봇 재시작 시 Redis가 살아있으면 세션 이어 진행, Redis도 날아갔으면 DB의 `in_progress` 세션을 abandoned 처리하고 사용자에게 재시작 옵션 제공.
- **prerequisite (선행 정리)**:
  - `IncrementTimesServed` / `IncrementTimesCorrect` / `UpdateSRS` 3개 메서드를 **단일 `RecordAnswer(questionID, isCorrect, srs SRSResult)` 메서드로 합쳐 1 UPDATE**로 줄인다. 이건 본 ADR과 독립적으로 즉시 정당화되는 cleanup이며, Redis 도입 작업의 1차 단계로 둔다.
- **장점**:
  - 답안 hot path에서 DB write 0회 → latency 감소, row-level 락 경합 해소.
  - "잃어도 되는 데이터(mid-session state)"와 "잃으면 안 되는 데이터(완료 세션 결과)"를 명시적으로 분리하여 각자에 맞는 durability 비용을 지불.
  - 세션당 DB 트랜잭션 1회로 카운터 hot row 경합이 사라짐.
  - 통계/SRS write가 본질적으로 eventual consistency 허용 가능한 데이터라는 사실이 코드 구조에 드러남.
- **단점**:
  - Redis가 활성 세션의 working state를 들고 있으므로, Redis 장애/eviction 시 진행 중 세션은 손실됨. 단, 이는 의도된 트레이드오프(아래 "검토 후 기각" 참조).
  - 세션 종료 flush 트랜잭션 실패 시 해당 세션 결과가 사라질 수 있어, retry 로직과 idempotent flush 설계가 필요함.
  - 기존 mid-session DB read 경로(`GetBySession`, `isQuestionAnswered`, `nextUnansweredQuestionIndex`, 손글씨 submit 검증 등)를 모두 Redis 기반으로 옮겨야 함.
- **검토 후 기각된 대안**:
  - **(A) 현재 구조 유지 + UPDATE 합치기만**: prerequisite 단계 한정으론 정당하지만, hot path 동기 4 write → 2 write로 줄어들 뿐 mid-session DB read와 카운터 hot row 경합은 그대로. 가정한 규모에서 부족한 개선폭이라 최종 대안으로는 기각.
  - **(B) "Redis SSOT"로 활성 세션 전체를 Redis가 소유**: 용어 자체가 부정확하다. 표준 아키텍처에서 Redis가 SSOT 역할을 하는 케이스는 HTTP session, rate limiter, 실시간 leaderboard 등 본질적으로 ephemeral한 데이터에 한정된다. 본 도메인은 DB가 SSOT를 유지해야 한다.
  - **(C) Outbox / event sourcing 패턴 (answer_events append-only log + async worker)**: 표준적으로 검증된 패턴이고 durability/scale 모두 보호하지만, **본 도메인에서 보호하려는 데이터의 가치가 outbox 도입 비용보다 작다.** 구체적으로 — mid-session state(현재 question idx, 부분 답안)는 잃어도 사용자가 세션을 다시 풀면 그만이고, SRS 업데이트 손실은 다음 출제 때 자연 복구되며, 카운터 손실은 통계 미세 어긋남에 그친다. 잃으면 안 되는 건 "완료된 세션의 최종 상태"뿐이고, 이는 종료 시점의 트랜잭션 1회로 보장 가능. 따라서 outbox 추가 복잡도(event log 테이블, async worker, 재처리/idempotency 로직)는 정당화되지 않음.
- **포트폴리오 관점 메모**: 본 ADR의 진짜 가치는 "Redis 도입했다"가 아니라 **각 데이터의 durability 요구를 명시적으로 분석하여 outbox 같은 표준 패턴을 의식적으로 기각한 사고 과정**이다. 채택 패턴 카탈로그보다 트레이드오프 추적이 평가 가능한 1급 산출물이라는 전제에서 작성됨.

---

## ADR-014: 세션 구성 비율 및 문제 유형 분포

- **날짜**: 2026-05-11
- **상태**: **검토 중 (Open)** — 사용자가 인지과학 관점에서 재설계 예정. 결정 시점에 "채택됨"으로 갱신.
- **맥락**:
  - 현재 `internal/service/session_builder.go`의 세션 구성 비율은 다음과 같음:
    - 오전 세션: 15문제 = 새 9 (60%) + 복습 6 (40%)
    - 오후 세션: 10문제 = 새 2 (20%) + 복습 8 (80%)
    - 카테고리(뉴스/시험대비) 비율: `GetNewQuestions(..., category="", ...)` 형태로 호출되어 세션 단계에서 미적용. ADR-008은 *수집 단계*의 4:6 비율을 정의했지만 세션 빌드에는 강제 메커니즘이 없음.
  - 위 비율은 초기 구현 시의 직관값이며, **인지과학적 근거(망각 곡선, spaced repetition, interleaving, desirable difficulty 등)를 반영한 설계는 미수행**.
  - 본 ADR은 작업 중 세션 빌드 규칙이 문서(이전 AGENTS.md §7)와 코드 사이에서 드리프트되어 있던 것을 발견하면서 분리됨. 잘못된 문서를 그대로 유지하는 것보다, 결정되지 않은 영역임을 ADR로 명시하는 편이 안전.
- **결정해야 할 사항**:
  - 세션 유형별 새/복습 비율 (오전 / 오후 / on-demand `BuildReviewSession`)
  - 한 세션 내 문제 유형(객관식 · 빈칸채우기 · 번역 · 듣기 · 독해 · 어순배열) 분포
  - 카테고리 비율(뉴스/시험대비)을 세션 단계에서도 강제할지, 수집 단계의 ADR-008만으로 충분한지
  - 비율의 인지과학적 근거 reference 정리
- **다음 단계 (사용자 본인 작업, 에이전트 위임 대상 아님)**:
  - 망각 곡선, spaced repetition, interleaving vs blocking, desirable difficulty 관련 reference 수집·정리
  - 위 결과를 바탕으로 비율 재산정
  - 결정 후 본 ADR을 "채택됨"으로 갱신 + `session_builder.go`의 const 갱신
- **운영 원칙**: 결정 후에도 비율은 **코드의 const + 본 ADR로만 관리**한다. 별도 문서 사본을 두지 않음 (드리프트 재발 방지).

---

## ADR-015: 학습 팁(Tips) 시스템 도입 — LLM 채점 대기 시간 활용

- **날짜**: 2026-05-11
- **상태**: 채택됨 (스키마/모델/repository/API/Mini App 연동 완료, scheduler 생성은 TODO로 분리)
- **맥락**:
  - 손글씨 Mini App 의 LLM 채점은 수 초 단위 latency. 사용자가 채점 결과만 기다리는 dead time 이 발생.
  - 단순 spinner 보다, 대기 시간을 짧은 학습 팁(요음 규칙, 비슷한 가나 구분, 획순 등)으로 채우면 UX 개선 + 학습량 누적이 동시에 달성됨.
  - 정적 JSON 으로 둘 수도 있으나, 본 프로젝트가 다국어(ADR-009) + JLPT/CEFR 레벨 분기를 이미 갖고 있어 (language, proficiency_level) 별 컨텐츠 자산화가 자연스러움.
- **결정**:
  - `tips` 테이블을 초기 스키마에 포함한다 (`migrations/001_init.sql`, 단일 파일 컨벤션). 컬럼: `language`, `proficiency_level`, `category`, `body (VARCHAR 500)`, `source_model`, `source_prompt_ver`, `is_active`, `created_at`. label 컬럼은 두지 않음 — 카드 eyebrow 는 `TipCategory.DisplayName()` 매핑으로 표시해 시각 일관성 + LLM 프롬프트 단순화.
  - `category` 는 DB 측은 VARCHAR, Go 측 `model.TipCategory` 화이트리스트로 검증 (`DisplayName()` 으로 한국어 eyebrow 매핑). 초기 7개 (가나 손글씨 전용): `kana_youon`, `kana_sokuon`, `kana_dakuten`, `kana_chouon`, `kana_shape`, `kana_stroke`, `kana_hira_vs_kata`. 다른 언어/스킬 추가 시 enum 확장.
  - 컨텐츠는 **AI 생성** — 별도 seeder CLI 가 아니라 **scheduler 세션 빌드 사이클에 통합**한다. (lang, level) 잔고가 50 미만일 때만 한 세션 빌드당 2-3개씩 LLM 으로 생성. 50 도달 후 자동 정지.
  - **dedup 메커니즘은 도입하지 않는다** (현 시점). UNIQUE 제약 / 의미적 dedup 모두 없이 누적, 사용자가 누적 결과를 보고 추후 결정.
  - 런타임 노출은 손글씨 Mini App 한정. `GET /api/miniapp/tips?language=..&level=..` → 클라이언트가 로딩 시 fetch → shuffle/회전.
- **장점**:
  - LLM 비용을 점진 분할 — 한 번에 500개 시드 부담 없음.
  - 잔고 임계치 기반이라 자동 정지, 무한 LLM 호출 위험 없음.
  - 손글씨 외 다른 wait point 가 생겨도 같은 테이블 재활용 가능.
- **단점 / 트레이드오프**:
  - dedup 없이 누적 시 (lang, level) 안에 의미적으로 유사한 tip 이 쌓일 수 있음 — 의도된 절충, 데이터 본 후 결정.
  - 매 세션 빌드 시 `COUNT(*)` 1회 추가 hit — `idx_tips_lang_level_active` 부분 인덱스로 비용 최소화.
- **대안**:
  - 정적 JSON: 다국어/레벨 확장 어려움, 컨텐츠 큐레이션 수동.
  - on-demand 생성 (Mini App 열 때마다 LLM 호출): 비용·지연 폭증, 동일 사용자에게 같은 tip 보이지 않게 하기 어려움.
  - 별도 seeder CLI 일회성 실행: scheduler 통합 대비 운영 포인트 증가, "점진 누적" 특성 살리기 어려움.
- **후속 TODO**: `docs/todos/tip_scheduler_generation.md` — (language, proficiency_level) 잔고가 임계치 미만일 때 scheduler 가 LLM 으로 tip 을 보충하는 생성 경로.

---

## ADR-016: 손글씨 가나 채점은 False Negative 최소화와 빠른 판정을 우선

- **날짜**: 2026-05-28
- **상태**: 채택됨
- **맥락**:
  - 손글씨 Mini App 채점은 작은 모바일 화면에서 손가락으로 입력한 stroke를 소형 PNG로 렌더링한 뒤 LLM multimodal verification으로 처리한다.
  - 사용자는 정답을 알고 쓰는 연습 중이며, 이 기능의 목적은 시험식 엄격 채점보다 반복 학습 흐름과 손글씨 시도 자체를 유지하는 것이다.
  - 실제 테스트에서 사용자가 맞게 썼다고 느낀 답안이 비슷한 kana 간 형태 차이 때문에 오답 처리되는 false negative가 발생했다.
  - LLM 호출 latency는 대부분 provider-side image understanding/queue 구간에서 발생한다. prompt/parameter tuning만으로 latency를 크게 줄이기는 어렵지만, prompt가 획 단위 분석을 유도하면 판단이 보수적이고 느려질 수 있다.
- **결정**:
  - 손글씨 채점 prompt는 `Expected Text` 기준 **Binary Verification**을 유지하되, 학습 UX상 false negative를 줄이는 방향으로 설계한다.
  - 모델에게 stroke-by-stroke forensic analysis를 하지 말고 **quick beginner-practice judgment**를 수행하도록 지시한다.
  - 작은 화면 입력 특성을 고려해 wobble, uneven stroke width, size, spacing, tilt, rough/faint marks는 reject 사유로 보지 않는다.
  - 비슷한 kana 사이에서 애매하지만 `Expected Text`로 plausible 하게 읽히면 accept한다.
  - reject는 명확한 mismatch에 한정한다: character missing/extra/swapped/different, dakuten/handakuten/small kana/sokuon/chouon 등이 **clearly absent or clearly wrong**인 경우.
  - 정답인 경우 feedback은 empty string으로 유지하고, 오답인 경우에도 client가 정답을 이미 보여주므로 필요할 때만 짧은 한국어 correction note를 반환한다.
  - `ReasoningEffort`는 사용하지 않는다. `reasoning_effort=low`와 `MaxCompletionTokens=80` 조합에서 Gemini OpenAI-compatible 응답이 JSON이 아닌 `Here`로 깨진 사례가 있어 안정성을 우선한다.
- **장점**:
  - 사용자가 맞게 썼다고 느낀 답안을 오답 처리하는 좌절감을 줄인다.
  - 작은 화면 손글씨 입력의 물리적 한계를 채점 기준에 반영한다.
  - LLM이 불필요한 획 분석을 덜 하도록 유도해, 가능한 범위에서 판단 경로를 단순화한다.
  - feedback format을 짧게 유지해 Mini App 결과 UI가 흔들리지 않는다.
- **단점 / 트레이드오프**:
  - false negative를 줄이는 대신 false positive가 일부 증가할 수 있다.
  - 비슷한 kana를 엄격하게 구분하는 시험식 채점에는 덜 적합하다.
  - latency 개선은 보장하지 않는다. provider-side multimodal 처리 시간이 dominant하면 체감 속도는 크게 변하지 않을 수 있다.
- **대안**:
  - 엄격 채점 유지: 학습 정확도는 높아질 수 있으나 모바일 손글씨 UX에서 좌절감과 재시도 비용이 커져 기각.
  - local OCR/heuristic 선채점: LLM 호출 감소 가능성이 있으나 kana stroke/shape 판정 구현 비용과 정확도 검증 부담이 커서 후속 최적화 후보로 보류.
  - 모델 교체(`gemini-2.0-flash-lite`, `gemini-2.5-flash-lite`) 실험: latency/cost 개선 가능성이 있으나 채점 품질 A/B가 필요하므로 별도 실험으로 분리.

---

## ADR-017: 진행 중 세션은 Redis Active Session State로 처리

- **날짜**: 2026-05-28
- **상태**: 채택됨
- **맥락**:
  - `showQuestion`, text answer, handwriting submit, restart recovery가 같은 session/session_questions/questions 데이터를 반복 조회했다.
  - 손글씨 제출은 `session_id`, `question_id`를 이미 알고 있는데도 session ownership, question type, membership, grader lookup 단계에서 DB read가 중복됐다.
  - ADR-013은 활성 세션 상태를 Redis 작업영역으로 유지하고 종료 시 DB에 일괄 flush하는 방향을 이미 채택했으나 구현은 미진행이었다.
- **결정**:
  - 진행 중 세션은 Redis `session:{session_id}:working_set`를 authoritative working set으로 사용한다.
  - working set에는 session metadata, ordered session_questions, question copy, progress(`user_answer`, `is_correct`), `current_index`, `answered_count`, `updated_at`, `version`을 함께 저장한다.
  - 세션 시작 시 DB에서 session + session_questions + questions를 한 번에 JOIN load하여 Redis state를 생성한다.
  - 세션 진행 중 문제 표시, 답변 여부 확인, 다음 문제 계산, 손글씨 제출 검증, grader question lookup은 Redis state만 본다.
  - 답변 hot path는 DB write를 하지 않는다. Redis state 안의 progress, question stats, SRS working copy를 갱신한다.
  - 세션 종료 시 Redis state를 DB transaction으로 flush한다:
    - `sessions` completed 처리
    - `session_questions` answer/is_correct 일괄 반영
    - `questions` stats delta와 SRS field 반영
  - flush 성공 후 user streak를 갱신하고 Redis state를 삭제한다.
  - Redis state missing/corrupt 시 DB fallback은 하지 않는다. 진행 중 progress의 신뢰 가능한 SoT가 Redis이므로, stale DB로 복구하면 사용자가 이미 푼 답안이 사라질 수 있다.
- **장점**:
  - 세션 중 반복 DB read와 answer별 DB write를 제거한다.
  - 손글씨 submit에서 session/question/session_questions/grader lookup 중복을 Redis state 조회로 통합한다.
  - 완료 시점 DB transaction으로 flush 경계를 명확히 만들어 batch flush 구조를 코드에 반영한다.
  - Redis state loss를 DB fallback으로 감추지 않아 progress 일관성 정책이 명확하다.
- **단점 / 트레이드오프**:
  - Redis 장애/eviction 시 진행 중 세션은 복구하지 않고 사용자에게 재시작을 안내해야 한다.
  - answer path에서 Redis read-modify-write가 추가되므로, 다중 app instance에서 같은 question에 동시 submit이 들어오는 경우 atomic CAS/Lua 보강 여지가 있다.
  - `questions` SRS field가 전역 row에 있어 동시 세션이 같은 question을 flush하면 SRS는 last-write-wins 성격을 유지한다. 기존 구조의 한계를 이번 작업에서 별도 사용자별 SRS로 확장하지는 않는다.
- **대안**:
  - read-through cache만 도입: DB write hot path가 남고 ADR-013의 batch flush 목표를 달성하지 못해 기각.
  - JOIN query만 추가: 구현은 단순하지만 progress write와 손글씨 중복 lookup 문제가 유지되어 기각.
  - DB fallback 허용: Redis state 손실 시 미flush progress를 잃은 DB 상태로 사용자를 계속 진행시킬 수 있어 기각.

---

## ADR-018: 손글씨 가나 채점은 Expected Text 기반 Conditional Verification으로 제한

- **날짜**: 2026-05-30
- **상태**: 채택됨
- **연관 ADR**: ADR-016 (손글씨 가나 채점은 False Negative 최소화와 빠른 판정을 우선)
- **맥락**:
  - ADR-016에 따라 초보자 모바일 손글씨의 false negative를 줄이는 prompt rubric을 적용했으나, 실제 사용에서 맞게 쓴 `ふ`, `オ`, `ニャ`, `びゃ`가 오답 처리되는 사례가 추가로 발생했다.
  - 특히 `オ`를 visually similar kanji인 `才`로 판정한 사례는 모델이 `Expected Text` 검증보다 대체 OCR 해석을 우선할 수 있음을 보여준다.
  - 이 기능은 시험식 OCR이 아니라 정답을 이미 알고 있는 학습 흐름의 Binary Grading이다. 대체 transcription 생성은 채점 목적에 필요하지 않다.
- **결정**:
  - 손글씨 채점 prompt를 `Expected Text` 기반 **Conditional Verification**으로 명시한다.
  - `Expected Text`가 plausible하게 읽히면 다른 kana 또는 kanji 해석 가능성이 있더라도 accept한다.
  - 모델이 대체 transcription을 탐색하거나 우선하지 않도록 지시한다.
  - rough mobile handwriting, joined/separated strokes, uneven proportions, ambiguous small kana, dakuten/handakuten 등은 plausibly present하면 accept한다.
  - 특정 문자에만 과적합되지 않도록 범용 규칙을 우선 기술하고, 실제 실패 사례인 `Expected Text: オ`, alternative interpretation `才`를 대표 예시 하나로만 추가한다.
  - 정답 feedback은 empty string으로 유지한다. 오답 feedback은 Expected Text에서 명확히 누락되거나 잘못된 feature가 있을 때만 짧은 한국어 correction note 한 문장으로 반환한다.
  - feedback에서 대체 문자를 제안, transcription, 언급하지 않는다. 신뢰할 수 있는 correction note가 없으면 empty string을 반환한다.
- **장점**:
  - Binary Verification 경계를 명확히 하여 대체 OCR 해석으로 인한 false negative를 줄인다.
  - 예시를 하나로 제한해 특정 문자에 대한 anchoring과 prompt 비대화를 억제한다.
  - 오답 feedback을 Expected Text 기준으로 제한해 학습 UX를 유지하면서 대체 OCR 해석으로의 회귀를 억제한다.
- **단점 / 트레이드오프**:
  - false negative 감소를 우선하므로 visually similar character에 대한 false positive가 증가할 수 있다.
  - 오답 feedback을 제한하므로 대체 문자 비교를 활용한 상세 교정은 제공하지 않는다.
  - prompt 변경만으로 모델의 판정 일관성이 완전히 보장되지는 않는다. 실제 사례 기반 회귀 검증이 별도로 필요하다.
- **대안**:
  - 범용 규칙만 추가: anchoring 위험은 가장 낮지만 실제 실패 모드의 우선순위를 모델에 충분히 전달하지 못할 수 있어 기각.
  - 여러 few-shot 예시 추가: 사례별 적중률은 높아질 수 있으나 prompt가 회귀 테스트 목록처럼 비대해지고 특정 문자에 과적합될 수 있어 기각.
  - 기존 ADR-016만 유지: 대체 OCR 해석에 대한 명시적 제한이 없어 기각.

---

## ADR-019: 손글씨 채점은 Static PNG Evidence Boundary 안에서만 판정

- **날짜**: 2026-05-31
- **상태**: 채택됨
- **연관 ADR**: ADR-016 (손글씨 가나 채점은 False Negative 최소화와 빠른 판정을 우선), ADR-018 (손글씨 가나 채점은 Expected Text 기반 Conditional Verification으로 제한)
- **맥락**:
  - Mini App은 사용자가 손가락으로 그린 sampled stroke points를 서버에 전송한다.
  - 서버는 points를 static PNG로 rebuild하고, LLM에는 최종 PNG만 전달한다.
  - LLM에는 stroke 순서, 시작점, 진행 방향 같은 temporal pen-movement 정보가 전달되지 않는다.
  - 실제 사용에서 LLM이 획순이나 작성 방향을 근거로 feedback을 생성하고, `ン/ソ`, `シ/ツ`, `ヤ/や`, 탁점/반탁점처럼 bitmap상 애매한 입력을 오답 처리하는 false negative가 발생했다.
- **결정**:
  - Prompt에 입력 provenance를 짧게 명시한다: 모바일 canvas에서 수집한 sampled stroke points를 서버가 static PNG로 rebuild했고, LLM은 그 PNG만 받는다.
  - LLM은 final visible bitmap만 평가한다.
  - 획순, 시작점, 작성 방향, pen movement를 추론하거나 채점 근거로 사용하지 않는다.
  - feedback에서도 획순, 시작점, 작성 방향, pen movement를 언급하지 않는다.
  - 다른 문자와 구분하려면 temporal pen-movement 정보가 필요한 경우, `Expected Text`가 plausible하면 정답 처리한다.
  - script identity 또는 diacritic type이 rough mobile handwriting에서 애매하면, `Expected Text`가 plausible한 경우 정답 처리한다.
  - 요음의 작은 `ゃ/ゅ/ょ`, `ャ/ュ/ョ`는 손가락 입력에서 비율과 shape가 거칠 수 있으므로 textbook size, proportions, exact shape를 요구하지 않는다.
  - 요음은 expected 위치에 plausible한 두 번째 작은 mark가 있고 전체 `Expected Text`가 plausible하면 정답 처리한다.
  - 요음 오답 처리는 작은 kana가 명확히 없거나 unrelated shape로 명확히 대체된 경우로 제한한다.
  - 특정 혼동 문자 pair를 Prompt에 계속 누적하지 않고 범용 규칙으로 유지한다.
- **장점**:
  - LLM 입력에 존재하지 않는 정보를 근거로 한 hallucination을 억제한다.
  - `Expected Text` 기반 Conditional Verification의 판정 경계를 더 명확히 한다.
  - 문자 pair별 예시 누적으로 인한 Prompt 비대화와 anchoring을 줄인다.
- **단점 / 트레이드오프**:
  - 애매한 문자와 mark를 정답 처리하므로 false positive가 증가할 수 있다.
  - Prompt만으로 provider model의 규칙 준수를 완전히 보장할 수 없다.
  - 서버 rebuild PNG 자체의 손실 여부는 별도 Renderer 정합성 검증이 필요하다.
- **대안**:
  - 혼동 문자 pair별 few-shot 예시 추가: 단기 적중률은 올라갈 수 있으나 Prompt 유지보수 부담과 anchoring 위험이 커서 기각.
  - 획순 feedback만 금지하고 provenance는 생략: 금지 이유와 evidence boundary가 약해져 기각.
  - Do nothing: 근거 없는 획순 feedback과 false negative가 지속되어 기각.

---

## ADR-020: 손글씨 PNG Renderer는 Bounded Aspect-Ratio Canvas를 사용

- **날짜**: 2026-05-31
- **상태**: 채택됨
- **연관 ADR**: ADR-016 (손글씨 가나 채점은 False Negative 최소화와 빠른 판정을 우선), ADR-019 (손글씨 채점은 Static PNG Evidence Boundary 안에서만 판정)
- **맥락**:
  - Mini App은 답안 글자 수에 비례해 가로로 긴 canvas를 제공한다.
  - 기존 서버 Renderer는 모든 답안을 고정 `512x512` PNG로 rebuild했다.
  - 긴 답안은 제한된 정사각형 안에 fit되면서 글자당 픽셀이 감소하고, 작은 가나·탁점·반탁점 같은 미세 feature가 뭉개질 수 있다.
  - 반대로 이미지를 무제한 확장하면 Base64 upload bytes, provider image processing latency, cost가 증가한다.
- **결정**:
  - 손글씨 PNG Renderer는 원본 stroke bounding box 비율을 유지하는 uniform scale을 사용한다.
  - 기본 canvas 높이는 기존 검증값인 `512px`로 유지한다.
  - canvas 폭은 bounding box 비율에 따라 `512~1536px` 범위에서 계산한다.
  - 가로로 긴 답안은 폭을 확장하되 `1536px`를 넘지 않는다.
  - 짧거나 세로로 긴 답안은 폭을 `512px` 아래로 줄이지 않는다.
  - padding은 기존 `48px`, brush 반경은 높이 기준 비례값을 유지한다.
  - `512~1536px` 범위는 calibration 시작점으로 사용하고, 실제 품질·latency 확인 후 별도 ADR로 조정할 수 있다.
- **장점**:
  - 다글자 답안의 작은 feature를 고정 정사각형보다 더 많은 픽셀로 보존한다.
  - 원본 비율을 유지해 stretch distortion을 만들지 않는다.
  - 최대 폭을 제한해 upload 및 provider 처리 비용 증가를 통제한다.
- **단점 / 트레이드오프**:
  - 기존보다 PNG dimensions와 Base64 payload가 증가할 수 있다.
  - 매우 긴 답안은 `1536px` 상한 때문에 여전히 축소된다.
  - 최적 상한값은 실제 Mini App 품질·latency 확인이 필요하다.
- **대안**:
  - 고정 `1024x1024` 확대: 단일 문자에도 불필요한 비용이 증가하고 긴 답안 비율 문제가 유지되어 기각.
  - 폭 무제한 확장: 품질은 높아질 수 있으나 upload·latency·cost 상한이 없어 기각.
  - 기존 `512x512` 유지: 다글자 미세 feature 손실 가능성이 유지되어 기각.
