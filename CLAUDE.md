# Claude Code — CopyLingo Overlay

> 이 파일은 Claude Code가 자동 로드하는 **얇은 overlay**입니다.
> 모든 공통 규칙은 [`AGENTS.md`](AGENTS.md)에 있으니 **먼저 읽고 시작하세요.**
> AGENTS.md가 모든 agent의 SSOT이며, 본 파일은 그 위에 얹는 Claude Code 고유 사항만 둡니다.

---

## 새 session 시작 절차

1. [`AGENTS.md`](AGENTS.md) 정독 — 진입 규칙 / 역할 matrix / 3-case 작업 protocol / 결정 기준 / 코딩 규칙
2. [`STATUS.md`](STATUS.md) 확인 — 현재 "🔨 진행 중" 항목과 사용자 요청의 관계 판단
3. 사용자 요청을 AGENTS.md §3 **3 cases 중 하나로 분류** 후 해당 절차 시작

---

## Claude Code 고유 사항

- Claude Code와 Codex는 현재 **대등한 main agent**입니다 (AGENTS.md §2). **Claude 전용 절차나 예외는 없음** — AGENTS.md를 그대로 따릅니다.
- 의미 있는 능력 차이나 사용 패턴 차이가 식별되면 본 파일에 누적합니다.
