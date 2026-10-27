# CopyLingo 의사결정 기록 (ADR)

## ADR-081: 학습 로직에 언어별 분기를 두지 않는다

- 날짜: 2026-10-26
- 상태: 승인됨
- 배경:
  - 레벨 순서·인접 레벨·레벨 통계 정렬이 일본어 전용으로 짜여 있었다. service에 `japaneseProficiencyLevels`·`jlptLevelRank`와 `if language != "ja"`·`if user.Language == "ja"` 분기가 있었다. 일본어가 아닌 언어는 오류 없이 다른 경로(그 레벨만 범위, 정렬 없음)를 탔다.
  - 2026-10-26 수정(ADR-079 정정)에서 레벨 순서를 언어 key map `proficiencyLevels`로 옮기고, `levelRank`·`neighborLevels`·`sessionLevelsFor`가 이 map만 읽게 바꿨다. 이 방식을 이후 모든 작업의 규칙으로 고정한다.
- 결정:
  - 로직 코드(service·bot·scheduler·repository·miniapp)는 학습 언어나 레벨 이름으로 분기하지 않는다. 언어별 사실은 언어를 key로 하는 data에 두고, 로직은 언어를 인자로 받아 조회한다.
  - data에 없는 언어·레벨은 오류나 예외 분기가 아니라 중립 기본값으로 처리한다(그 레벨만 범위, 순위 맨 뒤).
  - 일부 언어에만 있는 기능(가나 손글씨, 한자 해설)은 언어 코드가 아니라 문제 유형이나 data 유무로 켠다.
  - 원칙은 AGENTS.md §4 원칙 4에, 구현 규칙은 CONVENTIONS.md "Learning-language neutrality"에 둔다.
- 버린 대안:
  - 언어가 늘 때마다 분기 추가: 언어마다 로직 경로가 갈라져 테스트·검토 범위가 언어 수만큼 늘어난다.
  - 언어별 strategy interface 구현체: 지금 언어 간 차이는 data(레벨 순서·콘텐츠)뿐이라 과하다. 편성 규칙 자체가 언어마다 달라질 때 다시 검토한다.
- 결과:
  - 언어 추가는 data 추가(레벨 순서 배열, seed 파일)로 끝나고, 같은 편성 규칙이 그대로 적용된다.
  - 레벨·언어 로직 테스트는 data에 없는 언어·레벨 사례를 포함한다(`TestNeighborLevels`의 `en`/`B1`, `ja`/`N0`).
  - 단일 언어 외부 출처 adapter(`internal/pipeline/fetcher_nhk.go`)의 상수와 CLI 기본값(`cmd/admin/generate_listening_audio`의 `-language ja`)은 로직 분기가 아니므로 허용한다.

## ADR-082: 청해 음성 생성(TTS)은 서버 밖 admin 도구에서만 한다

- 날짜: 2026-10-26
- 상태: 승인됨 (ADR-079의 "scheduler가 Study·Quiz 범위 레벨 음성을 채운다"와 ADR-031의 "하루 몇 개씩 잔고 채움"을 대체)
- 배경:
  - 서버 scheduler가 30분 tick마다 사용자 레벨별로 `AudioService.TopUpSessionAudio`를 불러 TTS(LLM vendor 호출)를 돌렸다. 사용자 요청과 무관한 배치 작업이 서버 프로세스 안에 있었다.
  - 같은 일을 하는 `cmd/admin/generate_listening_audio`가 이미 있었고, 청해 150문항 전부 음성이 있었다. 청해 문항은 seed로만 늘어나므로 생성 시점도 seed 직후로 정해진다.
  - 서버가 TTS client를 들고 있어서 음성 전송(MinIO 읽기)까지 LLM API key 유무에 묶였다(key 없으면 `audio`가 nil).
- 결정:
  - 서버에서 음성 생성을 지운다. scheduler의 `topUpAudio`·`Deps.Audio`, `AudioService.TopUpSessionAudio`를 삭제한다.
  - `AudioService`를 둘로 나눈다. 서버용 `AudioService`는 저장된 클립 읽기(`GetClip`)와 Telegram file_id 저장(`CacheFileID`)만 한다. 생성은 `AudioGenerator.TopUpAudio`가 맡고 admin 도구만 쓴다.
  - admin 도구는 `-level` 없이 한 언어의 모든 레벨에서 음성 없는 청해를 채운다(`GetListeningNeedingAudio(language, limit)`).
  - `bootstrap`에는 둘이 같이 쓰는 S3 store 조립(`NewAudioStore`)만 남긴다. TTS client 조립은 admin 도구 안으로 옮긴다.
- 버린 대안:
  - scheduler top-up 유지: seed를 잊어도 언젠가 채워지는 안전망이지만, 서버가 배치 TTS와 vendor rate-limit·key를 계속 떠안는다.
  - seeder가 seed 직후 TTS까지 호출: 명령 하나로 끝나지만 seed가 vendor 장애·rate-limit에 묶인다. seed는 DB만 다루게 둔다.
  - 서버 `AudioService`에 TTS client를 nil로 넘기기: 분리 없이 끝나지만 nil 분기가 늘어난다.
- 결과:
  - 새 청해 문항을 seed한 뒤 `go run ./cmd/admin/generate_listening_audio -language ja`를 실행해야 한다. 잊으면 그 문항은 오류 없이 출제에서 빠진다(README 설치 절차에 명시).
  - 서버의 음성 전송은 LLM API key와 무관하게 항상 조립된다. 서버가 쓰는 LLM은 채점과 학습 질문 답변뿐이다.

## ADR-083: LLM API key를 필수로 하고, subjective 문항 타입을 삭제한다

- 날짜: 2026-10-27
- 상태: 승인됨
- 배경:
  - key가 없으면 config는 WARN만 남기고 서버를 띄웠다. LLM client의 guard는 `client == nil || model == ""`를 봤는데, 둘 다 운영에서 비지 않아 guard가 동작하지 않았다. 그래서 key가 없으면 빈 key로 vendor를 호출해 인증 실패가 일반 에러로 나갔고, `ErrAIConfigMissing` → `ErrAIUnavailable` 경로(오답 처리 후 진행, Mini App 503 "설정 사용 불가")는 쓰이지 않았다.
  - key 없이 서버를 돌리는 곳이 없다(CI 없음, 서버를 띄우는 테스트 없음, compose는 env를 그대로 넘김). Telegram token은 이미 필수다.
  - subjective(LLM 의미 채점) 문항은 seed에도 DB에도 0건이었다. 서버의 LLM 사용처는 손글씨 채점과 학습 질문 답변뿐이다(TTS는 ADR-082로 서버 밖).
- 결정:
  - `llm.api_key`가 비면 config 검증이 실패해 서버(와 config를 읽는 모든 cmd)가 시작하지 않는다.
  - `ErrAIConfigMissing`·`ErrAIUnavailable`·LLM client guard 3곳·Mini App 503 매핑을 지운다.
  - subjective 타입을 지운다: `model.QuestionSubjective`, `GradeAnswer`(LLM), grader 분기, 오답 처리 fallback(`GradingUnavailable`), 입력 중 표시 hook(`OnAIGrading`), AI 피드백 문구. 텍스트·선택지 답은 모두 exact-match다.
  - key가 있지만 무효(만료·폐기·한도 초과)한 경우는 런타임 에러로 처리한다. 손글씨는 기록 없이 에러를 돌려 재제출할 수 있다.
- 버린 대안:
  - `llm.enabled` 스위치(true면 key 필수, false면 LLM 기능 끔): key 없이 돌릴 실제 수요가 없다. 필요해지면 기본값 true로 추가하면 기존 설정이 깨지지 않는다.
  - guard가 key를 보도록 3줄 수정: key 없이 도는 상태를 계속 지원해야 하고, 그 상태에서 손글씨 문항이 막히는 문제가 남는다.
  - 시작 시 vendor를 호출해 key 검증: vendor 장애 때 서버 재시작까지 막힌다.
- 결과:
  - seeder·admin 도구도 config를 읽으므로 key가 필요하다(Telegram token과 같은 조건).
  - subjective 문항을 다시 넣으려면 LLM 채점 경로와 실패 정책을 새로 정해야 한다.

## ADR-084: 조립부가 채우는 의존에는 nil 방어 분기를 두지 않는다

- 날짜: 2026-10-27
- 상태: 승인됨 (ADR-059 §8.7 "nil 방어" 행의 "store nil 분기는 유지"를 대체)
- 배경:
  - ADR-059 §8.7은 service 의존의 nil 분기만 지우고 store nil 분기는 남겼다. 남긴 이유는 적혀 있지 않았고, 같은 ADR의 테스트 조립 원칙("빠진 의존은 nil로 남아 호출 시 바로 실패")과 어긋났다.
  - 운영 조립(`cmd/server`)은 모든 store·repo·limiter·claims·Mini App 화면 갱신자를 항상 채운다. nil 분기 약 37곳(service `Err{Quiz,Study}ActiveSessionDependencyMissing` 12곳, bot store 15곳, scheduler 7곳, Mini App 1곳, 청해 음성 1곳)은 일부만 채운 테스트에서만 실행됐다.
  - 몇몇 테스트는 nil repo를 "DB에 세션 없음" 대신 썼다. 분기가 조립 누락을 조용히 삼켜 버리므로, 실제 누락이 생기면 기능이 꺼진 채 로그만 남는다.
- 결정:
  - 생성자·조립부가 채우는 의존에는 nil 검사를 두지 않는다. 빠지면 첫 호출에서 nil panic으로 바로 드러난다.
  - 위 37곳과 sentinel 2개를 지운다. scheduler `claims`도 운영에서 항상 채워지므로 같은 기준으로 지운다.
  - 테스트는 필요한 의존을 fake로 채운다. "DB에 세션 없음"은 `sql.ErrNoRows`를 돌려주는 fake repo로 표현한다(bot 테스트 helper는 Quiz repo 기본값으로 둔다).
- 버린 대안:
  - store nil 분기 유지: 운영에서 실행되지 않는 경로를 테스트가 계속 고정하고, 조립 누락을 숨긴다.
  - 생성자에서 nil 검사 후 panic: 조립 누락을 시작 시점에 잡지만 생성자마다 검사 코드가 늘어난다. 조립부는 한 곳이고 첫 호출 panic으로 충분하다.
- 결과:
  - 데이터 조건 검사(예: 청해 문항의 `AudioPath` 없음)는 의존 nil 검사가 아니므로 남긴다.
  - 선택 의존이 다시 생기면 nil 대신 no-op 구현을 넣는다.
