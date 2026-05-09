# SessionFlow editMessageID 명시화

- **일시**: 2026-05-08 15:21
- **작업 범위**: `internal/bot/session_flow.go`, `docs/ADR.md`, `STATUS.md`
- **목표**: Telegram `messageID=0` sentinel 사용을 제거하고, 기존 메시지 수정과 새 메시지 전송 의도를 명확히 분리한다.

## 배경

`showQuestion`은 Telegram 세션 메시지를 렌더링할 때 두 가지 동작을 수행한다.

- 기존 봇 메시지가 있으면 해당 메시지를 `EditMessage`로 수정한다.
- 편집할 봇 메시지가 없거나 새 메시지 UX가 필요하면 `SendMessage` 계열로 새 메시지를 보낸다.

기존 코드는 이 차이를 `messageID int` 하나로 표현했다. `messageID > 0`이면 실제 Telegram 메시지 ID로 보고 edit하고, `messageID == 0`이면 새 메시지를 보내는 sentinel로 해석했다. 이 방식은 동작은 하지만, `0`이 실제 Telegram 메시지 엔티티처럼 읽혀 코드 리뷰 중 혼란을 만들었다.

## 변경 내용

`showQuestion`의 세 번째 인자를 `messageID int`에서 `editMessageID *int`로 변경했다.

- `editMessageID != nil`: 해당 Telegram 메시지를 수정한다.
- `editMessageID == nil`: 새 Telegram 메시지를 보낸다.

같은 파일의 `processAnswerText`도 동일한 의미의 `messageID=0` sentinel을 쓰고 있어 함께 정리했다.

- 객관식 callback 답변은 버튼이 붙은 봇 메시지를 편집해야 하므로 `editMessageID`를 전달한다.
- 텍스트 답변은 사용자 메시지로 들어오므로 편집할 봇 문제 메시지가 없어 `nil`을 전달한다.
- 손글씨 Mini App의 "제출 후 다음 문제" 흐름은 기존 메시지 버튼을 제거한 뒤, 다음 문제를 새 메시지로 보내기 위해 `nil`을 전달한다.

## 손글씨 Mini App 흐름

손글씨 문항은 일반 객관식과 다르게 Mini App HTTP API를 통해 제출된다. 따라서 Web App 버튼이 붙은 메시지를 다음 문제로 재사용하면 사용자가 방금 어떤 손글씨 문항을 제출했는지 맥락이 사라질 수 있다.

이번 변경에서는 이 의도를 코드 주석으로 명시했다.

- 같은 손글씨 제출 결과로 다음 문제를 중복 진행하지 못하게 기존 메시지의 버튼을 제거한다.
- Mini App 제출 흐름의 메시지 히스토리를 보존하기 위해 다음 문제는 새 Telegram 메시지로 렌더링한다.
- Web App 버튼이 붙은 손글씨 문항은 별도 메시지로 두고, 이전 메시지는 짧은 안내 문구로 축약한다.

## 의사결정

`docs/ADR.md`에 ADR-012를 추가했다.

- 결정: Bot 세션 메시지 렌더링은 nullable `editMessageID`로 분기한다.
- 이유: Telegram 메시지 ID와 렌더링 모드를 `0` sentinel로 섞지 않기 위해서다.
- 보류한 대안: 별도 enum(`QuestionRenderMode`)은 더 명시적이지만 현재 변경 범위에는 과하다고 판단했다.

## 검증

`go test ./...`를 실행해 전체 테스트 통과를 확인했다.
