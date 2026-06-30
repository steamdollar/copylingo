# 세션 빌드 후 학습 팁(Tips) LLM 생성 파이프라인

## 작업 범위

`docs/todos/tip_scheduler_generation.md` 구현. scheduler가 morning/evening 세션을
빌드·푸시한 뒤, 활성 사용자에서 등장하는 distinct `(language, level)` 페어 단위로
LLM을 호출해 tips 잔고를 최대 50개까지 점진 생성하는 파이프라인을 추가했다.

[ADR-015](../../ADR.md) 의 tips 생성 파이프라인 구현 단계에 해당한다. 스키마/모델/
repository(`Create`/`ListActive`/`CountActive`)는 기존에 완료되어 있어 생성 로직만 얹었다.

## 결정 사항

- 잔고 임계치 `TipBucketTarget = 50`, 1 사이클 생성 수 `TipGeneratePerCycle = 3`,
  prompt 버전 `TipPromptVersion = "v1"` — 모두 코드 상수.
- 카테고리 선택은 **random** (`math/rand`). round-robin의 last-used 추적은 과한
  복잡도라 ADR-015에서 배제됨. 7개 카테고리에 자연 분산을 기대.
- 실패 처리: LLM 호출 실패 시 해당 페어만 skip(에러 전파 → scheduler가 로그 후 continue),
  retry 없음. 다음 세션 사이클(12시간 후)이 자연 재시도 역할.
- tip top-up은 세션 빌드/푸시와 **분리**됨. tip 생성 실패가 세션 푸시 결과
  (`failures` 카운트/반환 에러)에 전파되지 않게 별도 메서드 `topUpTips`로 격리.
- `source_model`은 `cfg.LLM.Model`, `source_prompt_ver`는 `&TipPromptVersion`,
  `is_active=true`로 저장.

## 문서 대비 보정 (triage 지시대로)

- 문서의 `external.LLMClientIface`는 코드에 없어 `external.LLMClient`를 기준으로 작업.
- 로거는 문서의 `*log.Logger`가 아니라 scheduler/external이 쓰는 **`log/slog`** 사용
  (`slog.InfoContext`/`WarnContext`).
- `TipRepositoryIface`는 코드에 없어, `tip_generator.go` 내부에 필요한 메서드만 갖는
  **인라인 인터페이스**(`tipGeneratorRepo`/`tipGeneratorLLM`)로 선언. 기존 `tip.go`의
  `tipRepository` 인라인 인터페이스 패턴과 동일.

## 베이스 차이로 인한 비자명 판단 (주의)

작업 worktree의 base에서 `external.LLMClient`의 채점 메서드는 **`GradeResult` 리팩터
이전 시그니처**(`GradeAnswer/GradeHandwriting → (bool, string, error)`)였다. triage가
전제한 "`(GradeResult, error)`로 리팩터된 base"와 달랐다. 채점 시그니처는 지시대로
건드리지 않았다.

이로 인해 핵심 충돌이 발생했다: triage 문서는 "`LLMClient` 인터페이스에 `GenerateTips`
추가"를 지시했으나, `internal/service/llm_test.go`의 `mockLLMClient`가 `LLMClient`
인터페이스 전체를 구현 중이라, 인터페이스를 확장하면 그 mock이 깨진다. 그 테스트 파일은
**다른 병렬 agent 소유라 수정 금지** 경계에 묶여 있다.

→ 경계(병렬 충돌 방지)가 hard boundary이므로 우선했다. **`GenerateTips`를 인터페이스에
넣지 않고 `*DefaultLLMClient`의 메서드로만** 두고, `TipGenerator`는 자체 인라인
인터페이스 `tipGeneratorLLM`(GenerateTips 단일 메서드)에만 의존하게 했다. wiring은
`services.go`에서 `external.NewLLMClient`가 반환한 `LLMClient`를 `*DefaultLLMClient`로
타입 단언해 주입(`newTipGeneratorFromClient`). 단언 실패 시 LLM은 nil로 두고,
`TopUpBucket`이 `ErrAIConfigMissing`을 반환(scheduler가 페어 skip). typed-nil 함정을
피하려 nil 정규화를 생성자 helper 한 곳에 모았다.

결과적으로 기존 grading mock(`mockLLMClient`, `mockLLM`)은 전혀 영향받지 않는다.

## 변경 파일

- `internal/external/llm.go` — `GeneratedTip` 타입, `GenerateTips`(concrete 메서드),
  tip 생성 상수/프롬프트 빌더, `extractJSONArray`(펜스 제거) 추가. 채점 경로 무변경.
- `internal/service/tip_generator.go` — 신규. `TipGenerator` + 인라인 인터페이스 2종 +
  상수 + random 카테고리 선택 + `newTipGeneratorFromClient`.
- `internal/service/services.go` — `Services.TipGenerator` 필드 + wiring. LLM client를
  LLMService와 공유(중복 생성 제거).
- `internal/scheduler/scheduler.go` — `buildAndPushSessions` 사용자 루프 종료 후
  `topUpTips` 호출 + `langLevelPair`/`distinctLangLevelPairs` 헬퍼.
- `internal/service/tip_generator_test.go` — 신규. 5케이스.

## LLM 호출량 메모

한 scheduler 사이클(morning 또는 evening)의 tip 생성 LLM 호출 수 = **distinct
`(language, level)` 페어 수**. 사용자 수와 무관(페어당 최대 1회). 잔고 50 도달한 페어는
`CountActive` 후 즉시 return하여 호출하지 않는다. `cfg.LLM.RPM`/RPD 관리는 본 범위 밖.

## 검증

- `go build ./...` 통과, `go vet ./...` 클린.
- `go test ./internal/service ./internal/scheduler ./internal/external` 전부 통과.
- 전체 `go test ./...` 회귀 없음.
- tip_generator 5케이스: 잔고 full시 LLM 0회 / 잔고 미달시 LLM 1회+Create N회 /
  빈 배열시 Create 0회·에러 아님 / LLM 에러 전파 / 개별 Create 실패시 나머지 진행.
- 실제 LLM API 호출 없음(전부 mock).
