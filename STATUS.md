# CopyLingo 현재 상태

> 에이전트는 새 세션 시작 시 이 파일을 읽고 작업을 시작합니다.

---

## 🔨 진행 중

- (없음)

---

## ⏭️ 다음

- 아티클 요약 및 AI 대화 시나리오 (Phase 2.4, 미착수 — 청해 이후)

---

## 🚧 블로커

- (없음)

---

## TODO

> 각 항목은 `docs/todos/<file>.md`에 자기완결적 문서로 분리되어 있다. 작성/실행/완료 처리 규칙은 `AGENTS.md` §3 Case C 참조.

- [ ] 손글씨 client/server rebuild 정합성 검증 — 검증 **도구**(`cmd/dev/handwriting_renderer` CLI + Mini App `?debug=1` export + 단위 테스트) 구현 완료. **사용자 수동 시각 비교만 남음** (Mini App에서 직접 그려 client.png/strokes.json export → 서버 PNG와 대조). see [docs/todos/handwriting_rebuild_parity_verification.md](docs/todos/handwriting_rebuild_parity_verification.md)

- [ ] 사용자 선택형 세션 문제 조합 preset — Daily Session 생성 전에 Vocabulary/Kana/Handwriting 비율 preset을 선택할 수 있도록 설계 및 구현. **(Case A 선결: preset 비율/변경 UX/SRS 충돌 우선순위/vocab fallback 미결)** see [docs/todos/user_selectable_session_mix_presets.md](docs/todos/user_selectable_session_mix_presets.md)

- [ ] Cloudflare Tunnel(cloudflared/trycloudflare) Korea-block 노출 대응 — 손글씨 Mini App ingress가 Cloudflare 의존이라 한국 재차단 시 통째 중단 위험(현시점 도달은 정상). **(Case A 선결: 자체 도메인+named tunnel vs 비-CF ingress vs accept+monitor 미결)** see [docs/todos/cloudflare_korea_tunnel_risk.md](docs/todos/cloudflare_korea_tunnel_risk.md)

## 📝 최근 완료

| 날짜 | 작업 | workthrough |
|------|------|-------------|
| 2026-08-25 | 미완료 Quiz/Study 재알림 재진입을 Redis-first resume로 수정 + stale callback self-heal (ADR-042 보정) | `2608/2608252215_session_resume_fix.md` |
| 2026-08-24 | Linux host CPU·load·memory·swap·disk·상위 process resource monitor script 추가 | `2608/2608241826_host_resource_monitor.md` |
| 2026-08-24 | 손글씨 팔레트 85% 축소 + 정오표시 emoji·정답 노출 + 촉음 별도 칸 제거 | `2608/2608241746_handwriting_palette_ui.md` |
| 2026-08-24 | `/llm`·Quiz·Study LLM one-shot mode 활성화 안내에 취소 버튼 추가 + 사용자별 pending 상태 정리 | `2608/2608241420_llm_mode_cancel.md` |
| 2026-08-24 | Go 1.27.0 toolchain·Docker builder pin 정렬 + stale `/app/data` COPY 제거 (ADR-044) | `2608/2608241408_go_127_upgrade.md` |
| 2026-08-24 | Telegram tap-to-build Word Order 30문항: stable shuffle·Redis draft·undo/reset/submit·exact grading (ADR-043) | `2608/2608241307_word_order_telegram_mvp.md` |
| 2026-08-22 | Scheduled Quiz/Study가 새 세션 대신 미완료 backlog를 우선 재알림 (ADR-042) | `2608/2608222156_scheduled_session_backlog_reminder.md` |
| 2026-08-05 | Morning 17·Evening 12문항 확장 + Daily Session 신규 Listening 1자리 예약 (ADR-041) | `2608/2608050024_daily_session_listening_reservation.md` |
| 2026-08-04 | Reading Study 편성: unseen이 남으면 due 1+new 1, 모두 학습했으면 due 2 (ADR-040) | `2608/2608042344_reading_study_review_new_mix.md` |
| 2026-08-04 | 손글씨 채점 acceptance-first prompt 보강: 불확실한 오답은 feedback을 비우고 정답만 표시 | `2608/2608042248_handwriting_grading_acceptance_prompt.md` |
| 2026-08-04 | Main LLM을 Gemini 3.5 Flash-Lite로 업그레이드하고 Gemini 3.x deprecated sampling parameter 제거 (TTS 유지, 재시작 보류) | `2608/2608042240_gemini_35_flash_lite_upgrade.md` |
| 2026-07-28 | Study Session card별 owner-only AI 질문: 10분 one-shot context, session owner·Material membership 재검증 (ADR-037) | `2607/2607282230_study_ai_question.md` |
| 2026-07-24 | Listening audio path 10→50 복구: seed 재실행 시 동일 script의 runtime `audio_path` 보존 + session push 없는 일괄 TTS 복구 CLI 추가 | `2607/2607241702_listening_audio_backfill.md` |
| 2026-07-23 | N5 독해 seed 10→40 확장(0011~0040): Sonnet subagent 3배치 생성 + Opus 전수 검수(reading 전사·정답 유일성·N5 범위) + 4건 수정. integrity 40 통과, seeder 멱등 reading=40/material NULL 0. Telegram 수동 검수 잔여 | `2607/2607231015_n5_reading_study_quiz.md` |
| 2026-07-23 | N5 독해 Study·Quiz MVP: original 지문 10개 Material-first seed + Study 완료 후 Quiz admission + 세션당 1개 cap (ADR-036). Telegram 수동 검수 잔여 | `2607/2607231015_n5_reading_study_quiz.md` |
| 2026-07-20 | Quiz Question 공유 Catalog 유지 + 사용자별 SRS/served/correct progress 분리 및 owner 429건 backfill (ADR-035) | `2607/2607201728_user_question_progress.md` |
| 2026-07-20 | Luna 3-way 위임으로 N5 Listening seed 10→50문항 확장 + DB/TTS 50/50 생성 | `2607/2607201548_listening_seed_50_tts.md` |
| 2026-07-20 | Luna 초안+main 검수로 N5 Listening original seed 5→10문항 확장 | `2607/2607201527_listening_seed_luna_expansion.md` |
| 2026-07-20 | N5 Listening original MCQ 5문항 멱등 seed + Gemini TTS/MinIO/Telegram live smoke (ADR-034) | `2607/2607201520_listening_question_seed_smoke.md` |
| 2026-07-18 | Vocabulary 한자 recall 431문항 멱등 seed + morning/evening/review 세션당 최대 3개 query/admission cap (ADR-033) | `2607/2607182024_vocab_kanji_recall.md` |
| 2026-07-05 | Study Session `← 이전` 버튼: 이미 본 카드 재열람 (prev는 studied 상태 불변, 첫 카드 제외) | `2607/2607051110_study_prev_button.md` |
| 2026-07-03 | Agent 문서 정합성 수정: Case 분류/workthrough 경로/ADR 분리 파일 규칙/§4.4 TTS native 경로/capability 기록처 SSOT/dead ref 6건 | `2607/2607031640_agent_docs_consistency_fix.md` |
| 2026-07-01 | 청해 음성 파이프라인: Gemini native TTS→OGG(ffmpeg)→MinIO/S3 content-addressed 캐싱→Telegram sendVoice(file_id 캐시), scheduler 사전생성·SessionBuilder 편입 (ADR-031/032) | `2607/2607011825_listening_audio_pipeline.md` |
| 2026-06-30 | 학습 팁 AI 생성 파이프라인: scheduler 세션 빌드 후 (lang,level)별 잔고<50 점진 채움 (`GenerateTips`) | `2606/2606302309_tip_scheduler_generation.md` |
| 2026-06-30 | Unit test 커버리지 공백 보강(01_unit_test_plan): model 0→100%, external→79.8%, service→83.2% (운영코드 무수정) | `2606/2606302311_unit_test_coverage_gap_fill.md` |
| 2026-06-30 | LLM 채점 반환값 구조체화: `(bool, string, error)` → `(GradeResult, error)` 경계 정리 | `2606/2606302226_llm_grade_result_return_refactor.md` |
| 2026-06-30 | ScheduleConfig cron 필드 `CronExpr` 커스텀 타입 전환 + Load 단계 fail-fast 검증 | `2606/2606302221_schedule_cronexpr_config_type.md` |
| 2026-06-30 | Gemini CLI executor 래퍼 스크립트(provider retry/Tool Call 탐지 표준화) + 자체 테스트 32케이스 | `2606/2606302227_gemini_cli_invocation_stabilization.md` |
| 2026-06-30 | 손글씨 rebuild 정합성 검증 **도구** 구현(cmd/dev CLI + Mini App debug export + 테스트) — 수동 시각검증 대기 | `2606/2606302221_handwriting_rebuild_parity_tooling.md` |
| 2026-06-30 | Grammar 예문 읽기(furigana) 보강: `example_reading` 전문 かな 필드 80개 + `읽기:` 줄 렌더 (ADR-030) | `2606/2606301402_grammar_example_reading.md` |
| 2026-06-28 | Quiz 결과에 문제 원문 보존 + `🤖 이 문제 질문` 버튼(문제 컨텍스트 LLM 질문, owner-only) | `2606/2606281536_quiz_inquiry_llm_ask.md` |
| 2026-06-27 | 단어 문맥규정(vocab_context) 문항 도입: 예문 보유 15단어 × 3 cloze = 45문항, grammar cloze 미러(정적 N문항) | `2606/2606272150_vocab_context_question_type.md` |
| 2026-06-26 | SRS 첫 복습 간격 1→3일 상향 (study material + quiz question): 매일 재등장 체감 해소 | `2606261451_study_srs_first_interval.md` |
| 2026-06-25 | AGENTS.md 정리: 죽은 ADR 링크 수정, 콜백 규약 dedup, §5 코딩규칙·위임 detail 분리 (210→164줄) | `2606251247_agents_md_cleanup.md` |
| 2026-06-25 | N5 학습 데이터(grammar/vocab/kana) Go 리터럴 → embedded JSON 분리 | `2606250015_grammar_data_json_extraction.md` |
| 2026-06-23 | N5 일본어 문법 seed 도입 → 60개 확장 및 학습 세션 연동 | `2606232110_expand_n5_grammar_study.md` |
| 2026-06-23 | Study Session 기본량 상향 및 `/study [개수]` 지원 | `2606232010_study_session_limit.md` |
| 2026-06-23 | 손글씨 오답 복원 이미지 저장 로깅 추가 | `2606232002_handwriting_wrong_image_logging.md` |
| 2026-06-22 | Mini App → Bot coupling 제거: handwriting message ref parser를 callback package로 이동 | `2606221538_miniapp_bot_coupling_callback_parser.md` |
| 2026-06-21 | Telegram `/llm` owner-only 질문 및 tip candidate 수집 추가 | `2606212134_telegram_llm_tip_candidates.md` |
| 2026-06-21 | `/study` Study Session 즉시 생성 command 추가 | `2606212040_study_command.md` |
| 2026-06-17 | Quiz 후보 Study Material 우선순위 + Level Fallback | `2606170926_quiz_studied_priority_fallback.md` |
| 2026-06-17 | Study Session 메뉴 진입 및 stale session 복구 | `2606170915_study_session_menu_recovery.md` |
| 2026-06-16 | Quiz 후보를 학습 완료 Material로 제한 | `2606162305_quiz_studied_material_filter.md` |
| 2026-06-15 | 오후 Study Session Push 추가 | `2606151436_afternoon_study_push.md` |
| 2026-06-15 | JA Seeder 단일 command 통합 | `2606151409_ja_seeder_consolidation.md` |
| 2026-06-13 | Question Item Type taxonomy 및 기존 question backfill | `2606131243_question_item_type_taxonomy.md` |
| 2026-06-08 | Study Session Redis Working Set 및 완료 시 flush 적용 | `2606081615_study_active_session_redis_working_set.md` |
| 2026-06-08 | 정오 Study Session Vocabulary-only 8개 조정 | `2606080038_study_session_vocabulary_only.md` |
| 2026-06-06 | 정오 Study Material Session Scheduler 구현 | `2606062301_study_session_scheduler.md` |
| 2026-06-03 | Daily Session Vocabulary 최소 1/3 예약 정책 적용 | `2606030140_daily_session_vocabulary_reservation.md` |
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
