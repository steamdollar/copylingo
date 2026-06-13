# golangci-lint v2 설정 및 commit-time 자동 포맷 도입

> 작성일: 2026-06-12

## 배경

`make lint`는 `golangci-lint run ./...`을 부르고 있었지만 정작 `.golangci.yml` 설정도, golangci-lint 바이너리도 없었다.
사용자의 1차 요구는 "코드가 옆으로 과하게 퍼지는 것(라인 폭)이 불편하다"였고, gofmt/goimports/gopls는 라인 길이 줄바꿈을 해주지 않는다.
추가로 사용자는 "별도 명령 실행 없이 코드만 치면 자동으로 정리되길" 원했다.

## 결정

자세한 의사결정·트레이드오프는 [docs/adr/ADR_from_21_to_40.md](../../adr/ADR_from_21_to_40.md) **ADR-025** 참조.

- 긴 라인은 `golines` 포매터로 **자동 줄바꿈**(max-len 120), 못 줄이는 잔여분은 `lll` 린터가 보고.
- 자동화 시점은 **save-time이 아니라 commit-time** — 에디터 독립적이고 재현 가능한 git pre-commit hook 방식 채택.
- 린터는 실무 표준 세트(standard + revive/gocritic/gocyclo/misspell/errorlint/bodyclose/unconvert/nakedret/nolintlint/lll).

## 변경 파일

- `.golangci.yml` (신규)
  - golangci-lint **v2** 스키마. `linters`(진단) / `formatters`(자동 적용) 분리.
  - `formatters`: gofmt + goimports(local-prefix `github.com/lsj/copylingo`) + golines(max-len 120, tab-len 4).
  - `linters.settings.lll.line-length: 120`, gocyclo min 25, nakedret 30, nolintlint 위생 규칙.
  - 테스트 파일은 lll/gocyclo/errcheck/bodyclose 완화.
- `scripts/git-hooks/pre-commit` (신규)
  - staged `.go` 파일만 `golangci-lint fmt` 후 `git add` 재-stage.
  - golangci-lint 미설치 시 `make lint-install` 안내 후 종료.
- `Makefile`
  - `fmt`: `golangci-lint fmt` + `run --fix`.
  - `lint-install`: golangci-lint v2 설치.
  - `hooks`: `core.hooksPath=scripts/git-hooks` 설정으로 hook 활성화.

## 검증

- `golangci-lint config verify` 통과 (스키마 유효).
- `golangci-lint fmt --diff`로 긴 시그니처/호출이 인자별 줄바꿈됨을 확인 (dry-run, 코드 미변경).
- pre-commit hook end-to-end: 147자 시그니처를 가진 임시 파일을 stage → hook 실행 → 줄바꿈 적용 + 재-stage 확인 후 임시 파일 정리.
- golangci-lint v2.12.2 설치 (`go install .../v2/cmd/golangci-lint@latest`).
- `make test` 미실행: 런타임 동작 변경 없는 개발 툴링 설정 추가 작업.

## 미해결 / 후속

- 현재 작업 트리에 기존 컴파일 에러(`study_active_session_repo_test.go`가 미정의 `loadStudyActiveSessionQuery` 참조)가 있어 해당 패키지는 타입체크에서 막혀 전체 린트 노이즈 규모는 미확인. 해당 함수 구현 후 `make lint`로 전체 노이즈 점검 필요.
- `core.hooksPath`는 로컬 git 설정이라 다른 클론 환경에서는 `make hooks` 1회 실행이 필요.
