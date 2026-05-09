# CopyLingo 현재 상태

> 에이전트는 새 세션 시작 시 이 파일을 읽고 작업을 시작합니다.

---

## 🔨 진행 중

**Phase 2.2: JLPT 학습 자료 수집기 (진행 중)**

- 목표: JLPT 기출 문제 수집 → contents 테이블 저장
- 최근 작업: SessionFlow 메시지 렌더링의 `messageID=0` sentinel 제거
- 남은 작업: JLPTFetcher 구현 완료 및 Orchestrator 등록
- source_type: `exam_prep`

---

## ⏭️ 다음

- Phase 2.4: 아티클 요약 및 AI 대화 시나리오 구현

---

## 🚧 블로커

- (없음)

---

## TODO

> 각 항목은 `docs/todos/<file>.md`에 자기완결적 문서로 분리되어 있다. 작성/실행/완료 처리 규칙은 `AGENTS.md`의 "TODO 문서 프로토콜" 참조.

- [ ] `showQuestion` 반복 DB hit 개선 — 문제 이동마다 2회 read 발생, JOIN 또는 cache로 축소. see [docs/todos/show_question_db_hit_reduction.md](docs/todos/show_question_db_hit_reduction.md)
- [ ] Service 레이어 인터페이스 도입 + 단위 테스트 — repo concrete 의존을 unexported 인터페이스로 교체하고 SRS/Grader/SessionBuilder/Analyzer 테스트 작성. see [docs/todos/service_layer_interfaces_and_tests.md](docs/todos/service_layer_interfaces_and_tests.md)

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
| 2026-05-09 | 손글씨 Mini App tunnel 안정화 및 stale URL 복구 | `2605091337_tmux_tunnel_dashboard.md` |
| 2026-05-08 | showQuestion silent error 처리 (로그 + 사용자 안내) | `2605081617_show_question_silent_error_handling.md` |
| 2026-05-08 | showQuestion TODO 이슈 분리 | `2605081544_status_showquestion_todo_split.md` |
| 2026-05-08 | showQuestion 안정화 TODO 구체화 | `2605081538_status_showquestion_todo.md` |
| 2026-05-08 | SessionFlow editMessageID 명시화 및 ADR 기록 | `2605081521_session_flow_edit_message_id.md` |
| 2026-05-08 | 에러 처리/로깅 리팩터링 | `2605081351_error_handling_refactoring.md` |
| 2026-05-08 | 손글씨 Mini App 테스트 안정화 | `260508_handwriting_miniapp_test_stabilization.md` |
| 2026-05-07 | Cloudflare Quick Tunnel URL 자동 반영 스크립트 추가 | `2605072247_quick_tunnel_env_script.md` |
| 2026-05-07 | 손글씨 Mini App ingress/Cloudflare Tunnel ADR 및 운영 문서화 | `2605072128_handwriting_miniapp_ingress_docs.md` |
| 2026-04-25 | kana seeder Type 2 문항 script label 추가 | `2604250015_kana_seeder_type2_script_label.md` |
| 2026-04-24 | README에 Mini App + Cloudflare Tunnel 설정 절차 추가 | `2604241732_readme_miniapp_tunnel_setup.md` |
| 2026-04-24 | kana seeder batch insert + transaction 적용 | `2604241625_kana_seeder_batch_insert.md` |
| 2026-04-23 | 손글씨 가나 Mini App MVP 구현 | `2604231736_handwriting_miniapp_mvp.md` |
| 2026-04-23 | 손글씨 가나 문항 구현 방향 ADR 기록 (Mini App + Binary Grading) | `2604231712_handwriting_miniapp_adr.md` |
| 2026-04-16 | 서비스 계층 개별 의존성 주입(Individual DI) 적용 및 UserService 분리 | `2604162325_service_di_refactoring.md` |
| 2026-04-16 | Phase 2.3: AI 주관식 유사도 채점 기능 및 UX 인디케이터 추가 | `2604161909_phase_2_3_ai_subjective.md` |
| 2026-03-31 | 봇 세션 플로우 개선 및 결과 요약 에러 수정 | `2603310123_bot_fixes_and_dx_optimization.md` |
| 2026-03-31 | 'air' 핫 리로드 및 Tmux 통합 대시보드 구축 | `2603310123_bot_fixes_and_dx_optimization.md` |
| 2026-03-31 | config.go OPENAI_API_KEY 검증 완화 (선택적 사용) | `2603310041_remove_openai_key_validation.md` |
| 2026-03-31 | Phase 2.1.5: 히라가나/가타카나 학습 구현 | `2603310027_phase_2_1_5_kana_module.md` |
| 2026-03-22 | Phase 2.1: 3단계 파이프라인 + NHK 수집기 구현 | `2603222000_pipeline_nhk_collector.md` |
| 2026-03-22 | cmd/server/main.go Run 패턴 적용 및 구조 정리 | `2603221930_refactor_main_go.md` |
| 2026-03-22 | 다국어 지원 스키마 리팩토링 (ADR-009) | `2603222100_multilang_schema.md` |
| 2026-03-22 | CLAUDE.md 검토 및 문서 정합성 | `2603221800_claude_md_review.md` |
| 2026-03-11 | Phase 1 전체 (프로젝트 뼈대 32개 파일) | - |
