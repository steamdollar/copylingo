# 리팩토링 완료: 서비스 계층 개별 의존성 주입 (Individual Injection)

기존에 `repository.Repositories` 구조체를 통째로 주입받던 방식에서, 각 서비스가 실제로 사용하는 개별 Repository만 주입받도록 구조를 개선했습니다.

## 주요 변경 사항

### 1. 서비스 구조 및 생성자 수정
- **GraderService**: `User`, `Question`, `Session`, `SessionQuestion` Repository를 개별적으로 주입받음.
- **AnalyzerService**: `User`, `SessionQuestion` Repository를 개별적으로 주입받음.
- **SessionBuilderService**: `Question`, `Session`, `SessionQuestion` Repository를 개별적으로 주입받음.

### 2. Composition Root 업데이트
- [services.go](file:///home/lsj/project/copylingo/internal/service/services.go)의 `NewServices` 함수에서 각 서비스 생성 시 필요한 Repository 필드만 명시적으로 전달하도록 수정했습니다.
- 이를 통해 서비스 간의 불필요한 의존성 결합을 제거했습니다.

### 3. UserService 분리 (관심사 분리)
- `GraderService`에 존재하던 사용자 관리 로직(`GetUser`, `GetAllUsers`)을 신설된 `UserService`로 이관했습니다.
- **주요 개선 사항:**
    - `bot.Handler`와 `Scheduler`가 더 이상 사용자 조회를 위해 "채점기(Grader)"를 호출하지 않고 전담 서비스인 `UserService`를 사용하게 되었습니다.
    - 각 서비스의 책임(User Management vs Grading)이 명확해졌습니다.

## 검증 결과

### 빌드 테스트
`go build ./...` 명령을 통해 모든 리팩토링이 성공적으로 완료되었으며, 순환 참조나 타이핑 오류가 없음을 확인했습니다.

```bash
$ go build ./...
# 성공 (출력 없음)
```

## 향후 권장 사항
- 현재는 Concrete Repository 타입을 직접 주입하고 있으나, 나중에 단위 테스트(Mocking)가 필요한 시점에 각 Repository를 **Interface**로 전환하면 진정한 의미의 DIP(Dependency Inversion Principle)를 달성할 수 있습니다. 이미 개별 주입 체계가 잡혔으므로 인터페이스 전환은 매우 쉬울 것입니다.
