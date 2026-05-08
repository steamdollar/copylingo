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
