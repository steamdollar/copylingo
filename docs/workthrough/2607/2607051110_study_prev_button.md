# Study Session 이전 카드 이동 버튼 추가

- 일시: 2026-07-05 11:10
- 요청: study session에서 이미 본 카드를 다시 볼 수 있도록 뒤로 가는 버튼 추가

## 변경 내용

| 파일 | 변경 |
|---|---|
| `internal/config/constants.go` | callback format `FormatStudyPrev = "study:%d:prev:%d"` 추가 |
| `internal/bot/study_flow.go` | `HandleCallback`에 `prev` 분기, `prevMaterial` 핸들러 추가, `studyMaterialKeyboard`에 `isFirst` 파라미터 추가 후 첫 카드가 아니면 `← 이전` 버튼 렌더 |
| `internal/bot/study_flow_test.go` | 마지막 카드 2버튼(`prev`+`finish`) assertion 갱신, `rowCallbackData` helper 추가, `TestStudyFlowPrevNavigation` 왕복 시나리오 추가 |

## 결정 사항

- **prev는 학습 상태를 변경하지 않는다** — `MarkStudied` 없이 `GetOwned`로 state만 로드 후 `currentOrder-1` 카드를 다시 렌더. 되돌아가도 `StudiedAt`은 유지된다.
- **첫 카드(idx==0)에는 이전 버튼을 렌더하지 않는다** — `currentOrder-1`이 음수가 되는 경로 자체가 생기지 않음 (`showMaterial`의 0 클램프는 방어선으로 유지).
- **order-1 이동 규약** — 기존 `next`가 `currentOrder+1`로 이동하는 규약의 미러. order 연속성 가정은 기존과 동일.
- 이미 studied인 카드에서 `다음 →`을 다시 누르면 model `MarkStudied`가 already-studied에 대해 no-op(false 반환)이라 중복 마킹 없음.

## 검증

- `make test` 전체 통과 (전 패키지 ok, FAIL 없음)
- `make restart-app` 후 `http://localhost:8080/health` → `{"status":"healthy"}`
