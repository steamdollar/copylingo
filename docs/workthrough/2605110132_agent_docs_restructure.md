# 에이전트 가이드라인 문서 재구성 (AGENTS / CLAUDE / GEMINI)

- **날짜**: 2026-05-11
- **분류**: AGENTS.md §3 Case B (side task — 진행 중 Phase 2.4와 무관한 문서 정리)
- **검증**: 코드/마이그레이션/설정 변경 없음 → `make test` 미실행. 문서 변경만 발생.

---

## 배경

- 기존 `AGENTS.md` / `CLAUDE.md` / `GEMINI.md`가 부풀어 있고 중복이 심했음:
  - `AGENTS.md`와 `GEMINI.md`는 **100% 동일한 내용** (같은 11686 bytes)
  - `CLAUDE.md`는 그 일부를 축약 복제
  - 세 파일 합계 700줄+
- 기존 워크플로우 규칙이 산발적으로 흩어져 있어 어떤 케이스에 어떤 절차가 적용되는지 불명확
- 또한 일부 문서 내용이 코드와 드리프트되어 있음 (특히 세션 빌드 규칙 — 본 작업 중 발견)

## 결정한 구조

`AGENTS.md` = SSOT (모든 에이전트 공유), 나머지는 thin overlay:

| 파일 | 역할 | 자동 로드 |
|---|---|---|
| `AGENTS.md` | 공통 규칙 SSOT — 진입 규칙, 역할 매트릭스, 3-case 프로토콜, 프로젝트 성격, 코딩 규칙, 참고 문서 | Codex가 직접 |
| `CLAUDE.md` | thin overlay — AGENTS.md 위임 + 시작 절차 | Claude Code |
| `GEMINI.md` | thin overlay — AGENTS.md 위임 + 디스패치 패턴 + 함정 회피 | Gemini CLI |

**핵심 통찰**: 각 CLI는 자기 컨벤션 파일을 자동 로드하므로, AGENTS.md는 *Codex의 사실상 전용 파일*이기도 하다. 그래서 `CODEX.md`는 만들지 않음.

## 새로 박은 핵심 규칙

### 1. 3-case 작업 프로토콜 (`AGENTS.md` §3)

사용자 요청을 셋 중 하나로 분류:
- **Case A**: 의사결정 (ADR)
- **Case B**: 코드 작성 (plan → 구현 → 검증)
- **Case C**: TODO 분리 및 위임

각 케이스에 트리거 / 담당 / 절차 / 산출물 명시. 케이스 간 전이(A→B, B→A, B→C, C→B)도 표로 명시.

### 2. 사용자 = 최종 결정권자 (`AGENTS.md` §2)

역할 매트릭스 최상단에 사용자 행 추가. 비자명한 결정은 반드시 사용자 confirmation.

### 3. 역할 대체 / 폴백 (`AGENTS.md` §2)

매트릭스는 default일 뿐 권한 룰 아님. quota/장애 등으로 다른 에이전트가 다른 역할을 임시 수행 가능. 단 §3 절차는 누가 하든 동일.

### 4. ADR 능동 갱신 (`AGENTS.md` §3 Case A)

결정이 굳어지면 사용자 별도 요청 없이도 에이전트가 즉시 ADR 갱신. 사용자가 자주 잊는다는 점을 명시. 같은 행동 규칙을 메모리에도 저장 (`feedback_proactive_doc_updates.md`).

### 5. STATUS.md 갱신 조건부화 (`AGENTS.md` §3 Case B)

"진행 중" 항목을 실제로 완료할 때만 이동. side task는 STATUS.md를 안 건드리거나 "최근 완료"에만 한 줄 — 진행 중인 Phase 작업이 오염되지 않도록.

### 6. Case C plan 문서 강제 (`AGENTS.md` §3 Case C)

`docs/todos/<task>.md`를 자기완결적 plan으로 **반드시** 작성 (한 줄 요약 escape hatch 없음). 실행자는 정독 후 명확하면 즉시 시작, 아니면 사용자에게 질문.

## 분리·이동된 내용

| 원래 위치 | 이동 후 | 사유 |
|---|---|---|
| 기존 `AGENTS.md` §4 (프로젝트 개요) | `README.md` | 레퍼런스 자료, 워크플로우 규칙 아님 |
| 기존 `AGENTS.md` §6 (기술 스택) | `README.md` | 레퍼런스 자료 |
| 기존 `AGENTS.md` §7 (프로젝트 구조 디렉토리 트리) | 삭제 | `ls` 한 번이면 자명함, 갱신 누락 위험만 큼 |
| 기존 `AGENTS.md` §9 (핵심 비즈니스 로직) | 삭제 + `docs/ADR.md` ADR-014 (Open) | 코드와 드리프트 발견. 코드 SSOT 원칙으로 전환 |
| 기존 `AGENTS.md` §10 (개발 명령어) | 삭제 | `make test`는 §3에 inline으로 박힘. 나머지는 README/Makefile에 있음 |

## 발견·기록한 이슈

### 세션 빌드 규칙 드리프트

기존 문서가 적어둔 비율과 실제 코드가 다름:
- 오전 세션: 문서·코드 모두 `15 = 9(60%) + 6(40%)` ✅
- 오후 세션: 문서 `10 = 6(60%) + 4(40%)`, 실제 코드 `10 = 2(20%) + 8(80%)` ❌
- 카테고리 비율(뉴스 40%/시험 60%): 코드의 `GetNewQuestions(..., category="", ...)` 호출로 세션 단계에서 미적용. ADR-008은 *수집 단계* 비율만 정의됨

→ 사용자 본인이 인지과학 관점에서 재설계 예정인 영역으로 확인. 잘못된 문서를 유지하는 대신 `docs/ADR.md`에 **ADR-014 (Status: Open)**으로 분리. 결정 후 "채택됨"으로 갱신할 예정. 향후 비율은 코드 const + ADR로만 관리 (별도 문서 사본 금지).

### Gemini 모델 버전 불일치

- 기존 `CLAUDE.md`: Gemini 3.0 Flash
- 기존 `AGENTS.md`: Gemini 3.1 Flash Lite
- `README.md`: 표는 2.0 Flash, config 예시는 3.1 Flash Lite (자기 모순)

→ 통일: **Gemini 3.1 Flash Lite**. README 갱신, AGENTS.md에서 해당 항목 제거(README로 위임).

## 변경 파일

- `AGENTS.md` — 314줄 → 195줄. 6섹션 구조 (진입/역할/3-case/성격/코딩/참고). 추가로 사용자가 Case B 설명에 "→ 종료" 명시, Case C 실행 절차를 Case B 위임 + Case C 고유 차이만 명시하는 형태로 단축, "설정" 서브섹션을 4항목에서 1항목(민감 정보 환경변수 주입)으로 압축
- `CLAUDE.md` — 60줄 → 20줄. thin overlay
- `GEMINI.md` — 220줄 → 27줄. thin overlay (디스패치 패턴 + 함정 회피)
- `README.md` — 프로젝트 성격 섹션 추가, 기술 스택 표 보강·통일
- `docs/ADR.md` — ADR-014 추가 (세션 구성 비율, Open)
- `STATUS.md` — "📝 최근 완료"에 본 작업 한 줄 추가 (Phase 2.4 진행 중 항목 미변경)
- `docs/archived/agents/` — 원본 `CLAUDE.md`, `GEMINI.md`를 git HEAD에서 복원해 보관 (사용자가 미리 보관한 `AGENTS.md`와 같은 위치)

## 신규 메모리

- `feedback_proactive_doc_updates.md` — ADR/STATUS/workthrough 능동 갱신 규칙
- `feedback_tiny_edits_to_user.md` — 글자 하나·변수명 하나 같은 매우 작은 변경은 사용자에게 직접 위임 (토큰 효율)

## 추가 정리: 한글 음차어 → 영어 원어 일괄 치환

매 session마다 로드되는 SSOT 문서들의 토큰 효율을 위해 다음 음차어들을 영어로 일괄 치환:

| 변경 | 적용 파일 |
|---|---|
| 에이전트 → agent | AGENTS / CLAUDE / GEMINI / README / 메모리 4개 |
| protocol, session, tradeoff, fallback, dispatch, matrix, overlay, main, case, rule, review | 동일 (단어별로 등장 파일에만) |

- README.md는 `메인`이 모두 `도메인` 합성어 안에 있어 `메인` 치환만 제외
- 한글 음차어가 영어 원어 대비 보통 2~4배 토큰을 소비함을 활용한 마이크로 최적화. SSOT가 매 session 로드되므로 누적 효과 발생.
