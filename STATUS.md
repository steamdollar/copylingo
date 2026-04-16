# CopyLingo 현재 상태

> 에이전트는 새 세션 시작 시 이 파일을 읽고 작업을 시작합니다.

---

## 🔨 진행 중

**Phase 2.2: JLPT 학습 자료 수집기**

- 목표: JLPT 기출 문제 수집 → contents 테이블 저장
- 기존 파이프라인 재사용: JLPTFetcher 구현 후 Orchestrator에 등록
- source_type: `exam_prep`

---

## ⏭️ 다음

- Phase 2.3: AI 문제 생성 엔진 (Gemini 연동)

---

## 🚧 블로커

- (없음)

---

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
| 2026-04-16 | Phase 2.3 일부: AI 주관식 유사도 채점 기능 및 UX 인디케이터 추가 | `2604161909_phase_2_3_ai_subjective.md` |
| 2026-03-31 | 봇 세션 플로우 개선 및 결과 요약 에러 수정 | `2603310123_bot_fixes_and_dx_optimization.md` |
| 2026-03-31 | 'air' 핫 리로드 및 Tmux 통합 대시보드 구축 | `2603310123_bot_fixes_and_dx_optimization.md` |
| 2026-03-31 | config.go OPENAI_API_KEY 검증 완화 (선택적 사용) | `2603310041_remove_openai_key_validation.md` |
| 2026-03-31 | Phase 2.1.5: 히라가나/가타카나 학습 구현 | `2603310027_phase_2_1_5_kana_module.md` |
| 2026-03-22 | Phase 2.1: 3단계 파이프라인 + NHK 수집기 구현 | `2603222000_pipeline_nhk_collector.md` |
| 2026-03-22 | cmd/server/main.go Run 패턴 적용 및 구조 정리 | `2603221930_refactor_main_go.md` |
| 2026-03-22 | 다국어 지원 스키마 리팩토링 (ADR-009) | `2603222100_multilang_schema.md` |
| 2026-03-22 | CLAUDE.md 검토 및 문서 정합성 | `2603221800_claude_md_review.md` |
| 2026-03-11 | Phase 1 전체 (프로젝트 뼈대 32개 파일) | - |
