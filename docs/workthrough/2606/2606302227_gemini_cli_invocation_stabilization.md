# Gemini CLI 호출 안정화 래퍼 스크립트 + 자체 테스트

> TODO: `docs/todos/future_gemini_cli_invocation_stabilization.md`

## 배경

main agent가 Gemini CLI를 수동으로 호출하던 워크플로(`docs/GEMINI_CLI_DELEGATION.md` / `docs/GEMINI_CLI_EXECUTION.md`)에서, 2026-06-02 N5 vocab catalog 확장 중 일시적 provider 오류와 response-level Tool Call 오류가 반복됐다(`429 MODEL_CAPACITY_EXHAUSTED`, `503 UNAVAILABLE`, empty stream, `malformed tool call`, `replace.old_string` 누락, 파일 변경 없는 완료 보고). 호출 동작을 일관되게 만들되, **부분 편집을 남길 수 있는 재시도는 자동화하지 않는다**는 것이 목표.

## 변경 파일

- `scripts/run_gemini_executor.sh` (신규)
  - CLI: `run_gemini_executor.sh docs/todos/<task>.md`. TODO path 부재/읽기 불가 시 Gemini 호출 전 거부(exit 2).
  - 모델 순서: default `gemini-3.1-flash-lite` → fallback `gemini-2.5-flash`.
  - provider 오류(`429|MODEL_CAPACITY_EXHAUSTED|503|UNAVAILABLE`) 재시도: 5s → 10s → fallback 전환, 양 모델 모두 소진 시 exit 3.
  - Tool Call 오류(`Invalid stream|malformed tool call|required property|could not find the string to replace`): **자동 재시도 없음**, exit 4 + recovery prompt 출력 + `git status --short`/diff 점검 지시.
  - stdout/stderr 보존(조합 출력을 임시파일에 캡처 후 그대로 출력), 임시파일은 `trap ... EXIT`로 정리.
  - exit code: `2` usage / `3` provider 소진 / `4` Tool Call / `5` 일반 Gemini 실패.
  - `SUBAGENT_RETRY_SLEEP_SCALE`(기본 1, 테스트에서 0)로 sleep 스케일 조정.
- `scripts/run_gemini_executor_test.sh` (신규)
  - PATH 앞단에 fake `gemini` 실행파일을 두고 실제 CLI 미호출. 모든 케이스 격리 임시 디렉터리 + `trap` 정리.
  - fake는 sandbox의 `control.env`(`FAKE_MODE`/`FAKE_FAIL_COUNT`/`FAKE_FAIL_MODEL`/`FAKE_TOOLCALL_MSG`)와 모델별 카운터로 "N회 실패 후 성공", "특정 모델만 실패"를 재현.
  - 문서 명시 7케이스 전부 커버 + 호출횟수/recovery-prompt 출력 보조 단언 → 총 32 assertions.
- `docs/GEMINI_CLI_DELEGATION.md` (갱신)
  - "Dispatch" 하위에 "Dispatch Via Wrapper (preferred)" 절 추가: 래퍼 사용법/자동화 범위/exit code/자체 테스트 안내. 기존 정책 텍스트는 SSOT로 유지하고 래퍼가 그것을 집행한다는 점만 명시.

## 결정 (문서 "Decisions Already Made"를 그대로 따름)

- Bash 래퍼(`scripts/`), 3rd-party 의존성 없음.
- provider 오류만 안전 재시도. Tool Call 오류는 탐지하되 자동 복구 편집은 범위 밖 — main agent가 부분 편집을 먼저 점검.
- Tool Call 오류 분류는 **exit code가 아니라 출력 패턴** 우선(fake가 exit 0으로 Tool Call 메시지를 내도 exit 4로 분류됨을 테스트로 고정).
- 임의 결정 없음. 앱 Go/DB/API 무변경.

## 검증

- `bash -n scripts/run_gemini_executor.sh` 통과.
- `bash -n scripts/run_gemini_executor_test.sh` 통과.
- `bash scripts/run_gemini_executor_test.sh` → `TOTAL pass=32 fail=0` (exit 0). 7케이스 모두 통과:
  missing-todo(호출 전 차단) / success / 2회 provider 실패 후 동모델 재시도 성공 / 3회 실패 후 fallback 전환 / 양 모델 소진 exit 3 / Tool Call 4종 각각 exit 4 + 무재시도 / 일반 비정상 종료 exit 5.
- `git diff --check` 통과(whitespace 이슈 없음).
- `make test`(go test): 앱 코드 무변경이라 비대상. 실제 Gemini API 호출 없음(fake 경로만).

## 구현 중 수정한 자체 테스트 결함 2건

1. missing-todo 케이스가 exit 1로 나옴 — EXIT trap의 `cleanup`이 `[ -n "" ] && rm` 단축평가로 1을 반환해 `exit 2`를 덮어씀. `cleanup`을 `if`문으로 바꿔 항상 0 반환하도록 수정.
2. fallback 전환 케이스가 fail — 카운터가 모델별이라 `FAKE_FAIL_COUNT`가 fallback에도 동일 적용돼 fallback이 회복 못 함. fake에 `FAKE_FAIL_MODEL`을 추가해 default만 실패시키도록 테스트 시나리오 교정.

## 미해결 / 주의사항

- STATUS.md / `docs/todos`의 해당 문서는 본 작업에서 손대지 않음(closure는 상위 agent 처리).
- 실제 Gemini CLI 연동 검증은 quota 때문에 미수행 — fake 기반 계약 테스트로만 보장. 실제 CLI의 오류 메시지 표면 문구가 패턴과 다르면 탐지 정규식 보강 필요(현재는 문서 명시 문자열 기준).
