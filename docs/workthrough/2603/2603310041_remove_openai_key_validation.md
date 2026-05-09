# OPENAI_API_KEY 검증 완화 기록

- **날짜**: 2026-03-31 00:41
- **목적**: OpenAI 연동(AI 생성 기능)이 불필요하거나 키가 제공되지 않은 환경에서도 서버가 구동되고, 텔레그램 세션 생성 및 푸시 기능 등 타 핵심 로직이 정상 동작하도록 사전 검증 로직 해제.
- **대상 파일**: `internal/config/config.go`

## 변경 및 확인 상세

### 1. 이슈 및 원인
기존 `config.go` 내부의 `validate()` 메서드에서 `OPENAI_API_KEY` 환경변수가 누락되면 패닉(`log.Fatalf`)이 발생하여 애플리케이션 시작을 원천 차단함. 그러나 현재 AI 연동부는 Phase 2.3 마일스톤 대상으로 PassThrough 상태이므로, 강제 체크가 불필요한 상태였음.

### 2. 적용한 내용
- `internal/config/config.go`: `OPENAI_API_KEY` 누락 시 에러를 반환하지 않고, `log.Println("[WARN] openai.api_key is not set...")` 형태의 상태 경고 로그만 남기도록 Soft check로 변경.
- **영향도**: 현재 텔레그램 봇 푸시 및 학습 세션 생성 로직은 AI 개입이 없어도 정상 구동되므로, 향후 기능 추가/On-Off 토글 시에도 대비 가능한 `Graceful Degradation` 구조 마련.

### 3. 검증 결과
- `go test ./...` 및 앱 빌드 (go build ./cmd/server) 성공.
- 사용자가 더 이상 임의의 `.env` 환경변수에 `COPYLINGO_OPENAI_API_KEY=dummy` 값을 강제로 주입하지 않아도 서버 구동이 가능함.
