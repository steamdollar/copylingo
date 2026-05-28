# SessionBuilder 카테고리 랜덤 배분 로직 구현

## 배경
기존 `SessionBuilder`는 새 문항을 가져올 때 난이도(`difficulty`) 정렬에만 의존했다. 이로 인해 가나(Difficulty 1) 문항이 모두 소진될 때까지 단어(Difficulty 2) 문항이 노출되지 않는 문제가 발생했다. 또한, 사용자의 레벨에 관계없이 정적인 정렬 순서를 따르는 한계가 있었다.

## 변경 사항

### 1. 도메인 모델 확장 (`internal/model/question.go`)
- `QuestionCategory` 상수에 `CategoryKana` ("kana")와 `CategoryHandwriting` ("handwriting")를 추가하여 서비스 레이어에서 명시적으로 카테고리를 다룰 수 있게 함.

### 2. 세션 빌더 리팩토링 (`internal/service/session_builder.go`)
- **Random Slot Relay 알고리즘 도입**:
  - `kana` -> `handwriting` -> `vocabulary` -> `grammar` 순으로 카테고리를 순회.
  - 각 카테고리마다 세션 내 잔여 슬롯 한도 내에서 랜덤하게 문항 수(`alloc`)를 할당 (카테고리별 최대 6개).
  - DB에 해당 카테고리/레벨 문항이 부족할 경우, 가져온 만큼만 차감하고 남은 슬롯을 다음 카테고리로 이월.
  - 마지막 단계에서 빈 카테고리(`""`)로 쿼리하여 남은 슬롯을 최대한 채움 (Fallback).
- **레벨 기반 필터링 유지**: DB 쿼리 시 사용자의 `ProficiencyLevel`을 전달하므로, N1 유저 등 상급자에게는 DB에 존재하지 않는 `kana` 문항이 자연스럽게 배정되지 않음.

### 3. 단위 테스트 업데이트 (`internal/service/session_builder_test.go`)
- `GetNewQuestions`가 여러 번 호출되는 구조에 맞춰 Mock 로직 수정.
- 최종적으로 수집된 문항의 총합이 세션 설정치와 일치하는지 검증하는 방식으로 테스트 케이스 보강.

## 주요 결정 사항
- **랜덤성 보장**: `math/rand`를 사용하여 세션마다 문항 구성 비율이 달라지도록 설계함.
- **확장성**: `defaultCategoryOrder` 배열만 수정하면 향후 새로운 카테고리(청해, 독해 등)도 쉽게 릴레이 체인에 추가 가능.
- **하위 호환성**: 기존의 `GetNewQuestions` 인터페이스를 그대로 유지하면서 서비스 로직만 개선하여 리포지토리 레이어의 변경을 최소화함.

## 검증 결과
- `go test ./internal/service/...`: PASS
- `go build ./...`: SUCCESS
- 수동 확인: N5 유저 세션 생성 시 가나와 단어가 무작위 비율로 섞여서 생성되는 로직 확인.
