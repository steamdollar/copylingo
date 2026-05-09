# Phase 2.5: Service 레이어 리팩토링 및 단위 테스트 완결

## 배경 및 목적
`internal/service/` 의 모든 service struct가 `repository` 패키지의 concrete 타입을 직접 참조하던 구조를 개선하여, DB 의존성 없이 독립적인 단위 테스트가 가능한 구조로 리팩토링하고 테스트 커버리지를 확보했습니다.

## 주요 작업 내용

### 1. 인터페이스 기반 리팩토링 (Interface Injection)
- **대상 서비스**: `SRSService`, `GraderService`, `SessionBuilderService`, `AnalyzerService`, `UserService`, `HandwritingService`
- **변경 사항**: 
    - 각 서비스 상단에 unexported 로컬 인터페이스(`questionQuerier`, `graderUserRepo` 등) 정의.
    - 구조체 필드 및 생성자 파라미터를 인터페이스 타입으로 교체.
    - `internal/repository` 패키지에 대한 직접적인 의존성을 제거하여 계층 간 결합도 완화.

### 2. Mock 기반 단위 테스트 구축
- 외부 라이브러리 없이 각 테스트 파일 내에 수동 Mock struct를 구현하여 의존성 주입.
- **주요 테스트 항목**:
    - **SRS**: SM-2 알고리즘 정합성 검증 (Repetitions, IntervalDays, EaseFactor 변화).
    - **Grader**: 객관식/주관식(LLM) 채점 로직 및 세션 완료 후 Streak 업데이트 로직.
    - **SessionBuilder**: 오전/오후/복습 세션의 문항 믹스 비율 및 생성 로직.
    - **Analyzer**: 사용자 통계 계산 및 취약 구간(정답률 60% 미만) 필터링 로직.
    - **Handwriting**: Mini App 제출물에 대한 권한 검증 및 문항 타입 유효성 체크.

### 3. 실패 경로(Error Path) 테스트 보강
- Repository 또는 외부 서비스(LLM, SRS) 호출 실패 시의 에러 전파 및 후속 로직 중단 여부 검증.
- `UpdateSRS`, `RecordAnswer`, `IncrementServed`, `CreateSessionQuestions` 등 주요 DB 작업 실패 시나리오 추가.

## 검증 결과

- **컴파일**: `go build ./...` 성공
- **테스트**: `go test ./internal/service` 성공 (모든 서비스 및 실패 경로 테스트 포함)

```bash
$ go test ./internal/service
ok      github.com/lsj/copylingo/internal/service       0.007s
```

## 향후 과제
- `showQuestion` 핸들러의 DB hit 개선 (JOIN 또는 Cache 도입).
- Phase 2.4: 아티클 요약 및 AI 대화 시나리오 구현 진행.
