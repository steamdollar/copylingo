# VS Code 저장 시 Go 매개변수별 줄바꿈

## 요구사항과 적용 범위

줄 길이 기준만으로는 짧은 함수 선언·호출의 가독성을 해결하지 못하므로, 매개변수·인자가 2개 이상인 목록을 각각 별도 줄에 배치한다. 반환값 목록과 0~1개인 목록은 이 규칙의 대상에서 제외한다. 실제 코드는 파일 저장 시 적용하며 저장소 전체 일괄 포맷은 실행하지 않는다.

## 변경

- `cmd/dev/goparams/main.go`: Go 표준 라이브러리로 구문을 읽고 대상 목록에 개행을 적용한 뒤 gofmt 결과를 stdout으로 출력한다. 입력이 유효하지 않으면 오류로 종료하며 부분 결과를 출력하지 않는다.
- `cmd/dev/goparams/main_test.go`: 중첩 호출·주석·문자열·variadic·generics와 재실행 시 결과가 변하지 않는 성질을 검증한다.
- 로컬 `.vscode/settings.json`: 기존 설정을 보존하고 Go custom formatter와 저장 시 포맷을 연결한다. import 정리는 기존 Go 확장의 organize imports 기능을 사용한다.
- `docs/CONVENTIONS.md`, ADR-025: 길이와 무관한 매개변수 배치 및 저장 시 적용 규칙을 기록한다.

## 실행 및 재설정

프로젝트 루트에서 도구를 빌드한다. 소스 변경 후에도 같은 명령으로 갱신한다.

```bash
go build -o bin/goparams ./cmd/dev/goparams
```

현재 로컬 VS Code 설정:

```json
{
  "go.formatTool": "custom",
  "go.formatFlags": [],
  "go.alternateTools": {
    "customFormatter": "/home/lsj/work/copylingo/bin/goparams"
  },
  "[go]": {
    "editor.defaultFormatter": "golang.go",
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": "explicit"
    }
  }
}
```

`bin/`과 `.vscode/`는 기존 `.gitignore`에 따라 로컬 전용이다. 다른 체크아웃에서는 바이너리를 빌드하고 `customFormatter`의 절대 경로를 해당 위치로 바꾼다. Go 파일을 저장하거나 `Format Document`를 실행하면 적용된다. 설정을 즉시 인식하지 못하면 `Developer: Reload Window`를 실행한다.

기존 golangci-lint·커밋 hook은 유지한다. 이번 개발 도구 추가는 Go 서버 런타임 변경이 아니므로 앱·DB·Redis 재시작은 필요하지 않다.

## 검증

- 설치된 VS Code Go 확장 0.56.1 소스에서 custom formatter에 문서 전체를 stdin으로 전달하고 stdout을 편집 결과로 사용하는 계약을 확인했다.
- `go test ./cmd/dev/goparams` 통과. 3단계 중첩 호출의 위치 계산 회귀와 반복 실행 시 동일 결과를 확인했다.
- 프로젝트 Go 파일 174개를 메모리에서 포맷하고 다시 적용해, 오류 없이 같은 결과가 나오는 것을 확인했다. 기존 프로젝트 파일은 쓰지 않았다.
- 실행 파일의 짧은 `Add(left int, right int)` 선언·호출이 길이와 관계없이 각각 줄바꿈되고, 기존 `golangci-lint fmt --stdin`을 이어 적용해도 유지되는 것을 확인했다. 잘못된 입력은 nonzero 종료와 빈 stdout을 확인했다.
- `make test` 통과. 전용 DB 환경이 필요한 PostgreSQL 통합 테스트 6개는 기존 조건에 따라 skip됐다. 로그: `/tmp/copylingo-paramformat-test-lei_x2rz.log`.
- 신규 도구의 두 소스 파일에는 자체 포맷을 적용한 뒤 실행 파일을 다시 빌드하고 표적 테스트를 통과했다. VS Code GUI에서 실제 저장 이벤트를 직접 조작하지는 않았으며, 확장 소스의 호출 계약과 같은 stdin/stdout 동작까지 검증했다.

Go 토큰으로 목록 경계를 찾으므로 문자열·주석 안의 쉼표는 구분자로 처리하지 않는다. 같은 타입을 공유하는 `a, b int`도 이름별로 줄을 나누되 타입 공유는 유지한다. 주석은 gofmt에 의해 줄 위치가 정리될 수 있어 안정된 결과까지 포맷한 뒤 반환한다. 파일별 편집은 단순한 방식으로 구현했으며, 저장 지연이 체감될 때 단일 편집 패스로 바꾸는 것을 검토한다.

## 복구

로컬 `.vscode/settings.json`에서 이번에 추가한 `go.formatTool`, `go.formatFlags`, `go.alternateTools`, `[go]` 설정을 제거하면 이전 설정으로 돌아간다. 포맷터는 저장소 파일을 직접 쓰지 않으며 저장 시 편집 결과는 VS Code의 일반 Undo로 되돌릴 수 있다.

## 후속: 에이전트 작성 코드에도 같은 포맷 적용

- 사용자 저장 전에 에이전트가 포맷을 완료하도록 `AGENTS.md` Case B 검증 단계에 필수 절차를 연결했다. 상세 실행법은 `docs/CONVENTIONS.md`의 `Agent Go formatting`에 둔다.
- 모든 에이전트가 직접 수정한 Go 파일만 최신 `goparams`로 포맷한 뒤 테스트·인계하도록 명시했다. 이후 코드 수정 시 재적용하고, 포맷 오류 시 원본을 잘라 쓰지 않는 실행 예시를 추가했다.
- 문서 변경만 있으므로 `make test`와 런타임 재시작은 생략했다. 문서의 실행 예시를 임시 Go 파일에 실행해 정상 포맷·반복 적용·잘못된 입력 시 원본 보존을 확인했고, `git diff --check`도 통과했다.
