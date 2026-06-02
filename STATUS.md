# CopyLingo 현재 상태

> 에이전트는 새 세션 시작 시 이 파일을 읽고 작업을 시작합니다.

---

## 🔨 진행 중

- Phase 2.4: 아티클 요약 및 AI 대화 시나리오 구현

---

## ⏭️ 다음

- (없음)

---

## 🚧 블로커

- (없음)

---

## TODO

> 각 항목은 `docs/todos/<file>.md`에 자기완결적 문서로 분리되어 있다. 작성/실행/완료 처리 규칙은 `AGENTS.md`의 "TODO 문서 프로토콜" 참조.

- [ ] 학습 팁 AI 생성 (scheduler 통합) — (lang, level) 잔고 < 50 일 때 세션 빌드마다 2-3개 LLM 으로 생성. see [docs/todos/tip_scheduler_generation.md](docs/todos/tip_scheduler_generation.md)

- [ ] LLM 채점 반환값 구조체화 — `bool, string, error` tuple 대신 `(GradeResult, error)`로 의미 명확화. see [docs/todos/llm_grade_result_return_refactor.md](docs/todos/llm_grade_result_return_refactor.md)

- [ ] 손글씨 client/server rebuild 정합성 검증 — 동일 stroke JSON 기준 Mini App canvas와 서버 PNG 비교. see [docs/todos/handwriting_rebuild_parity_verification.md](docs/todos/handwriting_rebuild_parity_verification.md)

- [ ] Future Gemini CLI Invocation Stabilization — Gemini CLI wrapper로 provider retry와 Tool Call 오류 탐지를 표준화. see [docs/todos/future_gemini_cli_invocation_stabilization.md](docs/todos/future_gemini_cli_invocation_stabilization.md)

- 손글씨 쓰기 채점 - 너무 오래 걸림. 대안 필요. (속도를 줄이던가, 그 사이에 뭘 하게끔 하던가)

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
| 2026-06-02 | Native Spawn 및 Gemini CLI External Delegation Protocol 문서화 | `2606022109_agent_delegation_protocol.md` |
| 2026-06-02 | Study용 N5 Vocabulary Material Catalog 500개 확장 | `2606022041_expand_n5_vocab_material_catalog.md` |
| 2026-06-02 | Study Module Task 1: Material SSOT 및 Vocabulary Seed 추가 | `2606021642_material_ssot_seed.md` |
| 2026-06-02 | 가타카나 `ユ`·`ヲ` 손글씨 문항 제외 | `2606020056_katakana_handwriting_exclusion.md` |
| 2026-06-01 | 일별 JSONL Structured Logging 도입 | `2606011418_structured_logging.md` |
| 2026-05-31 | 손글씨 채점 정확도 튜닝 (Detail + Prompt + Renderer) | `2605312040_handwriting_image_detail_test_sync.md` |
| 2026-05-30 | Telegram Mini App tuning | `2605302255_telegram_mini_app_tuning.md` |
| 2026-05-30 | kana ji/zu 역방향 문항 행 힌트 추가 및 로컬 DB 보정 | `2605302242_kana_ji_zu_disambiguation.md` |
| 2026-05-30 | 손글씨 채점 Conditional Verification prompt 보강 | `2605301343_handwriting_conditional_verification.md` |
| 2026-05-30 | 동일 세션 중복 문항 출제 및 already-answered 오판 수정 | `2605300945_session_question_dedup.md` |
| 2026-05-28 | Redis Active Session State 구현 | `2605281946_redis_active_session_state.md` |
| 2026-05-28 | 손글씨 LLM 채점 튜닝 (generation bound + prompt rubric) | `2605281551_handwriting_llm_generation_bounds.md` |
| 2026-05-28 | Mini App HandlerDeps 생성자 정리 | `2605281528_miniapp_handler_deps.md` |
| 2026-05-28 | 손글씨 LLM 오류 사용자 노출 차단 | `2605281535_handwriting_error_sanitization.md` |
| 2026-05-28 | 손글씨 채점 Feedback 정책 정리 | `2605281450_handwriting_feedback_policy.md` |
| 2026-05-28 | 손글씨 채점 응답 포맷 Strict JSON Schema 적용 | `2605281417_handwriting_json_schema.md` |
| 2026-05-27 | 서버 재시작 후 Mini App public URL stale 복구 안정화 | `2605271519_public_url_recovery.md` |
| 2026-05-27 | `/exit` 명령어 구현 및 `/help` 텍스트 정비 | `2605271445_help_exit_commands.md` |
| 2026-05-27 | SessionBuilder 카테고리 랜덤 배분 로직 구현 (Random Slot Relay) | `2605271400_session_category_random_relay.md` |
| 2026-05-27 | Kana 이후 N5 단어 vocabulary seed 구현 | `2605271247_n5_vocab_seed.md` |
| 2026-05-20 | 손글씨 Mini App 학습 팁 통합 | `2605200103_handwriting_tips_integration.md` |
| 2026-05-11 | 에이전트 가이드라인 문서 재구성 (AGENTS SSOT + CLAUDE/GEMINI thin overlay, ADR-014 Open 분리) | `2605110132_agent_docs_restructure.md` |
| 2026-05-09 | Service 레이어 error path 단위 테스트 보강 | `2605091506_service_error_path_tests.md` |
| 2026-05-09 | Service 레이어 인터페이스 도입 및 단위 테스트 작성 (Phase 2.5) | `2605091440_service_layer_refactoring.md` |
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
