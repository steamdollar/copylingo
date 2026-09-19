# Workthrough: JLPT N4 vocab_usage (용법) 동어 반복 및 자명한 오답 선지 개선

## 작업 배경
- JLPT N4 `vocab_usage`(단어 용법) 120개 문항에서, 문제 지문(prompt)에 제시된 단어(예: 「とうとう」, 「諦める」 등)가 정답 선택지 단 하나에만 포함되어 있고 나머지 3개 오답 선지에는 해당 단어가 아예 누락되거나 무관한 단어로 치환되어 있는 결함 발견.
- 이로 인해 일본어를 학습하지 않고도 문제의 한자/단어 표기가 들어간 보기 하나만 찾으면 무조건 정답이 되는 자명한(Trivial) 문제가 발생함.

## 작업 내용
1. **문제 전수 분석**:
   - `questions` 테이블 및 `cmd/ja/catalog/data/n4_question_seeds.json` 내 `item_type = 'vocab_usage'` 120문항 전수 분석.
   - 120개 문항 중 119개가 오답 선지에 목표 단어/어간이 누락되어 정답 1개만 매칭되는 상태 확인.
2. **JLPT N4 용법 문제 규격으로 전면 재작성**:
   - 4개 선택지 모두에 목표 단어(또는 올바른/잘못된 활용형)가 반드시 포함되도록 구성 (글자 찾기 치팅 원천 차단).
   - 1개의 올바른 문맥(정답)과 3개의 명백한 문맥 오류(다른 단어가 와야 할 자리에 잘못 쓰여 어색한 문장)로 구성.
   - 복수 정답 가능성(False Distractor)을 배제하기 위한 엄격한 2단계 품질 감사 및 정제 수행.
   - 정답 해석 및 3개 오답 각각의 오용 원인과 대체 어휘를 명시한 상세 한국어 해설 보강.
   - 옵션 위치의 편중을 방지하기 위해 정답 위치를 1~4번에 고르게 셔플 (1번: 21, 2번: 28, 3번: 35, 4번: 36).
3. **시드 파일 및 DB 반영**:
   - `cmd/ja/catalog/data/n4_question_seeds.json` 120개 항목 갱신.
   - DB `questions` 테이블의 `options`, `correct_answer`, `explanation` 갱신 (`question_key` 보존으로 `session_questions`, `user_question_progress` 외래키 무결성 100% 유지).

## 검증 결과
1. **DB 선지 전수 검증**:
   - DB `questions` 내 120개 `vocab_usage` 문항 전수 조사: 120/120 (100%) 문항에서 4개 선택지 모두 목표 단어 포함 확인.
   - `session_questions`(7건) 및 `user_question_progress`(4건)의 외래키 및 학습 기록 정상 유지 확인.
2. **테스트 스위트**:
   - `make test`: 전체 PASS
   - `go test -count=1 ./cmd/ja/...`: PASS
3. **런타임 재시작**:
   - `make restart-app`: 재시작 및 `http://localhost:8080/health` 정상 응답 확인.
