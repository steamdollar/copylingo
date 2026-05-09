# CLAUDE.md 검토 및 문서 정합성 작업

**날짜**: 2026-03-22 18:00 KST
**에이전트**: Claude Code (Opus 4.5)

---

## 작업 개요

프로젝트 본격 착수 전 CLAUDE.md 및 관련 문서 검토/보완

## 완료 항목

### 1. CLAUDE.md 구조 개선
- tech stack 불일치 수정 (SQLite → PostgreSQL 16, Go 1.25)
- 세션 구성 정보 추가 (오전 15문제/오후 10문제, 신규 60% + 복습 40%)
- 문제 유형 6종 명시
- 개발 명령어 섹션 추가
- 필수 규칙에 테스트 정책 추가 (`make test` 필수)

### 2. AGENTS.md 업데이트
- AI 모델: Gemini 3.0 Flash로 통일
- Go 버전: 1.25로 통일
- CAUTION 간소화: 4개 → 1개 (API 키 하드코딩 금지만 유지)
- 오후 세션 비율: 20:80 → 60:40으로 수정 (오전과 동일)

### 3. ADR 업데이트
- ADR-004: AI 모델 변경 이력 추가 (GPT-4o-mini → Gemini 3.0 Flash)

### 4. 디렉토리 생성
- `docs/workthrough/` 디렉토리 생성

### 5. 에이전트 협업 방식 확정
- Claude Code: 테크 리드 (아키텍처, 설계)
- Gemini: 단순 구현 담당 (Claude가 프롬프트 작성 → 사용자가 복붙)

## 미결 사항 (Phase 2에서 결정)

| 항목 | 내용 |
|------|------|
| 문제 유형별 생성 로직 | 6종 각각의 생성 방식 |
| 문제 유형별 채점 로직 | translation, listening 등 AI 채점 필요 여부 |
| 듣기 문제 형식 | 출제 방식 및 TTS 파일 저장 |
| fill_blank 오타 허용 | 정확 일치 vs 레벤슈타인 거리 허용 |

## 수정 파일 목록

- `CLAUDE.md`
- `AGENTS.md`
- `docs/ADR.md`
- `docs/workthrough/` (신규 디렉토리)
- `STATUS.md` (신규 - CURRENT_TASK.md 대체)

## 삭제 파일

- `CURRENT_TASK.md` → `STATUS.md`로 대체
- `docs/HISTORY.md` → `docs/workthrough/`로 대체

## 프로토콜 변경

**기존:**
```
읽기: AGENTS.md → ROADMAP.md → CURRENT_TASK.md (3개)
쓰기: CURRENT_TASK.md + ROADMAP.md + HISTORY.md + workthrough (4개)
```

**변경:**
```
읽기: AGENTS.md → STATUS.md (2개)
쓰기: STATUS.md + workthrough (2개, 마일스톤 시 ROADMAP.md)
```
