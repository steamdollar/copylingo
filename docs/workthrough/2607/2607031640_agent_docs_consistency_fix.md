# Agent 문서 정합성 수정 (AGENTS.md / CLAUDE.md / STATUS.md)

- **날짜**: 2026-07-03
- **분류**: Case B (docs-only)
- **배경**: AGENTS.md/CLAUDE.md 검토(Case 0)에서 최근 작업(ADR-031/032 분리 파일, workthrough 월별 디렉토리화)과 문서 기술이 어긋난 지점 6건 발견 → 사용자 승인 후 일괄 수정.

## 변경 내역

| # | 파일 | 수정 |
|---|---|---|
| 1 | `CLAUDE.md` | "one of the 3 cases" → Case 분류 체계(4-case, default Case 0) 반영 |
| 2 | `AGENTS.md` Case B step 5 | workthrough 경로 `docs/workthrough/YYMM/YYMMDDhhmm_<job>.md` (월별 하위 디렉토리) 로 수정 |
| 3 | `AGENTS.md` Case A step 2 + §6 | **ADR 분리 파일 규칙 명문화**: entry chunk가 크거나 프로젝트에 중요한 결정은 별도 파일(시리즈 번호 유지) + range 파일에 pointer stub |
| 4 | `AGENTS.md` §4.4 | "OpenAI-compatible mode" 단일 창구 기술 수정 — chat은 compat layer, TTS는 native `generateContent` (ADR-031); "1,500 RPD/month" 단위 모순 제거 |
| 5 | `AGENTS.md` §2 + `CLAUDE.md` | capability difference 기록처 SSOT 분리: 공통 차이 = AGENTS.md §2, agent 전용 usage pattern = 각 overlay 파일 |
| 6 | `STATUS.md` TODO 헤더 | dead reference `"TODO 문서 프로토콜"` → `AGENTS.md §3 Case C` |

## 검증

- `make test` **skip** — docs-only 변경, 코드/마이그레이션/config 무변경.

## 미처리 (사용자 판단 대기)

- `docs/todos/02_integration_test_plan.md`, `03_e2e_test_plan.md` 가 STATUS.md TODO 체크박스에 미등록(고아 상태) — 유효 시 등록 / 폐기 시 삭제 결정 필요.
