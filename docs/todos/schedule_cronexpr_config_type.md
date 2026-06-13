# ScheduleConfig cron 필드 custom type 전환

## 배경/목적

현재 [internal/config/config.go](../../internal/config/config.go) 의 `ScheduleConfig` 는 cron expression 을 모두 `string` 으로 들고 있다.

```go
type ScheduleConfig struct {
	ContentCollectCron string `mapstructure:"content_collect_cron"` // 콘텐츠 수집 크론
	MorningBuildCron   string `mapstructure:"morning_build_cron"`   // 오전 세션 빌드 크론
	MorningPushCron    string `mapstructure:"morning_push_cron"`    // 오전 세션 푸시 크론
	StudyPushCron      string `mapstructure:"study_push_cron"`      // 정오 Study 세션 푸시 크론
	EveningBuildCron   string `mapstructure:"evening_build_cron"`   // 오후 세션 빌드 크론
	EveningPushCron    string `mapstructure:"evening_push_cron"`    // 오후 세션 푸시 크론
}
```

실제 사용처는 [internal/scheduler/scheduler.go](../../internal/scheduler/scheduler.go) 의 `cron.AddFunc` 호출이다.

```go
if _, err := s.cron.AddFunc(s.cfg.Schedule.MorningPushCron, func() {
	s.runJob("morning_push", 0, func(ctx context.Context) error {
		return s.buildAndPushSessions(ctx, model.SessionMorning)
	})
}); err != nil {
	// ...
}
```

문제:

- scheduler 설정값이 단순 `string` 이라 config 계층에서 의미가 드러나지 않는다.
- 잘못된 cron expression 이 `Load()` 단계에서 걸러지지 않고 scheduler 등록 시점까지 내려간다.
- `ScheduleConfig` 의 모든 필드가 같은 도메인 타입인데 타입 레벨 표현이 없다.

목표:

- cron expression 전용 custom type 을 도입한다.
- config 로딩 단계에서 non-empty cron expression 을 검증한다.
- scheduler 에서는 명시적으로 string 변환 또는 `String()` 메서드를 사용한다.
- 기존 `config.yaml`, env key, default 값은 유지한다.

## Issue

**Issue 1: ScheduleConfig cron 값의 도메인 타입 부재**

- 파일: `internal/config/config.go`
- 현재 라인: `ScheduleConfig` 정의부
- 영향: cron 값이 plain `string` 으로 노출되어 invalid value 검증과 호출부 의미 표현이 약하다.

## Options

### Option A — recommended: `CronExpr` custom type + config validation

`internal/config/config.go` 에 다음 custom type 을 추가한다.

```go
type CronExpr string

func (c CronExpr) String() string {
	return string(c)
}

func (c CronExpr) IsZero() bool {
	return strings.TrimSpace(string(c)) == ""
}

func (c CronExpr) Validate(name string) error {
	if c.IsZero() {
		return nil
	}
	if _, err := cron.ParseStandard(c.String()); err != nil {
		return fmt.Errorf("%s is invalid cron expression %q: %w", name, c.String(), err)
	}
	return nil
}
```

`ScheduleConfig` 는 다음처럼 바꾼다.

```go
type ScheduleConfig struct {
	ContentCollectCron CronExpr `mapstructure:"content_collect_cron"` // 콘텐츠 수집 크론
	MorningBuildCron   CronExpr `mapstructure:"morning_build_cron"`   // 오전 세션 빌드 크론
	MorningPushCron    CronExpr `mapstructure:"morning_push_cron"`    // 오전 세션 푸시 크론
	StudyPushCron      CronExpr `mapstructure:"study_push_cron"`      // 정오 Study 세션 푸시 크론
	EveningBuildCron   CronExpr `mapstructure:"evening_build_cron"`   // 오후 세션 빌드 크론
	EveningPushCron    CronExpr `mapstructure:"evening_push_cron"`    // 오후 세션 푸시 크론
}
```

`Config.validate()` 에서 schedule validation 을 호출한다.

```go
func (c *ScheduleConfig) validate() error {
	checks := []struct {
		name string
		expr CronExpr
	}{
		{name: "schedule.content_collect_cron", expr: c.ContentCollectCron},
		{name: "schedule.morning_build_cron", expr: c.MorningBuildCron},
		{name: "schedule.morning_push_cron", expr: c.MorningPushCron},
		{name: "schedule.study_push_cron", expr: c.StudyPushCron},
		{name: "schedule.evening_build_cron", expr: c.EveningBuildCron},
		{name: "schedule.evening_push_cron", expr: c.EveningPushCron},
	}
	for _, check := range checks {
		if err := check.expr.Validate(check.name); err != nil {
			return err
		}
	}
	return nil
}
```

그리고 `Config.validate()` 안에서 logging validation 이후 또는 이전에 다음을 추가한다.

```go
if err := c.Schedule.validate(); err != nil {
	return err
}
```

Pros:

- config 타입만 봐도 cron expression 임이 명확하다.
- invalid cron 을 app startup 초기에 fail-fast 할 수 있다.
- `cron.ParseStandard` 로 실제 scheduler library 와 동일한 문법을 검증한다.
- env/config/default key shape 는 유지되어 운영 호환성이 좋다.

Cons:

- `internal/config` 가 `github.com/robfig/cron/v3` 를 import 하게 된다.
- scheduler 호출부에서 `String()` 변환이 필요하다.
- invalid env 를 기존보다 더 이른 시점에 실패시켜 운영상 breaking behavior 로 느껴질 수 있다.

Metrics:

- 구현 공수: 30~45분
- 리스크: 낮음
- 타 코드 영향도: `internal/config`, `internal/scheduler`, config tests
- 유지보수 부담: 낮음

Recommendation:

- **Option A 권장.** 이 프로젝트는 scheduler 기반 push 학습이 핵심이고, cron 오타는 조용한 기능 중단으로 이어진다. config load 단계에서 fail-fast 하는 편이 운영 안정성에 유리하다.

### Option B: custom defined type 만 도입하고 validation 은 scheduler 에 유지

```go
type CronExpr string
```

필드 타입만 `CronExpr` 로 바꾸고 validation 은 추가하지 않는다.

Pros:

- 영향 범위가 가장 작다.
- 기존 runtime behavior 변화가 거의 없다.

Cons:

- invalid cron 을 조기에 검출하지 못한다.
- custom type 의 실익이 문서화 수준에 머문다.

Metrics:

- 구현 공수: 15~20분
- 리스크: 매우 낮음
- 타 코드 영향도: `internal/config`, `internal/scheduler`
- 유지보수 부담: 중간

### Option C: Do nothing

현재 `string` 유지.

Pros:

- 변경 없음.
- scheduler 등록 실패 로그만으로도 문제를 발견할 수 있다.

Cons:

- config 계층에서 domain invariant 를 표현하지 못한다.
- 잘못된 cron 이 배포 후 scheduler 시작 단계까지 내려간다.
- 같은 형태의 string 필드가 늘어날수록 의미가 흐려진다.

Metrics:

- 구현 공수: 0분
- 리스크: cron 설정 오류를 늦게 발견
- 타 코드 영향도: 없음
- 유지보수 부담: 중간

## 결정된 구현 방향

**Option A로 진행한다.**

다만 backward compatibility 를 위해 validation 은 **non-empty 값만 검증**한다. 빈 값은 `Validate()` 에서 통과시킨다.

이유:

- `content_collect_cron` 은 현재 `scheduler.Start()` 에서 빈 값이면 job 등록을 skip 하는 경로가 있다.
- push cron 들은 default 가 있으므로 일반 운영에서는 비어 있지 않다.
- 빈 값까지 금지하는 정책 변경은 별도 운영 의사결정이다. 이번 TODO 에서 scope 를 넓히지 않는다.

## 변경할 파일

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/scheduler/scheduler.go`
- `docs/workthrough/2606/YYMMDDhhmm_schedule_cronexpr_config_type.md`

건드리지 말 것:

- `config.yaml` key 이름과 값
- env key 이름 (`COPYLINGO_SCHEDULE_*`)
- scheduler job 종류/실행 순서
- `MorningBuildCron`, `EveningBuildCron` 사용 정책
- DB schema / migration
- ADR 문서. 이번 작업은 local type-safety refactor 이며 별도 아키텍처 결정으로 보지 않는다.

## Before/After 스니펫

### 1. `internal/config/config.go`

Before:

```go
import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)
```

After:

```go
import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)
```

Before:

```go
type ScheduleConfig struct {
	ContentCollectCron string `mapstructure:"content_collect_cron"` // 콘텐츠 수집 크론
	MorningBuildCron   string `mapstructure:"morning_build_cron"`   // 오전 세션 빌드 크론
	MorningPushCron    string `mapstructure:"morning_push_cron"`    // 오전 세션 푸시 크론
	StudyPushCron      string `mapstructure:"study_push_cron"`      // 정오 Study 세션 푸시 크론
	EveningBuildCron   string `mapstructure:"evening_build_cron"`   // 오후 세션 빌드 크론
	EveningPushCron    string `mapstructure:"evening_push_cron"`    // 오후 세션 푸시 크론
}
```

After:

```go
type CronExpr string

func (c CronExpr) String() string {
	return string(c)
}

func (c CronExpr) IsZero() bool {
	return strings.TrimSpace(string(c)) == ""
}

func (c CronExpr) Validate(name string) error {
	if c.IsZero() {
		return nil
	}
	if _, err := cron.ParseStandard(c.String()); err != nil {
		return fmt.Errorf("%s is invalid cron expression %q: %w", name, c.String(), err)
	}
	return nil
}

type ScheduleConfig struct {
	ContentCollectCron CronExpr `mapstructure:"content_collect_cron"` // 콘텐츠 수집 크론
	MorningBuildCron   CronExpr `mapstructure:"morning_build_cron"`   // 오전 세션 빌드 크론
	MorningPushCron    CronExpr `mapstructure:"morning_push_cron"`    // 오전 세션 푸시 크론
	StudyPushCron      CronExpr `mapstructure:"study_push_cron"`      // 정오 Study 세션 푸시 크론
	EveningBuildCron   CronExpr `mapstructure:"evening_build_cron"`   // 오후 세션 빌드 크론
	EveningPushCron    CronExpr `mapstructure:"evening_push_cron"`    // 오후 세션 푸시 크론
}
```

Add near `Config.validate()`:

```go
func (c *ScheduleConfig) validate() error {
	checks := []struct {
		name string
		expr CronExpr
	}{
		{name: "schedule.content_collect_cron", expr: c.ContentCollectCron},
		{name: "schedule.morning_build_cron", expr: c.MorningBuildCron},
		{name: "schedule.morning_push_cron", expr: c.MorningPushCron},
		{name: "schedule.study_push_cron", expr: c.StudyPushCron},
		{name: "schedule.evening_build_cron", expr: c.EveningBuildCron},
		{name: "schedule.evening_push_cron", expr: c.EveningPushCron},
	}
	for _, check := range checks {
		if err := check.expr.Validate(check.name); err != nil {
			return err
		}
	}
	return nil
}
```

`Config.validate()`:

```go
if err := c.Schedule.validate(); err != nil {
	return err
}
```

### 2. `internal/scheduler/scheduler.go`

Before:

```go
if s.orchestrator != nil && s.cfg.Schedule.ContentCollectCron != "" {
	if _, err := s.cron.AddFunc(s.cfg.Schedule.ContentCollectCron, func() {
		s.runJob("content_collection", 10*time.Minute, s.collectContent)
	}); err != nil {
		// ...
	}
}
```

After:

```go
if s.orchestrator != nil && !s.cfg.Schedule.ContentCollectCron.IsZero() {
	if _, err := s.cron.AddFunc(s.cfg.Schedule.ContentCollectCron.String(), func() {
		s.runJob("content_collection", 10*time.Minute, s.collectContent)
	}); err != nil {
		// ...
	}
}
```

Before:

```go
if _, err := s.cron.AddFunc(s.cfg.Schedule.MorningPushCron, func() {
	// ...
}); err != nil {
	// ...
}
```

After:

```go
if _, err := s.cron.AddFunc(s.cfg.Schedule.MorningPushCron.String(), func() {
	// ...
}); err != nil {
	// ...
}
```

동일하게 `StudyPushCron`, `EveningPushCron` 의 `AddFunc` 인자와 로그의 `cron` 값도 `.String()` 으로 명시 변환한다.

## 테스트 추가

`internal/config/config_test.go` 에 다음 테스트를 추가한다.

### 1. default schedule 값이 `CronExpr` 로 로딩되는지

```go
func TestLoadScheduleDefaults(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Schedule.MorningPushCron.String(), "0 8 * * *"; got != want {
		t.Fatalf("Schedule.MorningPushCron = %q, want %q", got, want)
	}
	if cfg.Schedule.MorningPushCron.IsZero() {
		t.Fatal("Schedule.MorningPushCron IsZero() = true, want false")
	}
}
```

### 2. env override 값이 `CronExpr` 로 로딩되는지

```go
func TestLoadScheduleEnvOverrides(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Setenv("COPYLINGO_SCHEDULE_STUDY_PUSH_CRON", "15 12 * * *")
	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Schedule.StudyPushCron.String(), "15 12 * * *"; got != want {
		t.Fatalf("Schedule.StudyPushCron = %q, want %q", got, want)
	}
}
```

### 3. invalid cron 은 `Load()` 에서 실패하는지

```go
func TestLoadRejectsInvalidScheduleCron(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Setenv("COPYLINGO_SCHEDULE_STUDY_PUSH_CRON", "not-a-cron")
	t.Chdir(t.TempDir())

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}
```

### 4. empty cron 은 validation 을 통과하는지

```go
func TestCronExprAllowsEmpty(t *testing.T) {
	if err := (CronExpr(" ")).Validate("schedule.content_collect_cron"); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
```

## 검증 방법

```bash
go test ./internal/config ./internal/scheduler
make test
```

로컬 runtime 반영이 필요한 서버 로직 변경은 아니므로 `make restart-app` 은 필수 아님. 단, 실행 agent 가 판단하기에 scheduler startup behavior 를 직접 확인하고 싶으면 `make restart-app` 후 `curl http://localhost:8080/health` 를 추가로 수행해도 된다.

## 완료 처리

작업 완료 후 다음을 수행한다.

1. `docs/workthrough/2606/YYMMDDhhmm_schedule_cronexpr_config_type.md` 생성
   - 변경 파일
   - 결정 사항
   - 검증 결과
2. `STATUS.md` 에서 이 TODO 항목 제거
3. `docs/todos/schedule_cronexpr_config_type.md` 삭제
