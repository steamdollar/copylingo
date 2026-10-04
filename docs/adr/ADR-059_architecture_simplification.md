# ADR-059: 기능별 호출 경계와 서버 초기화 구조 단순화

- 날짜: 2026-09-26
- 상태: **설계 방향 승인, 1단계 완료; 2·3단계는 §8의 세분화 순서(A~E)로 진행 중 — A·B 완료(2026-09-30), C 완료(2026-10-04), D 완료(2026-10-05), 다음 E**
- 보강: 2026-09-30 — 서비스 2계층·생성자 규칙·하위 계층 규칙 추가 (§8)
- 범위: 패키지 간 호출 경계, Quiz·Study 책임 배치, Redis 접근, 서버 인스턴스 생성·주입
- 관련 결정: [ADR-057·058·060](ADR_from_41_to_60.md), [1단계 구현 기록](../workthrough/2609/2609262045_redis_access_boundary.md)

## 1. 배경과 목표

현재 코드는 한 기능을 이해하려면 봇, 여러 서비스, Redis 상태를 함께 추적해야 한다. 서버 시작 코드에서도 같은 설정·서비스·연결 객체를 여러 초기화 함수에 반복 전달한다.

사용자는 프로젝트 전반의 의존 관계와 패키지 크기·파편화를 조사하고, 호출 경로와 초기화 구조를 단순화하는 방향에 동의했다. 이 문서는 그 논의를 단계별 구조 변화로 기록한 것이다. 독립 실행용 TODO나 구현 완료 보고서는 아니다.

목표는 다음과 같다.

1. Quiz 동작은 Quiz 담당 서비스에서, 화면 동작은 해당 핸들러에서 출발해 읽을 수 있다.
2. 각 객체의 생성자에서 실제 필요한 의존성을 확인할 수 있다.
3. Redis 저장 형식과 외부 클라이언트 생성은 해당 구현·조립 위치에 모인다.
4. 서버 조립 함수 한곳에서 전체 연결을 확인하고, 실행 단계에서는 완성된 객체를 시작·종료한다.

수만 사용자 규모를 가정한다. 현재의 working set, 트랜잭션, 멱등성, batch 조회, worker와 rate limit은 구조 정리 과정에서도 유지한다.

## 2. 리팩터링 전 구조에서 확인한 문제

설계 조사 당시(1단계 구현 전)의 working tree 기준 `internal`은 11개 패키지, 구현 Go 파일 78개, 12,719줄이었다. 테스트를 제외하고 주석·빈 줄을 포함한 수치다. `go list ./...`가 통과했고, 패키지 순환 의존이나 `bot → repository` 직접 import는 없었다. 아래 문제 목록은 그 시점의 조사 기록이다.

| 패키지 | 구현 줄 수 | 판단 |
|---|---:|---|
| `bot` | 3,708 | 화면 처리에 세션 정책·Redis 조작이 섞여 있어 우선 정리 |
| `service` | 2,762 | 기능별 책임과 공개 API의 분산을 정리 |
| `repository` | 2,398 | 쿼리와 트랜잭션 경계 유지 |
| `external` | 986 | 패키지 유지, 소비자가 사용하는 계약의 범위 점검 |
| `model` | 712 | 공통 모델·상태 소유 |
| `scheduler` | 612 | 실행 제어 유지, 서비스와 저장 기능의 주입 범위 축소 |
| `miniapp` | 405 | HTTP 처리와 Telegram 메시지 갱신 책임 구분 |
| `config` | 391 | 실행 설정 이외의 상수를 소유 위치로 이동 |
| `observability` | 358 | 유지 |
| `pipeline` | 308 | 자동 실행 비활성 상태와 기존 보존 결정 유지 |
| `callback` | 79 | 공유 callback 규약 유지, Redis 값 파싱 책임 이동 |

줄 수 자체를 패키지 분리 기준으로 사용하지 않는다. 다음 코드가 책임 분산의 직접적인 근거다.

- [Services](../../internal/service/services.go)는 서비스 16개를 공개하고, Bot은 그중 12개에 접근한다.
- [SessionFlow](../../internal/bot/session_flow.go)와 [StudyFlow](../../internal/bot/study_flow.go)는 `*Bot` 전체를 받아 API·설정·서비스·Redis에 접근한다.
- [SessionBuilder](../../internal/service/session_builder.go)는 생성 외에 조회·시작도 담당한다. 완료는 [Grader](../../internal/service/grader.go)에 있다.
- [봇 답안 처리](../../internal/bot/session_answer.go)는 AI 채점 설정이 없으면 직접 `QuizActiveSession.RecordAnswer(false)`를 호출한다.
- [Mini App](../../internal/miniapp/handler.go)은 봇이 기록한 Redis 키·값 형식을 알고 Telegram 버튼까지 구성한다.
- 세션 상태가 [config](../../internal/config/constants.go)와 [model](../../internal/model/session.go)에 중복 정의돼 있다.
- [서버 시작 코드](../../cmd/server/main.go)는 서비스·봇·DB·Redis를 worker와 router 초기화에 반복 전달한다.

## 3. 현재에서 최종 구조까지

아래 화살표는 주요 호출 관계다. 모든 import나 보조 기능을 표시한 그래프는 아니다. 변경 후의 타입·메서드명은 설계를 설명하는 예시이며 실제 API는 소스 코드를 따른다. 1단계는 구현됐고, Quiz 책임 통합과 서버 초기화 정리는 후속 단계다.

### 현재: 봇이 Quiz의 여러 세부 기능을 직접 조정

```mermaid
flowchart LR
    bot["bot / SessionFlow"]
    builder["SessionBuilder: 생성·조회·시작"]
    grader["Grader: 채점·완료"]
    active["QuizActiveSession: 진행 상태"]
    redis[("Redis")]
    repo["repository"]
    bot --> builder
    bot --> grader
    bot --> active
    bot -->|"입력 상태·초안"| redis
    grader --> active
    active --> redis
    builder --> repo
    active --> repo
    grader -->|"streak 갱신"| repo
```

답안 제출을 이해하려면 화면 처리뿐 아니라 상태 조회·채점·실패 시 기록 순서를 함께 읽어야 한다. 시작과 완료도 서로 다른 이름의 서비스에 배치돼 있다.

### 1단계: Redis 저장 구현을 모으기

신규 패키지 `internal/redisstore`가 키·직렬화·TTL·Redis 오류 변환·원자적 명령을 소유한다. 기존 Redis TODO를 실행했으며 구체적인 저장 계약과 검증 결과는 [1단계 구현 기록](../workthrough/2609/2609262045_redis_access_boundary.md)에 남겼다.

```mermaid
flowchart LR
    bot["bot"]
    miniapp["miniapp"]
    scheduler["scheduler"]
    service["기존 service"]
    store["redisstore"]
    redis[("Redis")]
    bot -->|"입력 상태·메시지 참조"| store
    miniapp -->|"메시지 참조"| store
    scheduler -->|"발송 claim"| store
    service -->|"Quiz·Study 진행 상태"| store
    store --> redis
```

호출자는 `Get/Set`이나 문자열 키 대신 `GetHandwritingMessage`, `ConsumePending`, `TryClaim` 같은 기능을 사용한다. 각 호출자에는 필요한 기능 인터페이스만 전달한다.

Telegram 입력·화면 상태와 scheduler claim은 각 호출자의 저장 인터페이스로 다룬다. 모든 Redis 접근을 학습 서비스에 넣지 않는다. 서버 조립부의 연결 생성·종료와 health Ping은 유지한다.

**단계 완료 후:** 저장 포맷 변경을 `redisstore`에서 추적할 수 있다. Quiz 책임 분산은 아직 남아 있다.

### 2단계: Quiz·Study의 처리 책임 모으기

Quiz의 생성·시작·답안 제출·완료를 담당하는 공개 진입점을 정한다. 기존 `SessionBuilder`·`Grader`·`QuizActiveSession`과 봇의 책임을 재배치한다. 기존 메서드를 그대로 노출하는 전달 계층을 추가하는 방식은 피한다.

```mermaid
flowchart LR
    bot["bot / SessionFlow"]
    subgraph service ["service 패키지"]
        quiz["QuizService: 생성·시작·제출·완료"]
        selection["문제 편성 로직"]
        grading["채점 로직"]
        progress["진행 상태·SRS 로직"]
        quiz --> selection
        quiz --> grading
        quiz --> progress
    end
    repo["repository"]
    store["redisstore"]
    bot --> quiz
    quiz --> repo
    quiz --> store
```

이 그림의 `QuizService`는 처리 순서와 정책을 책임지는 주체다. 모든 계산을 한 파일로 합치거나 생성·채점·상태 처리를 새 패키지로 각각 분리하라는 뜻은 아니다. 하위 계산과 저장 호출은 내부 구현으로 유지할 수 있다.

| 사용자 동작 | 현재 봇이 조정하는 내용 | 변경 후 진입점 예시 |
|---|---|---|
| 시작 | 진행 상태 조회, 소유자 확인, 시작 반영 | `Quiz.Start(...)` |
| 제출 | 현재 문제 확인, 채점 요청, 실패 시 기록 | `Quiz.SubmitAnswer(...)` |
| 완료 | 완료 서비스 호출, 후속 화면 상태 정리 | `Quiz.Complete(...)` |

```text
bot
  Telegram 입력 해석 → Quiz.SubmitAnswer → 결과 표시

Quiz.SubmitAnswer
  소유자·문제·중복 제출 검증
  → 문제 유형에 따른 채점
  → 기존 정책에 따른 답안·진행 상태 반영
  → 처리 결과 반환
```

Study는 현재 `StudyActiveSession.Start/MarkStudied/Complete`에 모인 흐름을 활용한다. 손글씨의 입력 검증·이미지 처리 경계는 유지하면서 공통 답안 반영은 Quiz 담당 로직으로 연결한다. 유형별 채점 실패 정책을 임의로 통일하지 않는다.

**단계 완료 후:** Quiz 동작 변경은 Quiz 담당 서비스에서 출발한다. 봇은 내부 서비스의 호출 순서나 답안 기록 정책을 알 필요가 없어진다.

### 3단계: 필요한 기능만 주입하고 소유권 고정하기

`SessionFlow`, `StudyFlow`, scheduler에 전체 `Services`나 `*Bot`을 전달하지 않는다. 각 객체가 사용하는 기능과 설정만 받게 한다. 같은 Quiz 구현도 scheduler에는 생성·조회 계약만, 풀이 흐름에는 시작·제출·완료 계약을 제공할 수 있다.

```mermaid
flowchart LR
    sessionFlow["SessionFlow"]
    studyFlow["StudyFlow"]
    scheduler["scheduler"]
    miniapp["Mini App 핸들러"]
    quiz["Quiz 기능"]
    study["Study 기능"]
    handwriting["손글씨 제출 기능"]
    sessionFlow -->|"시작·제출·완료"| quiz
    studyFlow -->|"시작·학습 표시·완료"| study
    scheduler -->|"생성·조회"| quiz
    scheduler -->|"생성·조회"| study
    miniapp --> handwriting
    handwriting -->|"답안 반영"| quiz
```

Mini App의 Telegram 버튼 구성은 봇의 좁은 메시지 갱신 계약으로 옮긴다. Mini App은 손글씨 처리 후 화면 갱신을 요청하고, 봇이 Telegram 표현을 소유한다. 이 연결을 위해 이벤트 버스를 도입하지 않는다.

| 대상 | 소유 위치 |
|---|---|
| 환경변수·실행 설정 | `config` |
| 세션 상태 | `model` |
| Telegram callback 규약 | `callback` 또는 봇 내부 |
| Redis 키·값 포맷 | `redisstore` |

**단계 완료 후:** 생성자에서 해당 객체의 호출 범위가 보인다. 기능 인터페이스로 메서드 접근을 좁히고 금지 import 검사로 패키지 경계를 고정한다.

## 4. 서버 시작·인스턴스 초기화도 함께 정리

좁은 인터페이스를 도입하는 것만으로 생성자 인자 수가 줄지는 않는다. 객체 생성 위치와 전달 경로도 함께 바꾼다. 이 작업은 3단계에 포함하며, 1·2단계의 생성자 변경도 같은 조립 위치에서 반영한다.

현재는 다음 경로에 초기화와 전달이 분산돼 있다.

```text
run → initApp → NewRepositories / NewServices / NewBot
run → startWorkers → initScheduler → scheduler.New
run → setupRouter → miniapp.RegisterRoutes → miniapp.NewHandler
```

목표는 `cmd/server`의 조립 함수에서 다음 순서로 한 번 연결하는 것이다. 단계별 코드는 기존 파일에 나눠 둘 수 있지만, 인자 전달만을 위한 중간 함수를 늘리지 않는다.

```text
DB·Redis 연결
  → 저장소·외부 API 클라이언트 생성
  → Quiz·Study 등 서비스 생성
  → Bot·Mini App·Scheduler 생성
  → HTTP 서버 연결
  → 실행·종료할 객체 반환
```

현재 `service.NewServices` 안의 LLM·TTS·S3 클라이언트 생성도 조립 위치로 이동한다. DB·Redis 연결과 공유 클라이언트는 필요한 범위에서 한 번 생성해 재사용한다. 사용처마다 새 연결을 만들지 않는다.

라우터는 완성된 핸들러의 경로를 등록한다. 다음은 책임 변화를 보여주는 예시다.

```go
// 현재: 핸들러 생성에 필요한 재료까지 전달한다.
setupRouter(cfg, db, rdb, services, botHandler)

// 목표: 준비된 핸들러의 경로만 연결한다.
setupRouter(healthHandler, miniappHandler)
```

설정·로깅 이후의 시작 코드는 다음과 같은 형태를 목표로 한다. `initApp`, `Run`, `Close`의 최종 시그니처는 구현 시 정한다.

```go
// 조립 결과의 시작·종료만 최상위에서 관리한다.
app, err := initApp(cfg)
if err != nil {
    return err
}
defer app.Close()
return app.Run()
```

`app`은 `cmd/server` 내부의 작은 실행 객체로, Bot·Scheduler·HTTP 서버와 자원 정리를 소유한다. 하위 서비스에 전달하는 전역 의존성 컨테이너로 사용하지 않는다. 초기화 중간 실패 시 이미 연 자원을 정리하고, 정상 종료 시 작업 중단과 연결 종료 순서를 명확히 한다.

이 변경 후에도 실제 객체 생성 코드는 남는다. 반복 전달과 분산된 초기화를 줄이는 것이 목적이며, 필요한 연결을 감추기 위한 DI 프레임워크·전역 변수·범용 컨테이너는 도입하지 않는다.

## 5. 최종 패키지 배치와 허용 경계

```text
cmd/server/        객체 생성·연결·시작·종료
internal/
  bot/             Telegram 입력과 화면
  miniapp/         HTTP 입력과 응답
  scheduler/       실행 시점과 발송 제어
  service/         Quiz·Study 등 기능별 처리 책임
  repository/      PostgreSQL 조회와 트랜잭션
  redisstore/      Redis 저장 구현 — 신규
  external/        LLM·TTS·S3 등 외부 연동
  model/           공통 모델과 상태
  config/          실행 설정
  callback/        Telegram callback 규약
  observability/   로그
  pipeline/        콘텐츠 수집
```

Quiz와 Study 담당 서비스는 기존 `service` 패키지의 구성요소다. 이 계획에서 내부 패키지는 11개에서 12개가 된다. 추가적인 기능별 패키지 분리는 이번 방향에 포함하지 않는다.

| 호출자 | 허용할 의존·역할 |
|---|---|
| `cmd/server` | 구현체 생성과 연결, 연결 상태 확인, 생명주기 관리 |
| `bot`, `miniapp`, `scheduler` | 필요한 서비스 기능과 각자의 기술적 상태 저장 인터페이스 |
| `service` | 모델, 필요한 저장·외부 연동 계약 |
| 저장·외부 연동 구현 | DB·Redis·외부 API 호출 |
| `model` | 다른 애플리케이션 레이어에 의존하지 않음 |

호출 관계와 import 관계는 구분한다. 인터페이스 주입은 객체가 사용할 수 있는 메서드를 제한한다. 소스 파일의 금지 import는 별도 검사로 확인한다. 인터페이스만 만들고 raw Redis 타입이나 전체 서비스 묶음을 그대로 노출하는 변경은 완료로 보지 않는다.

## 6. 결과와 trade-off

- Redis 패키지와 필요한 기능 인터페이스가 생기므로 파일·선언 수 일부는 늘 수 있다. 대신 저장 계약의 중복 지식과 넓은 서비스 접근을 줄인다.
- Quiz 공개 진입점을 정리하면서 기존 호출자와 테스트 수정이 필요하다. 세부 정책은 유지하고 책임 이동과 기능 변경을 섞지 않는다.
- 조립 코드는 서비스 패키지에서 `cmd/server`로 이동한다. 최상위 `run`은 간결해지지만 실제 연결은 조립부에 명시적으로 남는다.
- SQL의 eligibility 필터·정렬과 완료 트랜잭션을 기계적으로 service로 옮기지 않는다. DB에서 수행할 필터링과 원자적 저장은 보존한다.
- 작은 공유 패키지의 통폐합이나 전체 feature별 디렉터리 재배치는 하지 않는다. 변경하지 않는 선택도 유지한다.
- ADR-057·058의 자동 콘텐츠 수집 비활성 및 관련 코드 보존 결정을 변경하지 않는다. 새로운 실행 경로를 켜지 않는다.

## 7. 실행 순서와 완료 확인

| 순서 | 작업 | 단계 완료 시 확인할 결과 |
|---|---|---|
| 1 (완료) | Redis 저장 경계 정리 | 호출자가 키·직렬화·Redis 명령 반환 타입을 모름 |
| 2 → §8 B | Quiz 책임 통합, Study 경계 정리 | 화면 코드가 채점·답안 기록·완료 정책을 직접 조정하지 않음 |
| 3 → §8 A·C·D·E | 좁은 기능 주입, 서버 조립 통합, 상수 이동 | 한 조립 경로에서 생성 순서를 확인하고 각 생성자에서 의존 범위를 읽을 수 있음 |

구현 범위는 단계별로 잡는다. 후속 단계의 정확한 메서드명, 내부 helper 유지·통합 위치, 실행 객체의 종료 API는 설명용 예시만으로 확정하지 않는다. 기존 Redis TODO는 1단계 완료 후 제거하고 위 구현 기록에 결과를 남겼다.

후속 구현에서는 기존 테스트로 세션 시작·재개·중복 제출·채점 실패·완료, 손글씨 화면 갱신, 발송 claim을 확인한다. 키·TTL·원자성·트랜잭션·worker·rate limit의 동작을 보존한다. 서버 조립 변경은 초기화 실패 시 정리와 정상 시작·종료도 확인한다.

코드 변경 시 프로젝트 규칙에 따라 `make test`를 수행한다. 로컬 런타임 변경은 Makefile의 타깃 설명에 따라 앱을 재시작하고 `/health`를 확인한다. 이번 문서 export는 코드·설정·실행 상태를 변경하지 않으므로 해당 실행 검증 대상이 아니다.

## 8. 보강 (2026-09-30): 계층 규칙과 세분화된 실행 순서

### 8.1 추가 진단 — 인자 수가 아니라 묶음 전달이 문제

1단계 완료 후 실측 결과, 생성자 인자는 최대 5개로 많지 않았다. 읽기 어려움의 원인은 인자로 전달되는 묶음과 상호 참조다.

- `Services`(16필드)를 bot이 12종·52회, scheduler가 8종 사용한다. `NewBot(cfg, services, stores)`는 인자 3개지만 실제 의존은 본문을 모두 읽어야 드러난다.
- `SessionFlow{bot *Bot}`·`StudyFlow{bot *Bot}`와 `Bot`이 서로를 참조한다. `*Bot` 41개·`SessionFlow` 29개·`StudyFlow` 9개 메서드가 사실상 한 객체로 동작한다. 같은 `*Bot`이 scheduler의 `sessionPusher`와 Mini App의 `TelegramMessenger`로도 주입된다.
- 서비스 간 의존 계층(Handwriting → Grader → QuizActiveSession → SRS, Grader → LLM, SessionBuilder → SRS)은 존재하지만 `Services`가 16개를 평면으로 공개해 호출자가 상·하위 서비스를 섞어 직접 조정한다.
- 1단계의 `bot.StateStores` 5개 필드는 모두 같은 `interactions`를 받는다. 소비자별 분리 없이 인터페이스만 나뉜 상태이며 C 단계의 Flow 분리 후 소비자별로 전달한다.

패키지 import 그래프는 순환이 없고 계층 위반도 없다. 따라서 이번 보강은 **패키지를 늘리지 않는다**(12개 유지). 정리 대상은 패키지 내부의 타입·공개 범위·전달 경로다.

### 8.2 서비스 2계층

```mermaid
flowchart TB
    adapters["bot · scheduler · miniapp"]
    subgraph service ["service 패키지"]
        t1["Tier1 (exported): Session(Quiz+Study) · User · Preference · Stats · Tip · LLMQuestion · Audio"]
        t2["Tier2 (unexported): selection · progress(+SRS) · grading · llm"]
        t1 --> t2
    end
    lower["repository · redisstore · external"]
    adapters -->|"Tier1만"| t1
    t1 --> lower
    t2 --> lower
```

- 호출자(bot·scheduler·miniapp)는 Tier1만 참조한다. Tier2는 unexported로 두어 컴파일러가 경계를 강제한다.
- Tier2는 Tier1 생성자 내부에서 만든다. `cmd/server`는 하위 계층과 Tier1만 조립한다.
- Tier1은 기존 호출자가 조정하던 순서(소유자 확인 → 채점 → 실패 기록 → 완료)를 흡수해야 한다. 메서드를 그대로 전달하는 계층이면 완료로 보지 않는다.
- 원래 leaf인 서비스(User·Preference·Tip 등)는 감싸지 않는다. 큰 Tier1은 같은 패키지 안에서 파일로 나눈다(`quiz_start.go`, `quiz_submit.go` 등).
- Tier2를 하위 패키지(`service/srs`, `service/grading`)로 분리하는 안은 파편화를 늘려 기각한다.

### 8.3 생성자 규칙

- 위치 인자는 4개 이하. 초과 시 이름 있는 필드를 가진 `XxxDeps` struct를 사용한다(기존 `miniapp.HandlerDeps` 방식).
- `Deps`의 각 필드는 소비자가 정의한 좁은 인터페이스 또는 값이다. `service.Services`·`*bot.Bot`·`*config.Config` 전체를 넣지 않는다.
- 묶음 struct(`Repositories` 등)는 `cmd/server` 조립 중에만 사용한다. `Repositories`는 조립부 외 사용처가 없어 유지한다.
- 인자 수가 줄어드는 것은 목표가 아니다. Tier1 Quiz처럼 의존이 7~8개가 되는 생성자는 `Deps` 필드 목록이 곧 외부 경계 목록이 되도록 한다.

### 8.4 하위 계층(repository · redisstore · external) 규칙

- 구조는 유지한다. 테이블 단위 repository와 서비스의 트랜잭션 범위 결정(ADR-061), 소비자 정의 인터페이스 방식을 그대로 둔다. 기능별 repository 통합은 Quiz·Study가 같은 테이블을 공유해 중복만 늘리므로 기각한다.
- 하위 계층은 `model`과 드라이버만 import한다. `config`·`service`·`bot`을 import하지 않는다.
  - 세션 상태는 `model`로 일원화한다(`repository/session_repo.go`의 `config.SessionStatus` 제거).
  - `external` 생성자는 `*config.Config` 대신 필요한 값의 options struct를 받는다. `GenerateTips`를 인터페이스에 정식 포함해 `NewServices`의 concrete 타입 단언을 제거한다. (D단계에서 concrete client 반환·인터페이스 삭제로 대체했다. §8.8)
- 서비스의 저장·외부 인터페이스는 Tier1 기준으로 다시 정의한다. Tier2 간 호출은 같은 패키지의 concrete 호출로 바꿀 수 있다.

### 8.5 세분화된 실행 순서

각 단계는 독립 커밋이며 `make test` 통과를 완료 조건으로 한다.

| 단계 | 작업 | 완료 기준 |
|---|---|---|
| A | Telegram 전송 기능(`Send`/`Edit`/키보드/음성)을 bot 패키지 내부 타입으로 추출 | Flow가 전송을 위해 `Bot`을 거치지 않음 |
| B | Quiz·Study Tier1 통합, Tier2 unexported (§8.2) | bot·scheduler·miniapp이 Tier1 타입만 참조 |
| C | 기능별 Flow 분리, `*Bot` 역참조 제거, scheduler·Mini App에 Flow의 좁은 계약 주입 | bot 패키지 필드에 `*Bot`·`*service.Services`가 없음 |
| D | `Services` 묶음 삭제, 외부 클라이언트 생성 이동, `app.Run/Close`, 필요한 설정 값만 전달, 세션 상태 `model` 일원화 | 조립 경로 한곳에서 생성 순서 확인 가능, 하위 계층이 `config` import 안 함 |
| E | `go list` 기반 import 경계 테스트 | 금지 import가 CI에서 실패 |

B를 C보다 먼저 한다. 반대로 하면 Flow가 받는 인터페이스를 두 번 바꿔야 한다. A는 B·C와 독립적이어서 먼저 수행한다.

### 8.6 B단계 세부 결정 (2026-09-30)

B단계 착수 전 Discovery 결과로 §8.2의 Tier1 구성을 다음과 같이 확정한다. 위 다이어그램은 이 결정을 반영했다.

| 대상 | 결정 | 근거 |
|---|---|---|
| Quiz·Study | 하나의 Tier1 `SessionService`로 합친다. 파일은 모드 공통(`session.go`, `session_query.go`, `session_dispatch.go`)과 모드별(`session_quiz_*.go`, `session_study_*.go`)로 나눈다. | scheduler의 slot별 Quiz/Study 생성 분기와 모드 공통 조회(미완료 세션, 상태별 목록)가 한 곳에 모인다. 호출자는 C단계에서 좁은 인터페이스로 받으므로 타입 크기가 호출자에 전파되지 않는다. |
| 시작·완료 | 모드별 메서드로 유지한다(`StartQuiz`/`StartStudy`, `CompleteQuiz`/`FinishStudy`). | 상태 타입이 다르고 정책도 다르다(Quiz 완료만 streak 갱신). 유형별 정책을 임의로 통일하지 않는다. |
| `SessionQuery` | `SessionService`에 흡수한다. 미사용 `CountUnfinished`는 삭제한다. | 로직 없이 repo를 전달만 하는 37줄 서비스이고 호출자는 scheduler뿐이다. |
| `TipGenerator` | `TipService`에 흡수하고 unexport한다. LLM client가 없을 때의 nil 허용 동작은 유지한다. | Tier1 "Tip" 하나가 조회·후보 저장·보충을 담당한다. |
| `Audio` | leaf Tier1로 공개 유지한다. | TTS·S3 의존과 key 부재 시 nil 동작이 채점·진행 상태와 무관하다. 합치면 SessionDeps가 4개 늘고 nil 검사가 퍼진다. |
| `LLMQuestion` | 답변 생성 → 팁 후보 저장만 흡수한다. 문맥 프롬프트 조립은 bot에 남긴다. | 문맥 조립은 bot locale 문구와 `renderStudyMaterial` HTML 렌더에 묶여 있다. |
| Stats·Preference | 기존 `AnalyzerService`·`MaterialPreferenceService` 이름을 유지한다. | leaf이며 이름 변경의 동작상 이득이 없다. |
| Redis 진행 상태 저장소 | Quiz·Study 저장소를 한 타입으로 합치지 않는다. `SessionDeps`는 기존 `SessionStores{Quiz, Study}`를 필드 하나로 받는다. | Redis 연결은 이미 하나(`rdb`)이고 두 저장소는 제네릭 `SessionStore[T]`의 타입별 인스턴스다. 합치면 ADR-062에서 제거한 전달 계층이 되살아나고 테스트 대역이 넓어진다. |

`SessionService`의 의존은 repo 7개·`SessionStores`·LLM·DB로 10개이며 §8.3에 따라 `SessionDeps`의 이름 있는 필드로 받는다.

#### 8.6.1 B단계 구현 중 확정 사항 (2026-09-30)

구현 기록은 [B단계 workthrough](../workthrough/2609/2609300127_adr059_stage_b_session_service.md)에 있다. 계획서(`docs/todos/adr059_stage_b_plan.md`)는 완료 처리로 삭제했으며 git 이력에 남는다.

| 대상 | 결정 | 근거 |
|---|---|---|
| `SessionDeps` 필드 타입 | 소비자 정의 인터페이스를 export한다(`QuestionRepo`·`SessionRepo`·`QuizGradingLLM` 등). | 필드 목록이 외부 경계 목록이 된다(§8.3). 다른 패키지의 테스트 대역이 인터페이스를 embed해 필요한 메서드만 구현할 수 있다. |
| 실패 구분 | 호출자 화면이 달라지는 실패만 sentinel error로 둔다: `ErrQuizStatePrepareFailed`·`ErrNoDueReviews`·`ErrQuizStateUnavailable`·`ErrQuizAnswerStale`·`ErrQuizInvalidOption`·`ErrStudyFinalMarkFailed`·`ErrStudyCompleteFailed`. 소유자 불일치·문항 없음은 기존 `ErrQuizActiveSession*`를 재사용한다. 로그만 남기는 실패는 `%w` wrap으로 단계를 메시지에 남긴다. | sentinel 수를 화면 분기 수에 맞춘다. 이 때문에 Quiz 시작의 조회 실패·DB 시작 실패, 답안 채점 실패·대체 기록 실패의 bot 로그 event가 각각 하나로 합쳐졌다. |
| 주관식 typing 표시 | `QuizTextAnswer.OnAIGrading` 훅으로 AI 채점 직전에 호출한다. | 표시가 LLM 호출보다 먼저여야 하고, 텍스트 답안 경로의 bot은 문항 유형을 모른다. 훅은 Redis 조회를 늘리지 않는다. |
| 어순 문제 제출 | `SubmitQuizWordOrder(sessionID, questionID, selection []int)`로 받고, service가 현재 문항 옵션으로 답을 조립하며 순열 여부를 검증한다. | 선택지 제출과 같은 index 입력이고, 답 조립을 도메인 쪽에 둔다. 범용 문자열 제출 입구는 만들지 않는다. draft 관리는 bot에 남긴다. |
| 손글씨 | `HandwritingService` 타입을 없애고 `SessionService.SubmitHandwriting`으로 옮긴다. 렌더러는 기본 PNG 렌더러로 생성자 안에서 만든다. | 흐름은 그대로이고 전달 계층만 사라진다. |
| Tip·LLMQuestion 생성자 | `NewTipService(repo, generatorLLM, sourceModel)`가 generator를 내부에서 만든다. `NewLLMQuestionService(llm, tips, sourceModel)`는 답변 후 `TipService`로 후보를 저장한다. | 팁 후보의 source model을 bot 설정에서 읽지 않는다. `GenerateTips` concrete 단언은 D단계에서 제거할 때까지 `NewServices`에 남긴다. |

### 8.7 C단계 결정 (2026-10-01~04)

구현 기록은 [C단계 workthrough](../workthrough/2610/2610042335_adr059_stage_c_feature_flows.md)에 있다. 계획서(`docs/todos/adr059_stage_c_plan.md`)는 완료 처리로 삭제했으며 git 이력에 남는다.

| 대상 | 결정 | 근거 |
|---|---|---|
| Flow 분리 범위 | `SessionFlow`·`StudyFlow`·`SettingsFlow`·`LLMQuestionFlow` 4개로 나눈다. `Bot`은 update 라우팅과 메뉴·통계·streak·help·exit·study·test 명령만 맡는다. | 기능별 의존이 각 Flow의 `Deps` 필드 목록으로 드러난다. Settings·LLM을 `Bot`에 남기면 `Bot`이 User·MaterialPreference·LLMQuestion을 계속 모아 갖는다. |
| 조립 위치 | cmd/server가 `TelegramClient` → 각 Flow → `Bot` 순으로 만들고, scheduler·Mini App에 Flow를 직접 넘긴다. 조립 결과 묶음(`botComponents`)은 cmd/server 안에서만 쓴다. | `NewBot` 내부 조립 + getter 방식은 `Bot`을 다시 의존 보관소로 만든다. 묶음 struct는 조립부에서만 쓴다(§8.3). |
| Deps 필드 타입 | bot 패키지가 정의한 unexported 인터페이스에 실제 호출 메서드만 넣는다. Flow 간 참조(`Bot`→Flow, `SessionFlow`→`StudyFlow`)는 concrete 포인터다. | Mini App `quizSession` 선례. Flow 간 참조는 같은 패키지 안이라 대역이 필요 없다. |
| Mini App 손글씨 갱신 | Mini App은 `handwritingScreen.ShowHandwritingGraded(ctx, sessionID, questionID)`만 호출한다. 메시지 조회·`QuizProgress` 재조회·키보드·Edit는 `SessionFlow`가 한다. goroutine·15초 timeout·source 속성은 Mini App에 남긴다. | 봇이 Telegram 표현을 소유한다(§3). Mini App이 `tgbotapi`·`callback`을 import하지 않는다. 키보드 문구는 bot locale의 같은 문구 키(`nextQuestionButton`·`linkedMaterialSettingsButton`)를 쓴다. |
| scheduler 계약 | `New(Deps)`로 받고 push 계약을 `quizPusher`(SessionFlow)·`studyPusher`(StudyFlow)로 나눈다. `Deps.Services`는 D단계까지 유지한다. | 한 타입이 두 push를 모두 구현할 필요가 없어진다. `Services` 제거는 D단계 범위다. |
| nil 방어 | service 의존(Session·User·MaterialPreference·LLMQuestion)의 nil 분기는 제거하고 store nil 분기는 유지한다. Audio는 선택 의존으로 남기며 조립부가 `services.Audio != nil`일 때만 넣는다. | 생성자가 Deps를 채우므로 service nil은 조립 오류다. Audio는 TTS key가 없으면 production에서도 nil이고, nil 포인터를 인터페이스에 넣으면 non-nil이 된다. |
| `StateStores` | `StateStores`·`InputStateStore`를 삭제한다. 각 Flow Deps가 필요한 store 계약만 받는다. | §8.1 마지막 항목. 다섯 필드가 모두 같은 `interactions`였다. |
| 테스트 조립 | Flow별 헬퍼(`newTestSessionFlow`·`newTestStudyFlow`·`newTestSettingsFlow`·`newTestLLMQuestionFlow`)와 디스패치용 `newTestBot`을 쓴다. 공통 fixture struct는 만들지 않는다. | 테스트가 필요한 service 의존을 명시하고, 빠진 의존은 nil로 남아 호출 시 바로 실패한다. |

### 8.8 D단계 결정 (2026-10-04~05)

구현 기록은 [D단계 workthrough](../workthrough/2610/2610050008_adr059_stage_d_assembly_lifecycle.md)에 있다. 계획서(`docs/todos/adr059_stage_d_plan.md`)는 완료 처리로 삭제했으며 git 이력에 남는다.

| 대상 | 결정 | 근거 |
|---|---|---|
| 서비스 전달 | cmd/server 내부 unexported `services` struct를 `newServices`가 만든다. `initApp`이 bot·scheduler·Mini App을 조립할 때 필드를 하나씩 넘긴다. | §8.3 묶음 규칙과 `botComponents` 선례. 소비자 패키지는 묶음을 보지 않는다. `initApp`에 모두 펼치면 약 200줄이 된다. |
| LLM client | `NewLLMClient`가 `*DefaultLLMClient`를 반환한다. `external.LLMClient` 인터페이스와 Tier2 `llmService`를 삭제한다. §8.4의 "`GenerateTips`를 인터페이스에 정식 포함"을 이 방식으로 대체한다. | `llmService`는 도달하지 않는 nil 검사만 하는 전달 계층이고 cmd/server에서 만들 수 없다. 이를 지우면 남는 인터페이스는 소비자 없는 생산자 측 선언이다. 서비스는 이미 소비자 정의 인터페이스(`QuizGradingLLM`·`tipGeneratorLLM`·`LearningQuestionLLM`)로 범위를 좁힌다. |
| external 설정 | `LLMOptions`·`TTSOptions`·`S3Options`를 받는다. cfg → options 매핑은 조립부(cmd/server, cmd/admin)가 한다. | §8.4. `observability.LoggerOptions` 선례. |
| `app` | `initApp(cfg) (*app, error)`·`Run(ctx) error`·`Close()`. `Run`은 HTTP 포트를 먼저 bind한 뒤 scheduler → bot polling → 재시작 Mini App 갱신 → `Serve` 순으로 시작한다. ctx 취소나 serve 실패 시 bot → HTTP(10초) → scheduler 순으로 멈춘다. `Close`는 DB → Redis 순이다. signal은 `initApp` 성공 후 등록한다. | §4 "초기화 중간 실패 시 정리, 종료 순서 명확화". 포트 충돌이면 아무것도 시작하지 않고 오류를 반환한다(이전: goroutine 안의 `log.Fatalf`). 기동 중 Ctrl+C는 기존처럼 즉시 종료된다. |
| scheduler 계약 | `Deps.Services`를 User·Session·Tip·Audio 좁은 인터페이스로 바꾼다. 서비스 nil 분기는 제거하고 Audio nil 분기는 유지한다. | §8.7 nil 방어 선례. Audio는 TTS key가 없으면 production에서도 nil이다. |
| 콘텐츠 수집 | `initPipeline`은 `*service.ContentService`를 인자로 받는다. 기동 시 ContentService를 만들지 않는다. | ADR-057 "시작 시 만들지 않으며 관련 생성 코드는 유지". |
| SessionFlow 이름 | 유지한다. | 모드와 무관한 세션 목록·재개도 맡아 QuizFlow는 책임과 어긋난다. 바꾸려면 책임 분리가 먼저다. |
| 경로 상수 | `config.Path*`를 유지한다. | bot·miniapp이 공유하는 규약이며 §8.4 금지 대상(하위 계층)이 아니다. |
