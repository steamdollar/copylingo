# Workthrough: `/help`, `/exit` 명령어 구현 및 정비

- **ID**: `2605271445_help_exit_commands`
- **날짜**: 2026-05-27
- **작업자**: Gemini CLI

## 1. 개요

사용자가 현재 활성화된 텍스트 입력 대기 상태(주관식 문제 등)를 명시적으로 종료할 수 있도록 `/exit` 명령어를 추가하고, `/help` 도움말 텍스트를 최신화했습니다. 또한 테스트 코드 작성을 위해 `Bot` 구조체를 리팩토링했습니다.

## 2. 주요 변경 사항

### 2.1. `internal/config/constants.go`
- `CommandExit = "exit"` 상수 추가

### 2.2. `internal/bot/handler.go` (리팩토링 및 구현)
- **BotAPI 인터페이스 도입**: Telegram API(`tgbotapi.BotAPI`)에 대한 직접 의존성을 인터페이스로 추상화하여 단위 테스트에서 모킹이 가능하도록 개선했습니다.
- **Bot 구조체 변경**: `api` 필드를 `BotAPI` 인터페이스로, `rdb` 필드를 `redis.Cmdable` 인터페이스로 변경했습니다.
- **`/exit` 핸들러 구현**: `handleExit` 메서드를 추가하여 Redis에 저장된 `user:%d:active_question` 키를 삭제하고 취소 안내 메시지를 보내도록 했습니다.
- **`/help` 정비**: `/exit` 명령어에 대한 설명을 "현재 입력 취소 (세션은 보존, /menu 에서 재개)"로 업데이트했습니다.

### 2.3. `internal/bot/handler_test.go` (신규)
- `mockBotAPI` 및 `mockRedis`를 사용하여 `/exit` 명령어 실행 시 Redis 키가 삭제되고 올바른 메시지가 전송되는지 검증하는 단위 테스트를 추가했습니다.

## 3. 검증 결과

### 3.1. 단위 테스트
- `TestHandleExit`: **PASS** (Redis 키 삭제 확인 및 메시지 텍스트 일치 확인)
- 전체 테스트: `make test` 실행 결과 모든 테스트 **PASS**

### 3.2. 수동 검증 시나리오 (예정)
- 주관식 문제 진행 중 `/exit` 입력 -> "입력을 취소했습니다" 메시지 확인.
- 이후 `/menu` -> "학습하기" 선택 시 이전 문제부터 정상적으로 이어지는지 확인.

## 4. 관련 문서
- [docs/todos/help_exit_commands.md](../todos/help_exit_commands.md)
