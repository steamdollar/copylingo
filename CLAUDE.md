## Copylingo

### 1. goal
- Copylingo는 듀오링고를 모방한 개인용 언어 학습 프로그램입니다.
- 듀오링고에서 언어 학습에 필수적이지 않은 기능 (e.g. 랭킹 시스템)을 덜어내고 '언어 학습'이라는 어플리케이션의 본질에 더 집중된 구현을 목표로 합니다.
- 여러 언어를 학습할 수 있도록 구현합니다.
- 공인 인증 시험의 자격증을 따는 것을 중심으로 합니다.
- 해당 언어의 문자를 아예 모르는 사람부터 전문가까지 모든 레벨을 대상으로 합니다.
- 말하기, 듣기, 읽기, 쓰기를 균등하게 학습하는 것을 목표로 합니다.

### 2. workflow
- 학습 자료는 실제 학습하려는 외국어로 작성된 아티클과, 공인 인증 시험 기출 문제를 이용합니다. (해당 작업은 ai를 통해 자동으로 수집하고 가공합니다.)
- 자격증 레벨에 따라 사용자의 학습 수준을 조정합니다. 예를 들어, 일본어 n4급이 목표라면 해당 시험을 통과할 수 있는 수준을 목표로 학습을 진행합니다.
- 매일 정해진 시간에 백엔드에서 텔레그램 봇을 통해 사용자에게 학습할 내용을 전달합니다. 이를 세션이라고 합니다.
- 각 세션은 수집한 학습 자료를 사용하되, 매번 다른 문제들을 생성/재사용해 다른 조합의 문제를 가집니다.
- **세션 구성**: 오전 15문제 / 오후 10문제, 신규 60% + 복습 40%
- **문제 유형**: 객관식, 빈칸채우기, 번역, 듣기, 독해, 어순배열 (6종)
- 각 세션 별로 사용자의 정답 여부를 추적, 기록합니다. 틀린 문제는 SRS 기반으로 복습에 높은 비율로 재출제됩니다. 

### 3. tech stack
- Go 1.25
- PostgreSQL 16 + Redis 7
- Telegram Bot API
- Gemini 3.0 Flash (문제 생성, AI 대화)

### 4. how ai agents work in this project
- **작업 로그**: 작업 완료 후 `docs/workthrough/YYMMDDhhmm_<job_done>.md` 파일 생성 필수
- **역할 분담**: Claude Code는 테크 리드 (아키텍처, 설계). 단순 구현은 Gemini에게 프롬프트와 함께 위임 제안
- **협업 방식**: Claude가 프롬프트 작성 → 사용자가 Gemini에 복붙하여 실행
- **TODO 위임 프로토콜**: 디테일이 필요한 TODO는 `docs/todos/<task>.md`에 자기완결적 문서로 분리하고 `STATUS.md`에는 한 줄 요약 + 문서 링크만 둠. 작성/실행/완료 처리 규칙은 `AGENTS.md`의 "TODO 문서 프로토콜" 섹션 참조.

### 5. 필수 규칙
1. **작업 시작 전**: `AGENTS.md` (규칙 확인) → `STATUS.md` (현재 작업)
2. **작업 완료 후**:
   - `make test` 실행하여 테스트 통과 확인 (필수)
   - `STATUS.md` 업데이트 (진행 중 → 완료, 다음 작업 설정)
   - `docs/workthrough/YYMMDDhhmm_<job>.md` 생성
   - 새 의사결정 시 `docs/ADR.md`에 기록
   - 마일스톤 완료 시에만 `ROADMAP.md` 업데이트
3. **코딩 규칙**: `AGENTS.md` 참조 (DB, 에러 처리, 텔레그램 콜백 규약 등)
4. **주의**: `config.yaml`에 API 키/토큰 하드코딩 금지 → 환경변수로 주입

### 5.1. 에러 처리/로깅 정책
- 에러 발생 지점에서는 로그를 찍지 말고 `fmt.Errorf("context: %w", err)` 패턴으로 맥락을 붙여 반환합니다.
- Repository 계층은 함수명/주요 식별자 기반으로 검색 가능한 에러 컨텍스트를 포함합니다. 예: `SessionQuestionRepository.GetBySession session_id=%d: %w`
- Service 계층은 새로운 비즈니스 의미를 추가할 때만 래핑합니다. 단순 repository pass-through 함수는 그대로 반환합니다.
- `err`를 이후에 재사용하지 않으면 `if err := ...; err != nil` 또는 `if _, err := ...; err != nil` 형태로 스코프를 좁힙니다.
- Repository 같은 하위 계층에서는 직접 로그를 찍지 않고, Bot handler/HTTP handler/scheduler job 같은 경계 계층에서 사용자/작업 맥락과 함께 한 번만 로그를 출력합니다.

### 6. 개발 명령어
```bash
make infra      # PostgreSQL + Redis 시작
make run        # 앱 실행
make test       # 전체 테스트
make build      # 바이너리 빌드
```
