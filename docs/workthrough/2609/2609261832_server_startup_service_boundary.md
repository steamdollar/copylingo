# 서버 시작·콘텐츠 저장의 서비스 경계 정리

2026-09-26 18:32 서버 시작 경계, 18:37 콘텐츠 저장 경계, 20:02 서버 파일 분리를 통합한 기록이다.

## 서버 시작 경로 — ADR-057

- [ADR-057](../../adr/ADR_from_41_to_60.md#adr-057-현재-서버-시작-경로에서는-서비스만-조립-결과로-전달한다)에 따라 `initApp` 안에서 생성한 저장소 묶음을 `run`으로 반환하지 않는다.
- `cmd/server/main.go`: `initApp`에서 서비스와 봇 핸들러만 받고, `startWorkers`에 저장소를 전달하지 않는다.
- `cmd/server/server.go`: 자동 콘텐츠 수집 cron이 없는 현재 경로에서 미사용 파이프라인 초기화를 중단했다. `initPipeline`과 `Scheduler.collectContent` 등 관련 코드는 향후 재연결을 위해 유지했다. 사용하지 않는 설정 인자도 함께 제거했다.
- `STATUS.md`, `docs/adr/ADR_from_41_to_60.md`: 결정과 작업 완료를 기록했다.

## 콘텐츠 저장 경로 — ADR-058

- [ADR-058](../../adr/ADR_from_41_to_60.md#adr-058-콘텐츠-수집의-저장-규칙은-서비스가-소유한다)에 따라 파이프라인이 저장소를 직접 받지 않도록 했다. 자동 수집은 계속 비활성이다.
- `internal/service/content.go`, `internal/service/services.go`: URL 중복 확인, 콘텐츠 저장, 개별 실패 집계를 `ContentService`로 옮기고 서비스 묶음에 추가했다.
- `internal/pipeline/interfaces.go`: 저장 결과 타입을 서비스 결과에 연결했다. 저장소를 직접 사용하던 `internal/pipeline/saver.go`의 `ContentSaver`는 제거했다.
- `cmd/server/server.go`: 재도입용 `initPipeline`이 콘텐츠 서비스를 `Saver`로 등록하게 했다. `internal/pipeline/pipeline_test.go`의 저장 규칙 테스트는 `internal/service/content_test.go`로 옮겼다.
- `docs/ARCHITECTURE.md`, ADR, `STATUS.md`에 계층 경계와 이전 ADR의 복구 경로를 반영했다.

## 서버 파일 분리

- `cmd/server/infra.go`: `initInfra`, `initDB`, `initRedis`를 옮겨 DB·Redis 연결 생성과 정리를 모았다.
- `cmd/server/router.go`: `setupRouter`를 옮겨 HTTP 미들웨어, `/health`, Mini App 경로 등록을 모았다.
- `cmd/server/server.go`: 서비스·파이프라인 조립, worker 시작, scheduler와 HTTP 서버 실행·종료를 남겼다. 파일 분리 시점에는 202줄에서 110줄로 줄었다.

모두 같은 Go 패키지 안의 이동으로 호출 순서·인자·연결 확인·health 응답은 바꾸지 않았다. 별도 추상화나 의존성은 추가하지 않았다. 서버 초기화 전체 단순화의 다음 단계는 [ADR-059](../../adr/ADR-059_architecture_simplification.md)에 있다.

## 검증

- 세 작업 각각 `make test`, `git diff --check` 통과. 파일 분리에는 `gofmt`를 적용하고 기존 테스트로 컴파일·동작을 확인했다.
- 각 작업 후 `make restart-app`과 `http://localhost:8080/health` 정상 응답을 확인했다.
