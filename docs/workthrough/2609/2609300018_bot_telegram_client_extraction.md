# Bot Telegram 호출을 telegramClient로 분리 (ADR-059 §8 A단계)

## 변경

- `internal/bot/telegram_client.go`에 `telegramClient`를 추가했다. Telegram Bot API 호출(update polling, 메시지 전송·수정, 음성 전송, callback 응답, chat action)을 이 타입만 수행한다.
- `Bot`의 `api BotAPI` 필드를 `telegram *telegramClient`로 바꾸고, `SendMessage`·`SendMessageWithKeyboard`·`SendMessageWithReplyMarkup`·`SendVoiceFileID`·`SendVoiceBytes`·`EditMessage`·`EditMessageReplyMarkup`·`ClearInlineKeyboard`를 `telegramClient`로 옮겼다.
- `SessionFlow`·`StudyFlow`에 `telegram` 필드를 추가했다. Flow는 전송할 때 `*Bot`을 거치지 않는다. 서비스·저장소 접근은 아직 `bot` 필드를 통하며 B·C단계에서 정리한다.
- 흩어져 있던 `api.Request(tgbotapi.NewCallback...)`·`NewChatAction` 직접 호출 15곳을 `AnswerCallback`·`AnswerCallbackAlert`·`SendChatAction`으로 바꿨다.
- Mini App의 `TelegramMessenger` 계약을 유지하려고 `Bot.EditMessageReplyMarkup`은 `telegramClient`로 위임하는 메서드로 남겼다. C단계에서 Flow의 좁은 계약으로 옮긴다.
- 주관식 AI 채점 불가 안내는 raw `tgbotapi.NewMessage` 대신 `SendMessage`(HTML parse mode)로 보낸다. 문구에 HTML 특수문자가 없어 표시 결과는 같다.

## 커밋 분리

- bot 테스트 6개 파일은 기존에 goparams 레이아웃이 적용되지 않은 상태였다. 규칙대로 수정 파일에 goparams를 적용하면서 생긴 포맷 변경은 별도 `style(bot)` 커밋으로 분리했다. 리팩터 커밋에는 의미 변경만 남는다.

## 검증

- 수정한 Go 파일에 gofmt·`goparams` 적용.
- `make test`: 통과.
- `make restart-app`: 성공, `http://localhost:8080/health` healthy 확인.
