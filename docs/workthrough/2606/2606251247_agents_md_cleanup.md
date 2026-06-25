# AGENTS.md 정리 — 죽은 링크 수정 · 중복 제거 · per-session reference 분리

## 배경/목적

AGENTS.md는 Codex가 매 세션 자동 로드하는 SSOT(210줄)라 모든 줄이 세션당 토큰 세금이다. 리뷰 결과 (a) 정확성 버그(죽은 ADR 링크), (b) 중복(콜백 규약·YAGNI 문구), (c) 매 세션 안 읽어도 되는 reference가 본문에 상주하는 문제를 발견해 정리했다.

## 변경 사항

### P1. 정확성 — 죽은 `docs/ADR.md` 링크 (4곳)
- 실제 ADR은 `docs/adr/ADR_from_01_to_20.md`, `ADR_from_21_to_40.md`로 범위 분할돼 있는데 AGENTS.md는 존재하지 않는 `docs/ADR.md`를 가리키고 있었다 (ADR-028까지 차서 `21_to_40`이 활성).
- Case A(L63)·Case B(L82)·§4(L143)·§6(L204)의 링크를 `docs/adr/`로 수정. 신규 ADR은 "최신 ADR 파일"에 추가하도록 문구 조정.

### P2. 중복 제거
- **Callback Data 규약**: §5 텔레그램 1번이 `docs/ARCHITECTURE.md`의 "Callback Data 규약"과 중복(ARCHITECTURE가 더 완전·최신). AGENTS.md/CONVENTIONS.md에서 literal 정의를 제거하고 ARCHITECTURE를 SSOT로 참조.
- **YAGNI/1인 문구**: Case A 주의(L65)와 §4의 중복을 §4를 근거 SSOT로 두고 L65는 cross-ref 한 줄로 축소.

### P3. per-session reference 분리
- **§5 코딩 규칙(~40줄)** → `docs/CONVENTIONS.md` 신설로 이관. AGENTS.md §5는 thin pointer로 대체(번호 유지 → Case B의 "§5 준수" 참조 보존). 코드 작성 세션에서만 읽으면 됨.
- **§2 Subagent ROI 5불릿** → `docs/NATIVE_SUBAGENT_DELEGATION.md`에 "Delegation ROI Gate" 섹션 신설로 이관. 본문엔 핵심 원칙 한 단락 + 포인터만.
- **Case C 실행 step1-2**(`GEMINI_CLI_EXECUTION.md` contract와 중복) 축약. Case C 고유 종료처리(STATUS 체크박스 제거·todo 파일 삭제)는 유지.

## 결과

- AGENTS.md: 210 → 164줄 (−46, ~22%).
- 신규: `docs/CONVENTIONS.md`. 보강: `docs/NATIVE_SUBAGENT_DELEGATION.md`(ROI Gate 섹션).
- 콜백 규약 SSOT는 ARCHITECTURE 단일화.

## 검증

- 문서-only 변경이라 `make test` 미실행.
- `rg`로 AGENTS.md 내 잔존 `docs/ADR.md` 참조 0 확인.
- AGENTS.md가 참조하는 5개 파일(CONVENTIONS / adr / NATIVE_SUBAGENT_DELEGATION / GEMINI_CLI_EXECUTION / ARCHITECTURE) 존재 확인.
- 콜백 literal(`q:{session_id}`)이 AGENTS.md에서 사라지고 ARCHITECTURE/CONVENTIONS에만 존재함 확인.

## 결정 / 비고

- ADR 범위 분할(01-20 / 21-40) 구조는 유지하기로 함(단일 `docs/ADR.md` 병합 대신 링크만 정정).
- §5 번호를 제거·재배치하지 않고 pointer로 남긴 것은 Case B 본문의 "§5" 참조를 깨지 않기 위함.
