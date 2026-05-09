# Phase 2.1: 3단계 파이프라인 + NHK 수집기 구현

**날짜**: 2026-03-22
**작업자**: Claude Code

---

## 목표

다양한 학습 자료 소스(NHK, JLPT 등)를 수집할 수 있는 확장 가능한 3단계 파이프라인 구조 구현

---

## 아키텍처

```
┌─────────────────────────────────────────────────────────────┐
│                   Pipeline Orchestrator                      │
│                                                              │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐              │
│  │ Fetcher  │ →  │ Processor│ →  │  Saver   │              │
│  │ (수집)   │    │ (AI가공) │    │ (저장)   │              │
│  └──────────┘    └──────────┘    └──────────┘              │
│       │               │               │                     │
│  NHKFetcher      PassThrough     ContentSaver              │
│  JLPTFetcher*    AIProcessor*    QuestionSaver*            │
│  ...             (Phase 2.3)     (Phase 2.3)               │
└─────────────────────────────────────────────────────────────┘
```

---

## 구현 파일

### 신규 파일 (7개)

| 파일 | 역할 |
|------|------|
| `internal/pipeline/interfaces.go` | Fetcher, Processor, Saver 인터페이스 정의 |
| `internal/pipeline/fetcher_nhk.go` | NHK News Easy Fetcher 구현 |
| `internal/pipeline/processor.go` | PassThrough Processor (Phase 2.1) |
| `internal/pipeline/saver.go` | ContentSaver 구현 |
| `internal/pipeline/orchestrator.go` | 파이프라인 오케스트레이터 |
| `internal/pipeline/pipeline_test.go` | 파이프라인 테스트 |
| `internal/external/nhk_client.go` | NHK API HTTP 클라이언트 |
| `internal/external/nhk_client_test.go` | NHK 클라이언트 테스트 |

### 수정 파일 (3개)

| 파일 | 변경 내용 |
|------|----------|
| `internal/scheduler/scheduler.go` | Orchestrator 연동, 콘텐츠 수집 크론 작업 추가 |
| `cmd/server/server.go` | initPipeline 함수 추가 |
| `cmd/server/main.go` | 파이프라인 초기화 호출 추가 |

---

## 핵심 인터페이스

```go
// Fetcher: 외부 소스에서 raw 데이터 수집
type Fetcher interface {
    Name() string
    Fetch(ctx context.Context) ([]RawContent, error)
}

// Processor: raw 데이터 가공 (AI 처리 등)
type Processor interface {
    Process(ctx context.Context, raw []RawContent) ([]model.Content, error)
}

// Saver: 가공된 데이터 저장
type Saver interface {
    Save(ctx context.Context, contents []model.Content) (SaveResult, error)
}
```

---

## NHK News Easy API

- 목록: `https://www3.nhk.or.jp/news/easy/news-list.json`
- 본문: `https://www3.nhk.or.jp/news/easy/{newsID}/{newsID}.html`
- HTML에서 `<div id="js-article-body">` 내용 추출
- Ruby 태그 제거 (후리가나)

---

## 설계 결정

| 항목 | 결정 | 근거 |
|------|------|------|
| 파이프라인 | 3단계 분리 | 확장성, 각 단계 독립 테스트 |
| 저장 책임 | Saver 계층 | 단일 책임, DRY |
| 실행 방식 | 순차 | 개인 사용, Rate Limit 안전 |
| 인터페이스 | 처음부터 추출 | 테스트 필수 원칙 |
| HTML 파싱 | 표준 라이브러리 | 서드파티 최소화 |

---

## 테스트

```bash
make test
# internal/external: 3 tests
# internal/pipeline: 8 tests
```

---

## 확장 방법

### 새 소스 추가 (예: JLPT)

1. `JLPTFetcher` 구현 (Fetcher 인터페이스)
2. `server.go`의 `initPipeline`에서 등록:

```go
jlptFetcher := pipeline.NewJLPTFetcher(...)
orchestrator.Register(jlptFetcher, processor, saver)
```

### AI 가공 추가 (Phase 2.3)

1. `AIProcessor` 구현 (Processor 인터페이스)
2. `QuestionSaver` 구현 (Saver 인터페이스)
3. 새 파이프라인 등록

---

## 향후 작업

- Phase 2.2: JLPTFetcher 구현
- Phase 2.3: AIProcessor + QuestionSaver (Gemini 연동)
