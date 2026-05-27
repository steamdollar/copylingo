# 손글씨 Mini App 학습 팁 통합

- **날짜**: 2026-05-19 ~ 2026-05-20
- **분류**: AGENTS.md §3 Case B (side task — 손글씨 채점 대기 UX 개선)
- **검증**:
  - `make test` 통과
  - `migrations/001_init.sql` transaction 실행 후 `ROLLBACK` 으로 syntax 확인
  - `GET /api/miniapp/tips?language=ja&level=N5` 가 DB의 active tip 을 반환하는 것 확인
  - `GET /miniapp/handwriting?...&language=ja&level=N5` 가 cache-busting asset URL 을 포함한 HTML 을 반환하는 것 확인

---

## 배경

손글씨 Mini App 제출 후 LLM 채점까지 수 초가 걸린다. 기존에는 사용자가 단순 status 문구만 보고 기다려야 했으므로, 채점 대기 시간을 짧은 학습 팁 노출 시간으로 전환하기 위해 `tips` schema, 조회 API, Mini App 표시 흐름을 연결했다.

## 변경 사항

### DB / 문서

- `tips` 테이블을 별도 migration 에 두지 않고 `migrations/001_init.sql` 초기 schema 에 통합.
- `CREATE TABLE` / `CREATE INDEX`에 `IF NOT EXISTS`를 적용해 로컬 재실행 내성을 높임.
- `schema.dbml`, `docs/ADR.md`, `docs/todos/tip_scheduler_generation.md`의 tips schema 참조를 현재 구조에 맞게 정리.

### Tips API

- `GET /api/miniapp/tips` 추가.
- query:
  - `language` 필수
  - `level` 필수
  - `limit` 선택, 기본 30, 최대 50
- 응답은 `model.Tip`의 public JSON 필드만 반환한다: `id`, `language`, `proficiency_level`, `category`, `body`.
- 인증은 적용하지 않는다. tip 은 read-only public content 이고, Telegram init data 검증은 상태 변경 endpoint 에만 유지한다.
- `service.TipService`를 추가해 handler 가 repository 에 직접 의존하지 않도록 유지했다.

### Mini App 연동

- 봇이 생성하는 손글씨 Mini App URL에 question 기준 `language`, `level` query 를 추가했다.
- Mini App은 page load 시 tip 목록을 한 번 fetch 한 뒤 shuffle 해서 채점 중 표시한다.
- 제출 후 채점 완료 전까지 CSS spinner 와 tip card 를 보여주고, 응답 도착 후 loading panel 을 닫는다.
- tip fetch 실패 또는 빈 결과는 사용자에게 에러로 노출하지 않고 spinner 만 보여준다.

### 후속 수정

- `loadTips()` 가 늦게 끝나는 경우에도 이미 채점 중이면 tip rotation 이 시작되도록 `loadingActive` 상태와 `startTipRotation()` guard 를 추가했다.
- 같은 WebView session 에서 손글씨 문제를 반복해 열 때 `(language, level)` 기준 `sessionStorage` cache 를 재사용해 tips API 반복 호출을 줄였다.
- tips 응답이 배열이 아닌 경우 `shuffle()` 호출 전에 무시하도록 `Array.isArray()` guard 를 추가했다.
- Telegram WebView asset cache 회피를 위해 `style.css` / `app.js` URL 에 version query 를 붙였다.
- 리뷰 후 `GET /api/miniapp/tips` 의 빈 결과가 `null` 이 아니라 `[]` 로 반환되도록 수정했다.
- `TipService.ListActive` error path 테스트와 손글씨 메시지 ref parsing 테스트를 추가했다.

### 부수 변경: 제출 후 Telegram 버튼 정리

- 손글씨 문항 메시지의 `message_id` 를 Redis 에 `handwriting:msg:{session_id}:{question_id}` 로 1시간 저장한다.
- Mini App 채점 성공 후 서버가 best-effort goroutine 으로 해당 Telegram 메시지의 inline keyboard 를 `다음 문제 →` 버튼 하나로 정리한다.
- Redis 값이 깨진 경우 `chat_id=0` / `message_id=0` 으로 Telegram API 를 호출하지 않도록 parsing validation 을 추가했다.

## 결정 사항

- 서버 cache layer 는 두지 않는다. `idx_tips_lang_level_active` partial index 로 `(language, proficiency_level, is_active)` 조회 비용을 낮추고, Mini App client 는 `sessionStorage` cache 로 같은 session 내 반복 조회만 줄인다.
- `IF NOT EXISTS`는 schema drift 보정 수단이 아니라 “없는 객체 생성” 수준의 idempotency 로만 취급한다.
- tip category 표시명은 현 단계에서 Mini App JS map 으로 유지한다. 서버가 `category_display`를 내려주는 구조는 API surface 확장이므로 보류했다.
- 제출 후 Telegram 버튼 정리는 flow-local UX cleanup 으로 보고 ADR 로 분리하지 않는다. 구현 메모는 본 workthrough 에만 남긴다.

## 남은 일

- `tip_scheduler_generation.md` 실행 전까지는 수동 insert 된 tip 만 노출된다.
