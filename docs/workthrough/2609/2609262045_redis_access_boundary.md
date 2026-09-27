# 아키텍처 단순화 설계와 1단계 Redis 접근 경계 정리

2026-09-26 20:38 설계 export와 20:45 Redis 경계 구현 기록을 통합했다.

## 세션 생명주기와 Study·Quiz 분리 검토

진행 중인 세션의 최신 상태가 Redis에 있다는 기억은 맞습니다. **Go 앱이 세션을 시작·진행·완료시키고, Redis는 그동안의 상태를 보관합니다.** 여기서 보는 것은 Quiz·Study의 진행 상태 키이며, LLM 입력 대기나 메시지 참조는 별도의 보조 상태입니다.

사용자가 자동 발송된 세션 하나를 받아서 끝낸다고 생각하면 순서는 다음과 같습니다. 아래 서비스·봇 처리는 모두 Go 앱 안에서 실행되며, DB·Redis를 읽고 쓸 때 저장 서버와 통신합니다.

| 순서 | 실행 주체와 동작 | PostgreSQL | Redis 진행 상태 |
|---|---|---|---|
| ① 생성·발송 | Go 앱의 스케줄러가 생성 서비스에 문제·자료 편성을 요청하고 봇이 시작 버튼을 보냅니다. | 세션(`pending`)과 문제·자료 목록을 저장 | 아직 만들지 않음 |
| ② 시작 | Go 앱의 봇이 시작 버튼을 받아 시작 처리를 실행합니다. | 세션을 `in_progress`로 변경 | 문제·자료, 현재 위치 등을 저장 |
| ③ 진행 | Go 앱의 서비스가 Quiz 답안 또는 Study 읽음 표시를 반영합니다. | 중간 답안·읽음 기록은 아직 반영하지 않음 | 진행 상태를 갱신 |
| ④ 다시 열기 | Go 앱의 시작 처리가 기존 Redis 상태를 먼저 읽습니다. | Redis 상태가 없을 때 복구에 사용 | 있으면 보존하고 이어서 진행 |
| ⑤ 완료 | Go 앱의 서비스가 완료 조건을 확인하고 DB에 결과를 저장한 뒤 Redis 삭제를 요청합니다. | 결과·학습 이력과 `completed` 상태를 저장 | 성공 경로에서 삭제 |

예를 들어 **Quiz 10문제 중 3개를 풀었으면, 그 답안과 진행 상황은 Redis에 있습니다.** 시작 버튼을 다시 눌러도 Redis 상태부터 읽으므로 4번째 문제부터 이어갑니다. Study에서도 3개 자료를 읽었다면 같은 방식으로 다음 자료부터 이어집니다.

**Redis 만료와 세션 종료는 다릅니다.** 진행 상태는 마지막으로 성공한 저장부터 24시간 뒤 만료됩니다. 만료 자체가 DB의 세션을 `completed`나 `expired`로 바꾸지는 않습니다. 이후 접근하면 서비스가 DB에서 다시 구성합니다. 중간 진행은 완료 시 DB에 반영하므로, Redis 키가 실제로 사라지면 **DB에 아직 저장하지 않은 답안·읽음 기록까지 복원할 수는 없습니다.** 이 점은 두 종류를 합칠지와 별개로 현재 생명주기의 중요한 제약입니다.

실제 코드는 이 순서로 읽으면 됩니다.

1. **Go 앱의 생성 서비스**: [internal/service/session_builder.go](../../../internal/service/session_builder.go#L292)와 [internal/service/study_session.go](../../../internal/service/study_session.go#L228). Quiz 문제와 Study 자료를 각각 편성해 DB에 저장합니다.
2. **Go 앱의 시작 처리**: Quiz는 [internal/bot/session_flow.go](../../../internal/bot/session_flow.go#L313)의 `startSession`이 여러 서비스를 조정합니다. Redis 조회·소유자 확인 → DB 시작 반영 → 처음 시작하는 pending 상태라면 다시 구성하는 순서입니다. Study는 [internal/service/study_active_session.go](../../../internal/service/study_active_session.go#L64)의 `Start`가 이 책임을 가지고 있습니다.
3. **Go 앱의 진행 처리**: Quiz는 [internal/service/quiz_active_session.go](../../../internal/service/quiz_active_session.go#L142)의 `RecordAnswer`가 답안·정오답·다음 복습 일정을 갱신합니다. Study는 [internal/service/study_active_session.go](../../../internal/service/study_active_session.go#L178)의 `MarkStudied`가 읽음 시각을 기록합니다. 둘 다 Redis에 저장합니다.
4. **Go 앱의 완료 처리**: Quiz는 [internal/service/grader.go](../../../internal/service/grader.go#L184)의 `CompleteSession`이 세션 결과 저장 → 연속 학습 기록 갱신 → Redis 삭제를 조정합니다. Study는 [internal/service/study_active_session.go](../../../internal/service/study_active_session.go#L199)의 `Complete`가 모든 자료를 읽었는지 확인하고 DB 저장 → Redis 삭제를 실행합니다. DB 결과 저장은 각 [internal/repository/quiz_active_session_repo.go](../../../internal/repository/quiz_active_session_repo.go#L196)와 [internal/repository/study_active_session_repo.go](../../../internal/repository/study_active_session_repo.go#L144)의 트랜잭션으로 처리합니다.
5. **Go 앱의 저장 구현**: 그다음 [internal/redisstore/session_store.go](../../../internal/redisstore/session_store.go#L37)의 `Load/Save/Delete`를 보면 됩니다. 여기에는 상태 저장·만료시간·누락과 손상 오류 처리가 있고, 시작·완료 여부를 판단하는 학습 규칙은 없습니다.

**분리·통합에 대한 제 추천은 “학습 규칙과 진행 상태는 분리, 공통 저장 코드는 공유”입니다.**

이미 공통인 부분부터 보면, [internal/model/session.go](../../../internal/model/session.go#L39)의 `Session`과 DB의 `sessions` 테이블은 둘이 공유합니다. 세션 ID·사용자·상태·시작/완료 시각을 공통으로 관리하고, `mode`로 Study와 Quiz를 구분합니다. 세션 전체가 두 벌로 따로 구현된 것은 아닙니다.

| 비교 항목 | Study | Quiz |
|---|---|---|
| 사용자의 행동 | 자료를 읽고 넘김 | 문제에 답을 제출함 |
| 진행 상태 | 자료 목록·읽은 시각·이번에 새로 읽은 자료 | 문제 목록·답안·정오답·문제별 복습 상태 |
| 완료 조건 | 모든 자료를 읽음 | 모든 문제에 답함 |
| DB 반영 | 자료별 학습 이력·복습 일정 | 문제별 답안·정답 수·복습 일정 |
| 결과 화면 | 읽기 완료 | 채점 결과·오답 정리 |

이 차이는 [internal/model/study_active_session.go](../../../internal/model/study_active_session.go#L8)와 [internal/model/quiz_active_session.go](../../../internal/model/quiz_active_session.go#L8)에 그대로 나타납니다. 하나의 상태 타입·서비스에 전부 합치면 여러 곳에서 `if study / if quiz`를 확인하거나, 한쪽에서만 쓰는 필드를 함께 들고 다녀야 합니다.

반면 JSON 저장·TTL·삭제는 두 종류에서 같습니다. 이 부분은 공통 `SessionStore[T]`가 직접 제공하고, [internal/service/working_set.go](../../../internal/service/working_set.go#L23)의 저장 오류 변환·검증 연결도 공통입니다. 처음에는 별도 `QuizSessions`·`StudySessions` 포장 타입을 뒀지만, 학습 규칙의 분리를 위해 저장 객체까지 감쌀 필요는 없어 후속 작업에서 두 타입과 전달 메서드를 제거했습니다. 기존 생성 함수는 상태 타입과 Redis 키만 구분합니다.

| 선택 | 얻는 점 | 남거나 늘어나는 복잡함 |
|---|---|---|
| 현재대로 유지 | 추가 변경이 없음 | Quiz의 시작·완료 처리가 여러 곳에 흩어져 있음 |
| Study·Quiz 전체 통합 | 겉으로 보이는 서비스·상태 타입 수가 줄어듦 | 읽기와 채점의 차이를 내부 분기·공통 인터페이스로 처리해야 함 |
| **두 기능을 유지하고 각 생명주기의 담당자를 정리** | Study 또는 Quiz의 흐름을 한 곳에서 따라갈 수 있음 | 실제로 다른 규칙 두 벌은 유지해야 함 |

우선 정리할 부분은 **Quiz의 시작은 봇, 완료는 채점 서비스, 진행 상태는 세션 서비스가 나눠 맡는 구조**입니다. 봇이 여러 서비스를 호출하는 순서를 알지 않아도 되게 Quiz의 시작·답안 제출·완료 책임을 한 서비스에 모으는 편이 효과적입니다. 이름은 예시지만 호출하는 입장에서는 다음 정도로 보이면 됩니다.

```text
Quiz:  Start → SubmitAnswer → Complete
Study: Start → MarkStudied  → Complete
```

이는 [docs/adr/ADR-059_architecture_simplification.md](../../adr/ADR-059_architecture_simplification.md)의 2단계 방향과도 맞습니다. 각 기능 안의 생명주기를 모으는 것이며, Study와 Quiz를 하나의 학습 처리로 합친다는 뜻은 아닙니다. 수만 명을 가정해도 두 서비스는 같은 앱과 Redis 클라이언트를 공유하므로, 이 분리 자체가 서버나 연결 풀을 추가하지는 않습니다. 진행 기록의 보존·복구 정책은 별도로 명확히 해야 합니다.

이번에는 실제 생성·시작·진행·완료·복구 호출 경로와 저장 코드를 확인해 설명 문서를 갱신했습니다. 이 생명주기 검토 당시에는 문서만 수정해 테스트·앱 재시작을 생략했습니다. 이후 승인된 저장 타입 단순화의 변경·검증은 문서 하단에 별도로 기록했습니다.

## 패키지 코드: 세션 저장

이 패키지는 **구현 파일 6개와 테스트 파일 3개**로 구성돼 있습니다.

| 파일 | 역할 |
|---|---|
| [internal/redisstore/sessions.go](../../../internal/redisstore/sessions.go) | Quiz·Study의 상태 타입과 키를 지정하는 생성 함수 |
| [internal/redisstore/session_store.go](../../../internal/redisstore/session_store.go) | 공통 `SessionStore[T]`와 실제 `Load/Save/Delete` 구현 |
| [internal/redisstore/interactions.go](../../../internal/redisstore/interactions.go) | LLM·답안 대기, 문항 시작 시각 |
| [internal/redisstore/word_order.go](../../../internal/redisstore/word_order.go) | 어순 문제의 답안 초안 |
| [internal/redisstore/miniapp_state.go](../../../internal/redisstore/miniapp_state.go) | 손글씨 메시지 참조, Mini App 복구 정보 |
| [internal/redisstore/push_claims.go](../../../internal/redisstore/push_claims.go) | 사용자·슬롯·날짜별 중복 발송 방지 표식 |

[NewQuizSessions](../../../internal/redisstore/sessions.go#L11)는 `SessionStore[model.QuizActiveSessionState]`, [NewStudySessions](../../../internal/redisstore/sessions.go#L21)는 `SessionStore[model.StudyActiveSessionState]`를 반환합니다. 기존 호출부는 그대로이고, 두 생성 함수 모두 같은 저장 구현을 직접 구성합니다. 기존 `sessionRedis` 인터페이스를 바로 받으므로 운영의 `*redis.Client`와 테스트 대역 모두 같은 생성자를 사용합니다.

클라이언트 생성과 연결 확인은 서버의 `initRedis`가 맡습니다. 실패하면 `initInfra` → `run` → `main`으로 오류가 전달되어 서버 시작이 중단됩니다. 두 생성 함수와 저장 메서드는 초기화된 클라이언트를 받는다고 가정하며 nil 검사를 반복하지 않습니다. Redis 명령 오류는 기존처럼 error로 처리합니다. `SessionStore`의 zero value를 직접 만들어 사용하는 것은 지원하지 않습니다.

[internal/redisstore/session_store.go](../../../internal/redisstore/session_store.go#L26)의 공통 타입은 다음과 같습니다. `T`가 저장할 Quiz 또는 Study 상태 타입입니다.

```go
type SessionStore[T any] struct {
	rdb  sessionRedis
	key  func(sessionID int) string
	name string
}
```

서비스가 `Save(ctx, 77, state)`를 호출하면 전달용 객체를 거치지 않고 [실제 저장 메서드](../../../internal/redisstore/session_store.go#L58)가 실행됩니다.

```go
func (s *SessionStore[T]) Save(ctx context.Context, sessionID int, state *T) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal %s session state session_id=%d: %w", s.name, sessionID, err)
	}
	if err := s.rdb.Set(ctx, s.key(sessionID), raw, sessionWorkingSetTTL).Err(); err != nil {
		return fmt.Errorf("save %s session state session_id=%d: %w", s.name, sessionID, err)
	}
	return nil
}
```

- `json.Marshal(state)`로 상태를 JSON으로 바꿉니다.
- 생성 함수가 지정한 `s.key(77)`은 Quiz의 `session:77:working_set` 또는 Study의 `study_session:77:working_set`을 만듭니다.
- 기존 공유 Redis 클라이언트로 저장하고 만료시간을 24시간으로 설정합니다.

[Load](../../../internal/redisstore/session_store.go#L37)는 `GET → JSON 해석`을 수행하고 누락과 손상을 구분합니다. [Delete](../../../internal/redisstore/session_store.go#L70)는 해당 키를 삭제합니다. DB 복구·정답 처리·완료 조건은 서비스에 남습니다.

## 설계 문서화

- [ADR-059](../../adr/ADR-059_architecture_simplification.md)에 패키지별 조사, 현재·단계별 호출 구조, 서버 초기화·주입 정리, 최종 경계와 검증 기준을 기록하고 [ADR 목록](../../adr/ADR_from_41_to_60.md)에 연결했다.
- 기존 패키지를 활용하고 `redisstore`만 추가하는 방향이며 예시 API·종료 메서드는 구현 시 구체화한다. 20:38 문서화 시점에는 구현 미착수여서 Redis TODO와 진행 상태를 유지했고 코드·설정·런타임·데이터를 바꾸지 않았다. 이후 아래 1단계를 완료했으며 2·3단계는 미착수다.
- 문서화 당시 `go list ./...`, 로컬 링크·Mermaid 코드 블록·공백 검사를 통과했다. 동작 테스트 결과를 뜻하지 않으며, docs-only 작업이라 `make test`와 앱 재시작은 생략했다. 구현 후 검증은 아래에 별도로 기록했다.

## 범위와 결정

- [ADR-059](../../adr/ADR-059_architecture_simplification.md)의 1단계와 기존 Redis 경계 TODO를 구현했다. 상세 결정은 [ADR-060](../../adr/ADR_from_41_to_60.md)에 기록했다. 완료된 TODO 항목·파일을 제거하고 저장 계약은 이 문서에 보존했다.
- Redis 명령·키·직렬화·TTL을 `internal/redisstore`로 옮긴다. 서비스의 상태 검증·전이·DB 복구, 봇 화면 처리, 스케줄러 발송 정책은 기존 위치와 동작을 유지한다.
- 기존 미커밋 변경을 보존했다. DB 스키마·데이터·Redis 데이터 마이그레이션이나 초기화는 하지 않는다. Quiz 책임 통합과 서버 초기화 구조 전체 정리는 후속 단계다.

## 변경 파일과 역할

| 위치 | 변경 |
|---|---|
| `internal/redisstore/sessions.go` | Quiz·Study별 상태 타입과 키를 선택하는 생성 함수 |
| `internal/redisstore/session_store.go` | `SessionStore[T]`가 저장 계약을 직접 구현: JSON 읽기·저장·삭제, 손상 JSON 정리, 세션 TTL 관리 |
| `internal/redisstore/interactions.go` | 공유 `Interactions` 객체, LLM pending·답안 대기·문항 시작 시각 |
| `internal/redisstore/word_order.go` | 어순 초안 JSON·키·TTL 관리 |
| `internal/redisstore/miniapp_state.go` | 손글씨 메시지 참조와 복구 fingerprint 저장 |
| `internal/redisstore/push_claims.go` | 사용자·슬롯·날짜별 `SetNX` 발송 claim |
| `internal/model/{session_store,interaction}.go` | 저장 오류와 직렬화 형식을 노출하지 않는 상태·참조 타입 |
| `internal/service/{working_set,active_session,study_active_session,services}.go` | 타입이 있는 세션 저장 계약 주입, 기존 서비스 오류 변환·유효성 검사 유지 |
| `internal/bot/interactions.go`와 관련 handler·flow | 입력·초안·메시지·복구·시간 기록 계약을 개별 필드로 사용 |
| `internal/miniapp/handler.go` | 손글씨 메시지 조회 계약 사용, 누락·잘못된 참조·저장 오류 구분 유지 |
| `internal/scheduler/{scheduler,dispatcher}.go` | 날짜와 발송 정책은 유지하고 claim 저장 기능만 주입 |
| `internal/callback` | Redis 메시지 참조 파싱을 저장 구현으로 이동, callback·URL 규약 유지 |
| `internal/config/constants.go` | 소비처가 사라진 Redis 키 타입·상수 제거 |
| `cmd/server/{server,router}.go` | 기존 공유 Redis 연결로 저장 구현을 생성해 소비자에 주입 |
| 관련 `*_test.go` | 소비자는 기능별 저장 fake, Redis 구현은 키·직렬화·TTL·원자적 명령 계약 검증 |
| 아키텍처·E2E 계획·ADR·STATUS | 실제 경계와 조립 예시 반영, 1단계 상태 기록 |

서버의 연결 생성·종료와 `/health` Ping은 그대로다. 각 저장 구현은 기존 Redis 연결을 공유하며 별도 연결을 생성하지 않는다.

## 유지한 저장 계약

| 키 | 값 | TTL |
|---|---|---|
| `session:%d:working_set` | Quiz 상태 JSON | 24시간 |
| `study_session:%d:working_set` | Study 상태 JSON | 24시간 |
| `user:%d:llm_pending` | `1`, `q:session:question`, `study:session:order` | 10분 |
| `user:%d:active_question` | `session:index` | 1시간 |
| `handwriting:msg:%d:%d` | `chat:message` | 1시간 |
| `session:%d:word_order:%d:draft` | 선택 index 배열 JSON | 24시간 |
| `session:%d:question_start` | Unix 밀리초 | 30분 |
| `copylingo:miniapp:last_fingerprint:%d` | URL fingerprint | 24시간 |
| `copylingo:push:lock:%d:%s:%s` | `1` | 24시간 |

- LLM pending은 `GetDel`로 한 번 소비한다. 저장된 값이 잘못된 문맥 토큰이면 소비한 뒤 일반 LLM 질문으로 처리하는 기존 동작을 유지한다.
- 답안 대기 상태의 조회 후 삭제 순서는 유지한다. 이를 별도 승인 없이 `GetDel`로 바꾸지 않는다.
- working set 누락은 기존 DB 복구로 이어지고, 손상 JSON은 삭제 후 기존 corrupt 오류로 전달한다. 삭제 실패 오류도 함께 보존한다. 세션 ID·버전 검증은 서비스가 수행한다.
- 어순 초안의 손상 JSON은 best-effort 삭제 후 빈 초안으로 처리한다. 선택 index의 범위·중복 검증은 봇에 남긴다.
- 발송 claim은 같은 키에 `SetNX`를 사용한다. Redis 장애 시 경고 후 진행하고, 후속 발송 실패 시 claim을 임의로 해제하거나 재시도하지 않는 기존 정책을 유지한다.
- 세션 저장 fake도 실제 Redis처럼 상태 복사본을 반환·저장해, 메모리 참조 공유로 저장 누락이나 실패가 가려지지 않게 했다.

## 검증

- 표적 테스트: service, bot, miniapp, scheduler, redisstore 통과. callback의 메시지 참조 파서 테스트는 저장 구현 쪽으로 이동했다.
- 최종 `make test` 통과. 전용 `COPYLINGO_TEST_DATABASE_URL`이 없어 기존 PostgreSQL 통합 테스트 6개는 건너뛰었다. 저장소 SQL·스키마는 이번 작업에서 수정하지 않았다.
- `service`, `bot`, `miniapp`, `scheduler`의 production Go 코드에서 `go-redis`, raw 클라이언트 호출, Redis 키 상수, `GetDel`·`SetNX` 직접 호출이 남지 않았음을 확인했다.
- `git diff --check` 통과. 새 저장 구현과 소비자 생성자·오류 경계 및 테스트를 확인했다.
- `make restart-app` 통과. 2026-09-26 21:04 KST에 `http://localhost:8080/health`가 `healthy`를 반환했다. DB·Redis·터널을 별도로 재시작하거나 데이터를 초기화하지 않았다.
- 마지막 코드 변경은 Bot에 남아 있던 미사용 LLM TTL 상수 제거였다. 이를 포함한 전체 테스트 통과 후 앱을 재시작했다.

## 후속 문서 정리

2026-09-26에 9/25~9/26 workthrough 14개를 자료 설정, 설정 소스·LLM·TTS, 발송 규칙, 두 화자 청해, 서버·콘텐츠 경계, 아키텍처 설계·Redis 경계의 6개로 정리했다. 파일 분리·설계 export와 중복 후속 기록은 관련 본문에 흡수하고 날짜·ADR·검증 결과·미완료 항목을 보존했다. `STATUS.md`의 중복 완료 항목도 합쳤다.

이번 정리는 Markdown만 변경하므로 `make test`와 앱 재시작은 생략했다. 남은 6개 문서와 `STATUS.md`의 로컬 링크 77개, 저장소 Markdown의 삭제 파일 참조 부재, 공백·코드 블록 및 `git diff --check` 검증을 통과했다. 작업 전후 파일 해시를 비교해 이 기간의 workthrough와 `STATUS.md` 외에는 변경하지 않았음을 확인했다.

## 후속 파일 정리 (2026-09-27)

- 사용자가 승인한 구성에 따라 같은 `redisstore` 패키지 안에서 파일만 나눴다. `sessions.go`는 외부용 타입·생성자·메서드를 남겨 140→70줄, `interactions.go`는 입력·시각 상태를 남겨 216→122줄로 줄였다.
- 공통 세션 저장은 `session_store.go`(81줄), 어순 초안은 `word_order.go`(46줄), Mini App 메시지·복구 상태는 `miniapp_state.go`(80줄)로 옮겼다. 키·TTL·파서도 사용처와 함께 옮겼고 `push_claims.go`는 유지했다.
- 패키지·객체·생성자·인터페이스를 추가하지 않았다. 함수·타입·상수 선언 54개를 이동 전과 비교해 동일함을 확인했고, 기존 테스트 파일도 그대로 유지했다. 본문과 소스 링크를 새 위치로 갱신했다.
- `make test` 통과: 테스트가 있는 16개 패키지 성공. 전용 DB 환경변수 미설정으로 기존 PostgreSQL 통합 테스트 6개는 건너뛰었다. 기존 저장 계약 테스트로 이동 후 동작을 확인했으며 새 테스트는 추가하지 않았다.
- 로컬 링크 44개와 Go 코드 발췌, `git diff --check` 통과. 작업 전후 해시 비교로 승인한 소스 파일 5개와 기존 문서 3개 외의 변경이 없음을 확인했다.
- `make restart-app` 통과. 2026-09-27 13:05 KST에 `/health`의 `healthy` 응답을 확인했다. DB·Redis·터널은 별도로 재시작하지 않았다.

## 후속 저장 타입 단순화 (2026-09-27)

- 사용자 승인에 따라 `QuizSessions`·`StudySessions` 포장 타입 2개와 전달 메서드 6개를 제거했다. `sessionStateStore[T]`를 `SessionStore[T]`로 바꾸고 `Load/Save/Delete`를 공개해 서비스의 기존 저장 계약을 직접 충족하게 했다.
- 이 저장 타입 변경 당시에는 `NewQuizSessions`·`NewStudySessions` 이름과 호출부를 유지했다. 상태 모델, Redis 키·JSON·24시간 TTL, 누락·손상·삭제 오류 전달, nil 클라이언트 처리도 유지했다. 두 생성 함수의 반환 타입만 공통 저장 타입으로 바꿨으며 nil 처리의 후속 변경은 아래에 기록한다.
- 저장 메서드 본문은 타입·메서드 이름 변경을 제외하면 이전과 같음을 비교했다. 기존 테스트로 키·직렬화·TTL·손상 JSON·nil 의존성 및 서비스 연결을 검증한다.
- `sessions.go`는 70→37줄, 두 세션 저장 파일 합계는 151→122줄로 줄었다. 기존 소비자·테스트 파일은 변경하지 않았다.
- `make test` 통과: 테스트가 있는 16개 패키지 성공. 기존 PostgreSQL 통합 테스트 6개는 전용 DB 환경변수 미설정으로 건너뛰었다. 로컬 링크 32개·실제 Go 코드 발췌·공백 검사와 변경 범위 검증도 통과했다.
- `make restart-app` 통과. 2026-09-27 15:04 KST에 `/health`의 `healthy` 응답을 확인했다. DB·Redis·터널은 별도로 재시작하지 않았다.

## 초기화 보장과 생성자 단순화 (2026-09-27)

- 사용자가 명확히 한 최종 의도에 따라 Redis 클라이언트의 정상 초기화는 서버의 `initRedis`에서만 보장한다. 초기화 실패 시 기존 오류 전파로 서버 시작을 중단하므로 `sessions.go`에 추가했던 nil 검사·panic을 제거했다. 결정은 ADR-060에 반영했다.
- `NewQuizSessions`/`newQuizSessions`, `NewStudySessions`/`newStudySessions`를 종류별 공개 생성자 하나로 합쳤다. 기존 `sessionRedis` 인터페이스를 직접 받아 서버의 실제 클라이언트와 테스트 대역을 동일한 경로로 주입한다.
- `session_store.go`의 `Load/Save/Delete`에도 반복 nil 검사를 두지 않는다. 실제 Redis 명령의 오류는 계속 전달하고 zero value 사용은 지원하지 않는다.
- `sessions_test.go`에서 생성자 panic 테스트를 제거하고, 기존 키·TTL·직렬화·손상 데이터 테스트가 공개 생성자를 직접 호출하도록 바꿨다.
- `make test` 통과: 테스트가 있는 16개 패키지 성공. 기존 PostgreSQL 통합 테스트 6개는 전용 DB 환경변수 미설정으로 건너뛰었다. 문서 링크·실제 코드 발췌·공백 검사도 통과했다.
- `make restart-app` 성공 후 2026-09-27 15:22 KST에 `/health`의 `healthy` 응답을 확인했다.

## Quiz·Study 세션 이름 구분 (2026-09-27)

- `ActiveSessionState`·`ActiveSessionQuestion`은 Quiz 전용이므로 `QuizActiveSessionState`·`QuizActiveSessionQuestion`으로 바꿨다. 상태 버전 상수, 서비스·저장소 타입과 생성자, 오류, 관련 인터페이스·필드·지역 변수·테스트 이름에도 같은 기준을 적용했다. Study는 기존 `StudyActiveSession…` 이름으로 구분된다.
- 서비스·저장소 묶음의 `ActiveSession` 필드는 `QuizActiveSession`이 되어 Bot·Mini App·채점·손글씨 처리의 호출부에서도 종류가 보인다. Quiz 완료 결과는 `QuizSessionResult`·`QuizSessionWrongAnswer`로 명시했다.
- 모델·서비스의 `active_session.go`와 저장소의 `active_session_repo.go`, 각각의 테스트 파일까지 6개 파일 이름에 `quiz_`를 붙였다. 기존 문서의 파일 참조도 함께 고쳤다.
- Go 파일 33개를 토큰 단위로 비교해 식별자·주석·서식 외에는 바뀌지 않았음을 확인했다. Redis 키, JSON 필드, SQL, 상태 버전을 포함한 리터럴은 동일하고 이전 Go 식별자 참조는 남지 않았다.
- `make test` 통과: 테스트가 있는 16개 패키지 성공. 기존 PostgreSQL 통합 테스트 6개는 `COPYLINGO_TEST_DATABASE_URL` 미설정으로 건너뛰었다.
- 변경한 소스 링크와 공백 검사를 통과했다. `make restart-app` 성공 후 2026-09-27 18:34 KST에 `/health`의 `healthy` 응답을 확인했다.
