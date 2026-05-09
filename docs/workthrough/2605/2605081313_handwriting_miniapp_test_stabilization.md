# 손글씨 Mini App 테스트 안정화 기록

## 작업 배경

손글씨 가나 Mini App을 실제 Telegram 세션에서 테스트하면서 공개 URL 설정, 진행 중 세션 복구, Gemini 모델 설정, 손글씨 채점 품질, 세션 결과 표시 기준을 연속으로 점검했습니다.
개별 문제를 처리할 때마다 작은 workthrough가 많이 생겼기 때문에, 최종적으로 의미 있는 흐름만 하나의 문서로 합쳤습니다.

## 최종 반영 내용

### 1. Mini App 공개 URL 환경 정리

- `.env`에 `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 추가 (telegram mini app이 볼 public url, 이 url이 로컬 머신으로 요청 전달) 
- `docker-compose.yml`의 `app` 서비스에도 `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 전달하도록 추가했습니다.
- `make tunnel`로 Cloudflare Quick Tunnel URL을 `.env`에 자동 반영하고, 서버가 해당 값을 읽도록 재시작했습니다.

### 2. 진행 중 세션 재개 지원

- `status = 'in_progress'` 세션 조회를 repository/service에 추가했습니다.
- `/menu -> 학습하기` 진입 시 pending 세션보다 in-progress 세션을 먼저 확인하도록 변경했습니다.
- 진행 중 세션이 있으면 `session_questions.is_correct IS NULL`인 첫 문항부터 다시 보여줍니다.

이 변경으로 손글씨 문항이 설정 문제로 한 번 막혀도, 설정을 고친 뒤 같은 세션을 이어서 진행할 수 있습니다.

### 3. Gemini 모델 설정 정리

- 실제 모델 목록과 OpenAI 호환 `chat/completions` 호출을 확인한 뒤, 최종 모델을 `gemini-3.1-flash-lite`로 설정했습니다.
- `.env`, `config.yaml`, `internal/config/config.go`, `README.md`의 모델 예시를 같은 값으로 맞췄습니다.
- 텍스트 요청과 `image_url` 포함 요청이 모두 성공하는 것을 확인했습니다.

### 4. 손글씨 채점 품질 및 지연 보정

- stroke 렌더링 PNG 기본 크기를 `256x256`으로 키웠습니다.
- 큰 캔버스에서 획이 너무 가늘게 보이지 않도록 brush 크기를 보정했습니다.
- Gemini 프롬프트에 단일 가나 손글씨, 초보 학습자 기준, 모바일 손글씨 허용 기준을 명시했습니다.
- 이미지 detail은 품질 확인 중 `high`까지 올렸으나, 지연을 줄이기 위해 최종적으로 `auto`로 조정했습니다.
- 손글씨 제출 경로에 단계별 지연 로그를 추가했습니다.

로그 예시:

```text
[Handwriting] llm model=... elapsed=...
[Handwriting] grader total=... llm=... record=...
[Handwriting] service total=... render=... grade=...
[Handwriting] submit total=...
```

### 5. 손글씨 문항과 결과 표시 기준 정리

- 손글씨 문항 프롬프트에 히라가나/가타카나 중 무엇을 써야 하는지 명시했습니다.
- 기존 로컬 DB의 `kana_handwriting` 문항 208개도 프롬프트와 해설을 보정했습니다.
- `questions.correct_answer`는 계속 실제 가나 문자로 유지합니다.
- `session_questions.is_correct`가 최종 정오답 판정의 기준입니다.
- `session_questions.user_answer`는 제출 표시 용도로 유지하며, 손글씨는 `handwriting:submitted`로 저장합니다.
- 세션 결과의 틀린 문제 목록은 repository에서 `is_correct = FALSE`인 row만 조회하고, UI 조립 단계에서도 `is_correct` guard를 추가했습니다.
- 손글씨 오답은 제출 문자열을 표시하지 않고 문제와 정답만 보여줍니다.

## 현재 스키마 역할

```text
questions.correct_answer
= 문제의 기준 정답. 손글씨 문항에서는 실제 히라가나/가타카나 문자.

session_questions.user_answer
= 사용자가 제출한 값 또는 제출 방식 표시.

session_questions.is_correct
= 최종 정오답 판정. 세션 결과, 통계, SRS의 기준.
```

## BotFather 도메인 관련 확인

현재 구현은 BotFather에 고정 Mini App 도메인을 설정해서 여는 방식이 아니라, inline keyboard의 `web_app.url`에 tunnel HTTPS URL을 직접 넣습니다.
따라서 테스트 환경에서는 BotFather 도메인 설정 없이도 동작했습니다.

다만 운영 환경에서는 고정 도메인과 BotFather Mini App/Web App domain을 맞추는 것이 안전합니다.
Quick Tunnel 임시 URL은 재시작 시 바뀌므로 운영용으로 적합하지 않습니다.

## 검증

```bash
go test ./...
```

- 전체 테스트 통과
- 서버 재기동 후 `/health` 정상 응답 확인
- tunnel URL 경유 Mini App 페이지 응답 확인
- Gemini OpenAI 호환 텍스트/이미지 요청 성공 확인

## 제외한 세부 기록

아래 내용은 최종 설계 결정이 아니거나 지나치게 전술적인 중간 기록이라 통합 문서에는 상세히 남기지 않았습니다.

- `gemini-2.5-flash`로 잠시 전환했던 중간 단계
- `image_url.detail`을 `high`로 올렸다가 `auto`로 다시 낮춘 세부 이력
- 손글씨 `user_answer`에 실제 정답 가나 또는 `handwriting:x`를 저장하려 했던 중간 시도
- tmux window를 재생성하며 서버를 재시작한 반복 절차
- 개별 curl/psql 확인 명령의 전체 출력
- 기존 DB의 소수 테스트 row를 수동 보정한 상세 내역

최종 기준은 `is_correct`를 정오답의 단일 기준으로 두고, `user_answer`는 표시/제출 기록으로만 사용하는 구조입니다.
