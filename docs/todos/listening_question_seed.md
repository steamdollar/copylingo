# TODO: 청해(Listening) 문항 시드 작성

> Case C 분리 문서. 청해 **음성 파이프라인**(TTS 생성·저장·전송·렌더·채점)은 이미 구현·검증 완료
> (see [docs/adr/ADR-031_032_listening_audio_pipeline.md](../adr/ADR-031_032_listening_audio_pipeline.md),
> workthrough `2607/2607011825_listening_audio_pipeline.md`).
> 남은 것은 **문항 콘텐츠 자체**(스크립트·질문·보기·정답)를 시드하는 일이다.

## 배경 / 목적

- 파이프라인은 "`audio_script`가 있고 `audio_path`가 없는 청해 문항"을 찾아 음성을 채우는 구조다.
  즉 **문항이 DB에 있어야** 음성 생성·세션 편입·렌더가 동작한다. 현재 청해 문항 시드는 **0건**이라
  파이프라인이 돌 대상이 없다.
- 기존 grammar/vocab/kana처럼 **정적 JSON seed + Go 생성 로직** 패턴으로 N5 comprehension MCQ를 작성한다.

## ⚠️ 계획 재검토: quiz-only vs material-first (2026-07-01 논의 중)

> 이 TODO는 처음에 "청해 quiz 문항을 먼저 시드해 완성된 음성 파이프라인을 스모크한다"는 좁은 범위로
> 작성됐다. 하지만 사용자가 **study 세션에서도 듣기 학습을 하고 싶다**고 명시했으므로, 콘텐츠 SSOT를
> `questions`가 아니라 `materials`에 둘지 재검토가 필요하다.

- **현재 코드 사실**:
  - `questions.material_id`는 nullable이라, 기존 계획대로면 **material 없는 listening question**으로도 quiz 출제는 가능하다.
  - quiz 세션의 `defaultCategoryOrder`에는 이미 `listening`이 포함되어 있고, `GetNewQuestions`는
    `category='listening'` 문항을 **`audio_path IS NOT NULL`일 때만** 출제한다.
  - 반면 `/study`는 `materials` 기반이고, 현재 `MaterialRepository.GetForStudySession`은
    `vocabulary`, `grammar` material만 조회한다. listening material category/renderer/audio 재생은 아직 없다.
- **대안 1: quiz-first**:
  - 장점: 기존 파이프라인 스모크가 가장 작고 빠르다.
  - 단점: material 없는 question이 되어 study 학습 흐름과 연결되지 않는다.
- **대안 2: material-first** *(사용자 제안, 장기적으로 더 자연스러운 후보)*:
  - listening script/audio를 study material로 먼저 만들고, study session에서 음성을 재생한다.
  - comprehension MCQ question은 해당 listening material의 `material_id`를 참조한다.
  - 장점: "듣기 학습 → 듣기 문제" 흐름이 생기고, study 이력이 quiz 우선순위에도 반영된다.
  - 단점: 현재 TODO보다 범위가 커진다. `MaterialCategoryListening`, listening material payload,
    study material 조회/렌더/audio 전송, material-question 연결 seed가 필요하다.
- **결정 필요**: 이번 작업을 기존처럼 **quiz-first 스모크**로 유지할지, 아니면 Case A로 승격해
  **material-first listening study 설계**를 먼저 확정한 뒤 seed를 작성할지 정해야 한다.

### 다음 단계 (2026-07-02)

- 우선 **Case A 설계 결정**으로 전환한다. 바로 seed 구현하지 않는다.
- 결정할 것:
  1. 청해 콘텐츠 SSOT를 `questions`가 아니라 `materials`로 둘지.
  2. 음성 메타데이터(`audio_script`/`audio_path`/`audio_file_id`)를 `materials`에도 둘지, 아니면 별도
     `audio_assets` 같은 공유 테이블로 분리할지.
  3. `/study`에서 listening material을 어떻게 보여줄지: 음성 먼저 재생 → 스크립트/해석/핵심표현 공개 →
     다음 material 이동.
  4. quiz listening question이 listening material을 `material_id`로 참조하고, study 완료 이력을 quiz 우선순위에
     활용할지.
- Case A 결정이 끝나면 ADR에 기록하고, 이 TODO는 material-first seed/구현 계획으로 재작성한다.

## ⚠️ 선결: 문항 소스 확보 (이 TODO의 최대 미결점)

> 파이프라인/시드 로직보다 **콘텐츠 소스를 어디서 얻을지가 먼저**다. 실행 전 아래에서 방향을 정할 것.

- **후보 소스**:
  - **(A) 자체 작성 / LLM 생성** *(권장)* — N5 수준 짧은 대화·독백을 직접 또는 LLM으로 생성. 저작권 clean, 난이도·주제 통제 가능, 포트폴리오상 안전. 단 품질 검수 필요(자연스러운 일본어·정답 유일성).
  - **(B) 공개 라이선스 코퍼스** — Tatoeba(CC-BY) 등 문장 코퍼스에서 대화 조합. 라이선스 표기 필요, 청해용 대화 형태로 가공 부담.
  - **(C) JLPT 기출/시판 교재** — **기각 권장**: 저작권 리스크(복제 배포). 개인 학습 참고만.
- **결정 필요**: 위 중 택1 (혹은 A+B 혼합). **이 선택이 정해지기 전엔 실행 착수 금지** — 정해지면 이 문서에 pin.
- **결정됨 (2026-07-01)**: **(A) 자체 작성 / LLM 생성**으로 진행한다. 단, 기출/교재 문항 복제 없이
  **JLPT-style original**로 작성한다. 공개 sample이나 시험 유형은 난이도·문항 형식 참고까지만 허용하고,
  스크립트·질문·보기·정답·해설은 전부 신규 작성한다.
- **분량 기준**: 우선 N5 8~12문항으로 파이프라인 실동작 확인 → 이후 점진 확장.
- **초기 작성 수량 (2026-07-01 결정)**: **10문항**. `TopUpAudio`가 cycle당 5개를 채우므로 2회 cycle로
  전체 seed가 음성 생성 대상이 된다.
- **초기 난이도 배분**:
  - 쉬움 3개: 가격/시간/장소처럼 단일 정보 추출.
  - 보통 5개: 짧은 대화에서 행동/선택/이유 파악.
  - 약간 어려움 2개: 날짜·순서·부정 표현 포함.
- **품질 게이트**: 스크립트-질문-정답 정합성(정답이 스크립트에서 유일하게 도출되는지), 오답 보기의 그럴듯함, かな/한자 난이도 N5 적합성.

## 이미 정해진 것 (재논의 불필요)

- **문항 형태**: comprehension MCQ 1차 (dictation/받아쓰기 후순위) — ADR-031.
- **채점**: 신규 grader 없이 기존 MCQ exact-match 경로 재사용 ([grader.go](../../internal/service/grader.go) `Type != Subjective` → `userAnswer == CorrectAnswer`). 따라서 문항은 `options` + `correct_answer`만 채우면 됨.
- **필드 매핑** (문항 1건당):
  - `type = "listening"`, `category = "listening"`, `language = "ja"`, `proficiency_level = "N5"`
  - `audio_script` = 음성으로 읽을 JA 스크립트(대화/독백). **화면에 노출 안 됨** — 이게 sha256되어 object key가 됨.
  - `prompt` = 화면에 보이는 comprehension 질문 (예: `音声を聞いて答えてください。男の人は何を買いますか?`)
  - `options` = JSONB 문자열 배열(4지선다 권장), `correct_answer` = 정답 옵션 문자열(옵션 중 하나와 정확히 일치)
  - `explanation` = 해설(정답 근거)
  - `question_key` = 안정적 seed 키 (예: `ja:listening:n5:0001`) — upsert 멱등성용
  - `item_type`(Skill)은 `listening_task`/`listening_key_point` 등 ([question.go](../../internal/model/question.go) 청해 Skill 5종) 중 매핑

## 변경할 파일 + before/after

1. **`cmd/ja/catalog/data/n5_listening.json`** (신규) — 문항 배열. 초안 스키마:
   ```json
   [
     {
       "id": "n5_listening_0001",
       "skill": "listening_task",
       "script": "すみません、この本はいくらですか。 — 800円です。",
       "prompt": "音声を聞いて答えてください。本はいくらですか?",
       "options": ["800円", "700円", "500円", "1000円"],
       "correct_answer": "800円",
       "explanation": "店員が「800円です」と答えている。"
     }
   ]
   ```
   - 초기 분량: **N5 8~12문항** 권장 (파이프라인 cycle당 5개 생성이라 2~3 스케줄 사이클이면 소진).

2. **`cmd/ja/catalog/datasets.go`** — `//go:embed data/n5_listening.json` + `ListeningQuestion` 구조체 + 파서 추가
   (기존 `VocabWord`/`GrammarPoint` 패턴 미러).

3. **`cmd/ja/catalog/materials.go`** 또는 seeder — 청해 문항 build 함수 추가:
   `[]ListeningQuestion` → `[]*model.Question`(위 필드 매핑, `AudioScript` 세팅, `AudioPath`/`AudioFileID`는 nil로 둠)
   → `repos.Question.UpsertSeedBatch`. (batch upsert는 이미 `audio_script` 포함하도록 구현됨.)

4. **`cmd/ja/seeder/main.go`** — 청해 seed 호출 등록 (기존 grammar/vocab/kana seed 호출부 옆).

## 검증 방법

1. `make test` — seeder/catalog 단위 테스트(파서·필드 매핑) 추가 후 통과.
2. **로컬 e2e 스모크** (파이프라인 실동작 확인):
   - 선행: **ffmpeg 설치** (`sudo apt install -y ffmpeg`) — transcode 필수, 현재 미설치.
   - `make infra` + MinIO 기동(`docker compose up -d minio minio-createbucket`) + `make migrate`
   - seed 실행 → 청해 문항 INSERT 확인
   - 앱 기동 후 스케줄러 `TopUpAudio`가 돌거나, 수동으로 한 사이클 트리거 → MinIO에 `tts/ja/kore/<sha>.ogg` 생성 + `audio_path` 세팅 확인
   - `/test` 세션에 청해 문항 편입 → Telegram에서 voice 수신 + 보기 선택 채점 확인 → `audio_file_id` 캐시 확인

## off-limits / 주의

- **파이프라인 코드는 건드리지 말 것** — 이미 완성. 이 TODO는 **콘텐츠(JSON) + seed 로직**만.
- Cloudflare tunnel 대응은 별도 TODO(`cloudflare_korea_tunnel_risk.md`), 무관.
- 실 API 호출은 Gemini 무료 quota + preview TTS rate-limit 안에서. 대량 생성 아님(문항 소량).
