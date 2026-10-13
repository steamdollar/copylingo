# Quiz 결과 메시지가 Telegram 429로 사라져 세션이 멈춘 건

- 날짜: 2026-10-11
- 결정: [ADR-070](../../adr/ADR_from_61_to_80.md#adr-070-telegram-bot-api-호출-실패는-telegramclient-한-곳에서-log한다)

## 증상

- Quiz 세션 574의 11/15번(`素晴らしい` 읽기, 텍스트 답) 이후 bot이 답을 하지 않았다.
- 서버 `/health`, DB, Redis, polling은 모두 정상이었다. slog log에는 WARN/ERROR가 없었다.

## 원인

```
10:11:04  사용자 "すばらしい" 입력 → 정답 채점, working set 저장 성공 (11/15)
10:11:04  결과 메시지(✅ 정답 + [다음 문제 →]) sendMessage → 429 Too Many Requests
10:11:07~18  getUpdates도 429, 502 Bad Gateway (Telegram 쪽 일시 장애)
10:17:28  같은 답 재입력 → 이미 답한 문제라 대기 중인 텍스트 입력이 없어 무응답
```

- `SessionFlow.renderQuizAnswerResult`가 전송 error를 버렸다. 그래서 log가 없었다.
- 텍스트 답을 받으면 active question을 먼저 지운다. 결과가 전달되지 않아도 같은 답을 다시 보낼 경로가 없다.
- 일반 텍스트를 무시하는 경로(`handleMessage` → `HandleTextInput`이 false)에도 log가 없었다.

## 진단에 쓴 곳

- slog JSON log(`logs/copylingo-YYYY-MM-DD.jsonl`)에는 단서가 없었다.
- tmux `copylingo` session의 서버 stdout에 tgbotapi Debug 출력(`Endpoint: sendMessage, response: ...429`)이 남아 있었다: `tmux capture-pane -p -J -t copylingo:0 -S -3000`.
- 진행 상태는 Redis `session:{id}:working_set`의 `current_index`와 `answered_count`로 확인했다.

## 복구 (재시작 없이)

- stdout에 찍힌 실패 sendMessage의 params(text와 `q:574:next:10` 버튼)를 Bot API로 그대로 다시 보냈다.
- 사용자가 [다음 문제]를 누르자 서버가 이어서 진행했다.

## 후속

- 실패 log 추가(ADR-070)는 끝났다.
- 결과 미전달 시 복구 경로(ADR-071)도 끝났다. 진행 중 Quiz에서 아무 텍스트나 보내면 직전 정오답과 다음 문제를 다시 받는다. 이번 사례는 재입력 한 번으로 풀렸을 것이다.
- 429/5xx 전송 retry는 아직 정하지 않았다.
