# 다국어 지원 스키마 리팩토링

- **날짜**: 2026-03-22
- **작업자**: Claude Opus 4.5
- **관련 ADR**: ADR-009

## 배경

초기 설계는 일본어(JLPT) 전용으로 `jlpt_level` 필드를 하드코딩했으나, 그리스어 등 추가 언어 학습 가능성 대비 확장성이 필요했다.

## 변경 사항

### DB 스키마 (migrations/001_init.up.sql)

```sql
-- 변경 전
current_level VARCHAR(2) DEFAULT 'N5'    -- users
jlpt_level VARCHAR(2) DEFAULT 'N5'       -- contents, questions

-- 변경 후
language VARCHAR(10) DEFAULT 'ja'         -- ISO 639-1
proficiency_level VARCHAR(10) DEFAULT 'N5' -- JLPT: N5-N1, CEFR: A1-C2
```

추가 변경:
- `contents.source_url`에 UNIQUE 제약조건 추가 (중복 수집 방지)
- `questions` 테이블에 `(language, proficiency_level)` 복합 인덱스 추가

### 수정된 파일 목록 (12개)

| 파일 | 변경 내용 |
|------|----------|
| `migrations/001_init.up.sql` | 전면 재작성 |
| `schema.dbml` | 다국어 필드 반영 |
| `internal/model/user.go` | `CurrentLevel` → `Language` + `ProficiencyLevel` |
| `internal/model/content.go` | `JLPTLevel` → `Language` + `ProficiencyLevel` |
| `internal/model/question.go` | `JLPTLevel` → `Language` + `ProficiencyLevel` |
| `internal/model/stats.go` | `WeakArea.JLPTLevel` → `ProficiencyLevel` |
| `internal/repository/user_repo.go` | INSERT/UPDATE 쿼리 수정 |
| `internal/repository/content_repo.go` | INSERT 쿼리 + `GetArticles` 시그니처 변경 |
| `internal/repository/question_repo.go` | INSERT 쿼리 + `GetNewQuestions` 시그니처 변경 |
| `internal/service/session_builder.go` | `BuildMorningSession`, `BuildEveningSession` 파라미터 확장 |
| `internal/scheduler/scheduler.go` | `user.Language`, `user.ProficiencyLevel` 사용 |
| `internal/bot/handler.go` | 메뉴에 언어 표시 + `languageDisplayName` 헬퍼 추가 |

## 테스트

```bash
$ go build ./...   # 성공
$ make test        # 테스트 파일 없음 (no test files)
```

## 다중 언어 학습 전략

현재는 사용자당 1개 언어만 지원. 여러 언어 학습 필요 시:
- 단기: 별도 user 레코드 생성 (Telegram ID 동일해도 가능하도록 추후 PK 변경)
- 장기: `(telegram_id, language)` 복합키로 마이그레이션

## 후속 작업

- Phase 2.1 NHK 크롤러 구현 시 `language: "ja"` 하드코딩
- 그리스어 소스 추가 시 ContentSource 인터페이스 도입 검토
