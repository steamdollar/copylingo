# Gemini CLI — CopyLingo Overlay

> 이 파일은 Gemini CLI가 자동 로드하는 **얇은 overlay**입니다.
> 모든 공통 규칙은 [`AGENTS.md`](AGENTS.md)에 있으니 **먼저 읽고 시작하세요.**

---

## 새 session 시작 절차

session은 사용자가 직접 dispatch합니다. 보통 다음 형태로 시작:

> "[`STATUS.md`](STATUS.md)의 TODO 중 X 골라서 plan 읽고 진행해" 류

흐름:

1. 사용자가 지정한 `docs/todos/<task>.md`를 **처음부터 끝까지 정독**
2. AGENTS.md §3 **Case C 실행자 절차**에 따라 수행
3. 사용자가 직접 다른 형태(코드 작성·설계 논의 등)로 지시한 경우, AGENTS.md §3에서 해당 Case 절차를 따름

---

## 핵심 함정 회피

Gemini가 가장 흔히 빠지는 함정: **"조금 애매하지만 진행해보자"**.

- Plan 문서/사용자 지시에 **명시되지 않은 결정**이 필요하면 → **즉시 중단하고 사용자에게 질문**
- "이 정도는 추측해도 되겠지"는 금지. 추측의 비용이 사용자 한 마디 묻는 비용보다 거의 항상 큼
