# Walkthrough: cmd/server/main.go Refactoring (Run Pattern)

`cmd/server/main.go`의 비대해진 로직을 `server.go`로 분리하고, 테스트 용이성을 위해 `run()` 패턴을 도입했습니다.

## Changes Made

### 1. File Separation
초기화 및 서버 설정 관련 헬퍼 함수들을 신규 파일로 분리했습니다.
- **cmd/server/main.go**: 프로그램의 Entry point 역할만 수행.
- **cmd/server/server.go**: 실제 초기화 로직 및 라우터 설정 포함.

### 2. Run Pattern Introduction
- `main` 함수의 모든 로직을 `run() error` 함수로 이동했습니다.
- `main` 함수는 `run()`의 실행 결과에 따른 에러 처리만 담당합니다.
- 이 구조를 통해 향후 통합 테스트 시 `run()` 함수를 직접 호출하여 테스트 환경에 맞는 설정을 주입하기 용이해졌습니다.

### 3. Extracted Functions (Moved to server.go)
- `initDB`, `initRedis`, `initScheduler`, `startHTTPServer`, `waitForShutdown`, `setupRouter`

### 4. Verification Results
- `go build ./cmd/server` 명령을 통해 정상 컴파일 확인 완료.

## Related Files
- [main.go](file:///home/lsj/project/copylingo/cmd/server/main.go)
- [server.go](file:///home/lsj/project/copylingo/cmd/server/server.go)
