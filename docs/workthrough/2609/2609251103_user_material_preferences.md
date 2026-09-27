# 사용자별 자료 유지 복습·학습 제외

2026-09-25 11:03 자료 설정 구현과 15:15 Quiz 연결 자료 설정 후속 작업을 통합한 기록이다.

## 결과와 이유

사용자는 Study 카드에서 자료를 **일반 학습**, **이미 알아요 · 드물게 복습**, **학습에서 제외** 중 하나로 설정할 수 있다. 연결 자료가 있는 Quiz 문제에서는 **⚙️ 연결 자료 설정**을 눌러 **유지 복습** 또는 **학습에서 제외**를 선택한다. 두 진입점은 같은 사용자별 설정을 저장한다. 설정 메뉴에서는 유지·제외 자료를 8개씩 확인하고 일반 학습으로 복원할 수 있다. 이전에는 Study의 자료별 복습일과 Quiz의 문제별 복습일이 따로 움직여, 이미 아는 자료의 다른 연결 문제가 다시 출제될 수 있었다. 이제 새 Study와 연결 Quiz가 자료별 공통 기한 `next_check_at`을 함께 확인한다. 이미 생성된 대기·진행 세션은 그대로 둔다. 설계 근거와 절충은 [ADR-051](../../adr/ADR-051_user_material_preferences.md)에 있다.

## 실행 주체와 상태 소유권

| 실행 주체 | 책임·수명·경계 |
| --- | --- |
| 사용자와 Telegram 대화 | 사용자가 카드 버튼을 누르면 Telegram Bot API에 callback이 생기고 Go 앱이 [update를 polling](../../../internal/bot/handler.go#L75)해 받는다. Telegram 메시지는 화면이고 자료 설정의 원본은 아니다. |
| Go 앱의 Bot callback 처리 | callback마다 [Bot 라우터](../../../internal/bot/handler.go#L335)와 Quiz/Study/설정 handler가 실행된다. 사용자·세션·자료를 확인하고 서비스·저장소를 호출한다. 서비스와 저장소는 별도 프로세스가 아닌 이 앱의 코드다. |
| Go 앱의 스케줄러 작업 | 예약 시점마다 새 세션을 편성·발송한다. 현재 코드에서는 Bot과 스케줄러가 [같은 앱 프로세스에서 시작](../../../cmd/server/server.go#L55)한다. 배포 복제본 수는 [UNKNOWN: deployment replica count not confirmed]. |
| PostgreSQL | `(user_id, material_id)`별 설정과 공통 기한, 기존 Study/Quiz 이력, 생성된 세션을 영속 저장한다. 앱과 DB 사이에 네트워크 경계가 있다. |
| Redis | 진행 중인 Study/Quiz 세션의 임시 작업 상태를 보관한다. 설정 버튼 자체는 Study 진행 상태를 바꾸지 않는다. |

PostgreSQL의 [설정 테이블](../../../migrations/001_init.sql#L95)에는 유지·제외일 때만 행이 있다. 행이 없으면 일반 학습이다. 이 테이블은 **사용자 의도**를, 기존 `user_material_progress`와 `user_question_progress`는 **실제 학습 이력**을 소유한다. 따라서 미학습 자료에 유지 설정을 해도 학습 완료 이력이 생기지는 않는다. 모드와 30일 초기값·180일 상한은 [모델](../../../internal/model/material_preference.go#L5)에 정의돼 있다.

## Quiz 문제에서 연결 자료를 설정하면

예를 들어 사용자 42가 Quiz 세션 77의 문제 5에서 **⚙️ 연결 자료 설정**을 눌러 **유지 복습**을 고른다고 하자. 문제 5가 자료 10에 연결돼 있다고 가정한다.

1. **Go 앱의 Bot 문제 화면**은 연결 자료 ID가 있는 문제에만 [설정 버튼](../../../internal/bot/session_question.go#L74)을 붙이고 `q:77:policy:5` callback을 만든다. **사용자와 Telegram**은 이 callback을 Go 앱에 보낸다. 답변 후 [텍스트 결과](../../../internal/bot/session_answer.go#L204), [단어 배열 편집 화면](../../../internal/bot/word_order.go#L290), [손글씨 Mini App 제출 후 메시지](../../../internal/miniapp/handler.go#L270)에도 같은 버튼이 남는다.
2. **Go 앱의 Bot 라우터**는 `q:` callback을 [Quiz 라우터](../../../internal/bot/session_flow.go#L214)로 보내고, `policy` 동작을 [설정 handler](../../../internal/bot/session_flow.go#L259)에 넘긴다. **Go 앱의 설정 handler**는 [진행 중 Quiz 상태](../../../internal/bot/material_preference.go#L21)를 Redis에서 읽거나 DB에서 복구해 세션 소유자가 사용자 42인지, 문제 5가 세션에 있는지, 연결 자료가 있는지 확인한다. callback의 자료 ID를 신뢰하지 않고 서버에 보관된 문제의 자료 ID 10을 사용한다.
3. **Go 앱의 설정 handler**는 [현재 설정을 조회](../../../internal/bot/material_preference.go#L59)한 뒤 원래 Quiz 메시지를 그대로 두고 별도 Telegram 메뉴에 **유지 복습 / 학습에서 제외** 두 버튼을 보낸다. 메뉴를 열기만 해서는 설정이 저장되지 않는다. 예전에 발송된 `q:77:exclude:5` 버튼도 이 메뉴를 연다.
4. **사용자와 Telegram**이 **유지 복습**을 고르면 `q:77:policy:5:maintenance` callback이 Go 앱에 전달된다. **Go 앱의 설정 handler**는 세션 소유권·문제 포함 여부를 다시 확인한다. **Go 앱의 설정 서비스**는 [사용자 ID·자료 ID·모드](../../../internal/service/material_preference.go#L33)를 검사한다. **PostgreSQL**은 [사용자 42·자료 10의 설정](../../../internal/repository/material_preference_repo.go#L49)을 `maintenance`로 저장하고 `next_check_at`을 30일 뒤로 정한다. **학습에서 제외**를 고르면 `excluded`를 저장한다. **Go 앱의 설정 handler**는 별도 메뉴 메시지를 확인 문구로 바꾸며 Quiz의 답안·현재 문제·진행 상태는 바꾸지 않는다.
5. **Go 앱의 스케줄러 작업**이 다음 세션을 만들 때 **PostgreSQL**은 [Study 후보](../../../internal/repository/material_repo.go#L128)와 [연결 Quiz 후보](../../../internal/repository/question_repo.go#L123)에 자료 10의 설정을 적용한다. 유지 복습 자료는 기한 전까지, 제외 자료는 복원할 때까지 후보에서 빠진다. 이미 생성된 세션 77의 문제 5는 계속 풀 수 있다. 사용자는 나중에 설정 목록에서 일반 학습으로 복원할 수 있다.

처음 구현한 Quiz 버튼은 즉시 제외를 저장했지만, 15:15 후속 작업에서 두 선택지를 먼저 보여 주는 메뉴로 바꿨다. 이전 `exclude` callback도 같은 메뉴로 연결한다.

버튼은 `material_id`가 없는 Quiz 문제에는 없다. Quiz 메뉴의 `현재: 일반 학습`은 상태 표시이며 선택지가 아니다. 일반 학습으로 복원하려면 [설정 목록](../../../internal/bot/material_preference.go#L204)을 사용한다. [Bot 테스트](../../../internal/bot/material_preference_test.go#L127)는 다른 사용자·세션 밖 문제·연결 자료 없는 문제의 거부, 메뉴만 열었을 때 저장하지 않는지, 두 선택지와 현재 Quiz 상태 보존, 버튼 노출을 확인한다.

## 한 자료를 유지 복습으로 바꾸면

예를 들어 사용자 42가 Study 세션 77의 자료 10에서 **이미 알아요**를 누른다고 하자.

1. **사용자와 Telegram**은 카드의 [학습 설정 버튼](../../../internal/bot/study_flow.go#L229)을 통해 `study:77:policy:10` callback을 Go 앱에 보낸다. [Study 라우터](../../../internal/bot/study_flow.go#L38)는 설정 화면과 모드 선택 callback을 설정 handler로 전달한다.
2. **Go 앱의 Bot handler**는 [세션 77이 사용자 42의 Study인지, 자료 10이 그 세션에 있는지](../../../internal/bot/material_preference.go#L96) 확인한다. 모드 선택 시 [설정 서비스](../../../internal/service/material_preference.go#L33)가 ID와 모드를 검사한 뒤 저장소에 전달한다. 메뉴 열기와 모드 변경은 카드의 `studied` 상태를 변경하지 않는다.
3. **PostgreSQL**은 [설정 저장소의 upsert](../../../internal/repository/material_preference_repo.go#L47)로 사용자 42·자료 10에 `maintenance`와 현재 시각에서 30일 뒤의 `next_check_at`을 기록한다. 같은 모드를 다시 누르면 기한을 연장하지 않는다. `excluded`는 출제에서 제외하고, `normal`은 설정 행만 삭제한다. 공유 자료와 기존 학습 이력은 유지된다.
4. **Go 앱의 스케줄러 작업**은 다음 새 세션을 만들 때 [Study 자료 조회](../../../internal/repository/material_repo.go#L102) 또는 [Quiz 문제 조회](../../../internal/repository/question_repo.go#L100)를 호출한다. PostgreSQL은 제외 자료와 기한 전의 유지 자료를 후보에서 빼며, 신규·복습 순위와 부족분 보충에도 같은 조건을 적용한다. 자료 ID가 없는 문제는 이 정책의 대상이 아니다.
5. **PostgreSQL**은 기한이 지난 유지 자료에서 기존 문제별 복습 기한과 독해 선행 학습·청해 음성 준비 조건까지 맞는 [대표 Quiz 문제 한 개](../../../internal/repository/material_preference_repo.go#L89)를 우선 후보로 낸다. 그런 문제가 없으면 [Study 자료 후보](../../../internal/repository/material_repo.go#L128)가 될 수 있다. **Go 앱의 Quiz 세션 조립기**도 현재·인접 레벨 등의 별도 조회 결과를 합칠 때 [같은 유지 자료의 중복 문제](../../../internal/service/session_builder.go#L125)를 막는다. 그 후 새 세션의 자료·문제 목록이 DB에 저장된다.

`next_check_at`은 자료 전체의 공통 출제 문턱이다. 기한이 왔다고 기존 자료·문제 SRS 일정을 앞당기지는 않는다. 허용되는 후보가 부족하면 세션이 짧아질 수 있다. [자료 후보 테스트](../../../internal/repository/material_preference_repo_test.go#L185)와 [세션 조립 테스트](../../../internal/service/session_builder_material_preference_test.go#L11)가 사용자별 차단, 신규·복습·복습 개수, 대표 문제와 중복 제한을 확인한다.

## 세션을 마치면

- **사용자와 Telegram**이 Quiz 답안을 제출하면 Go 앱은 진행 중 답안과 문제별 SRS를 [Redis 작업 상태](../../../internal/service/quiz_active_session.go#L145)에 기록한다. **Go 앱의 완료 서비스**는 모든 문제가 답변됐는지 확인한 뒤 [완료 저장소](../../../internal/service/quiz_active_session.go#L180)를 호출한다.
- **PostgreSQL**은 [한 Quiz 완료 트랜잭션](../../../internal/repository/quiz_active_session_repo.go#L195)에서 답안·문제별 이력과 자료 설정을 기록한다. 같은 유지 자료의 답이 모두 맞으면 간격을 30→60→120→180일(상한)로 늘리고, 하나라도 틀리면 설정 행을 지워 일반 학습으로 돌린다. [결과 SQL](../../../internal/repository/quiz_active_session_repo.go#L242)은 세션 생성 시점이 현재 설정 시점과 당시 기한 이후인지 확인한다. 완료 재시도는 이미 완료한 세션을 다시 반영하지 않는다.
- **사용자와 Telegram**이 연결 Quiz가 없어 배정된 Study 카드를 넘기면 Go 앱은 [Redis 작업 상태에 읽음](../../../internal/service/study_active_session.go#L177)을 표시한다. **PostgreSQL**은 [Study 완료 트랜잭션](../../../internal/repository/study_active_session_repo.go#L144)에서 학습 이력을 저장하고, 해당 유지 자료의 [다음 기한만 같은 간격으로 미룬다](../../../internal/repository/study_active_session_repo.go#L191). 읽었다는 사실로 Quiz 정답을 추정해 간격을 늘리지 않는다.

사용자가 나중에 [설정 목록](../../../internal/bot/material_preference.go#L193)에서 복원을 누르면 Bot handler가 사용자 ID로 [페이지 목록과 복원](../../../internal/bot/material_preference.go#L204)을 처리한다. PostgreSQL은 그 사용자의 설정 행만 삭제한다. 새 세션부터 일반 학습 후보가 되며 과거 SRS 이력은 유지된다. [Bot 테스트](../../../internal/bot/material_preference_test.go#L290)는 페이지 이동과 사용자별 복원을 확인한다. Study 소유권·자료 membership·진행 상태 보존은 [별도 Bot 테스트](../../../internal/bot/material_preference_test.go#L224)가 확인한다.

## 검증과 실행 반영

- 새 스키마 전체를 격리 DB에서 transaction으로 실행한 뒤 rollback: 통과.
- 로컬 DB에는 새 `user_material_preferences` 생성문만 additive하게 적용했다. 기존 progress와 사용자 데이터는 변경하지 않았다.
- PostgreSQL 회귀 테스트는 별도 빈 DB `copylingo_preferences_test_260925`에서 임시 테이블로 실행했다. credential은 환경으로만 전달하고 출력하지 않았다. 검증 후 이 임시 DB와 credential 주입용 임시 실행 스크립트를 제거했다.
- `go test ./internal/bot ./internal/config ./internal/callback`: 통과. 소유권·자료 membership·페이지 목록과 사용자별 복원·HTML escape·잘못된 callback·진행 상태 보존을 검증했다.
- `COPYLINGO_TEST_DATABASE_URL=<격리 DB 연결> go test ./internal/repository -count=1`: 통과. 실제 SQL로 설정·복원·기한·선정·정답/오답·기존 세션·재시도 동작을 검증했다.
- 초기 전체 테스트 통과 후 최종 검토에서 다른 레벨의 문제가 같은 유지 자료에 연결된 경우의 중복 편성 가능성을 발견했다. 세션 조립 단계의 자료별 제한과 회귀 테스트를 추가했다.
- 최종 `COPYLINGO_TEST_DATABASE_URL=<격리 DB 연결> make test`: 통과, 테스트가 있는 16개 패키지 성공, 하위 테스트 포함 PASS 692건, SKIP 0건. 실제 PostgreSQL 테스트도 포함했다.
- `git diff --check`: 통과.
- `make restart-app`: 성공. 2026-09-25 11:14 KST `http://localhost:8080/health`가 `healthy`를 반환했다. DB·Redis 자체는 재시작하지 않았다.
- Quiz 연결 자료 설정 메뉴 작업에서 `make test`와 `git diff --check`가 통과했다. 기본 `make test`에서는 PostgreSQL 전용 6개 테스트가 테스트 DB 환경 변수 미설정으로 건너뛰었고, 로컬 PostgreSQL을 이용한 관련 통합 테스트 3개는 별도로 통과했다. `make restart-app` 후 로컬 `/health`가 준비 상태를 반환했다. 이 메뉴 변경에는 SQL 수정이 없다.
- 로컬 설정 테이블은 검증 후에도 0행이다. 테스트를 위해 실제 사용자 설정·학습 이력을 생성하거나 수정하지 않았다.
- 실제 Telegram 대화에는 시험 메시지를 보내지 않았다. UI는 bot 테스트로 검증했다.

이 문서는 두 완료 변경의 코드 경로를 읽어 현재 Quiz 선택 메뉴까지 반영했다. 이번 수정은 문서에만 해당하므로 `make test`를 다시 실행하지 않았다. 위 검증 기록은 각 구현 작업의 결과다.

## 제약과 복구

- `material_id` 없는 문제(현재 청해 seed 등)는 이번 설정 대상이 아니다.
- 새로 생성되는 세션부터 적용하므로 기존 대기 세션에서 제외 자료가 보일 수 있다.
- 설정 메뉴에서 일반 학습으로 복원해도 기존 이력과 SRS는 유지한다.
- 배포를 되돌려야 하면 새 테이블과 설정은 보존한다. 이전 바이너리는 설정을 적용하지 않으므로 앱 rollback은 정책 중단 효과가 있다. 데이터 삭제로 복구하지 않는다.
