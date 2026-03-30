# CopyLingo 로드맵

> 전체 프로젝트 진행 상황. 에이전트는 새 대화 시작 시 이 파일을 읽고 현재 상태를 파악합니다.
> 마지막 업데이트: 2026-03-11 15:15 KST

---

## Phase 1: 프로젝트 기반 구축 ✅

**완료일**: 2026-03-11

| Subphase | 상태 | 산출물 |
|---|---|---|
| 1.1 Go 모듈 & 디렉토리 | ✅ 완료 | `go.mod`, 프로젝트 구조 |
| 1.2 설정 관리 | ✅ 완료 | `internal/config/config.go`, `config.yaml` |
| 1.3 도메인 모델 | ✅ 완료 | `internal/model/` (6개 파일) |
| 1.4 DB 스키마 | ✅ 완료 | `migrations/001_init.up.sql` (7개 테이블) |
| 1.5 레포지토리 레이어 | ✅ 완료 | `internal/repository/` (7개 파일) |
| 1.6 서비스 레이어 | ✅ 완료 | `internal/service/` (SRS, SessionBuilder, Grader, Analyzer) |
| 1.7 텔레그램 봇 핸들러 | ✅ 완료 | `internal/bot/` (handler, session_flow, menu) |
| 1.8 스케줄러 | ✅ 완료 | `internal/scheduler/scheduler.go` |
| 1.9 인프라 | ✅ 완료 | `docker-compose.yml`, `Dockerfile`, `Makefile` |
| 1.10 빌드 검증 | ✅ 완료 | `go build` 통과 |

---

## Phase 2: 콘텐츠 파이프라인 ⬜

**상태**: 미착수

| Subphase | 상태 | 설명 |
|---|---|---|
| 2.1 NHK News Easy 크롤러 | ✅ 완료 | `internal/pipeline/collector.go`, `internal/external/nhk.go` |
| 2.1.5 히라가나/카타카나 학습 | ✅ 완료 | `cmd/kana_seeder`, 주관식 봇 핸들링 추가 |
| 2.2 JLPT 학습 자료 수집기 | ⬜ 대기 | `internal/external/` 추가 소스 |
| 2.3 AI 문제 생성 엔진 | ⬜ 대기 | `internal/pipeline/generator.go`, `internal/external/openai.go` |
| 2.4 TTS 음성 생성 & 캐싱 | ⬜ 대기 | `internal/pipeline/tts.go`, `internal/external/tts_client.go` |
| 2.5 파이프라인 통합 테스트 | ⬜ 대기 | 수집 → 문제 생성 → TTS 전체 흐름 검증 |

---

## Phase 3: 학습 엔진 고도화 ⬜

**상태**: 미착수 (Phase 2 완료 후)

| Subphase | 상태 | 설명 |
|---|---|---|
| 3.1 커리큘럼 JSON 정의 | ⬜ 대기 | `data/curriculum/n5.json` ~ `n1.json` |
| 3.2 커리큘럼 매니저 | ⬜ 대기 | `internal/service/curriculum.go` |
| 3.3 아티클 읽기 플로우 | ⬜ 대기 | `internal/bot/article_flow.go` |
| 3.4 AI 대화 모드 (독후감) | ⬜ 대기 | `internal/bot/chat_flow.go` |
| 3.5 레벨 자동 승급 로직 | ⬜ 대기 | N5→N4→...→N1 진행 기준 |

---

## Phase 4: 분석 & 피드백 ⬜

**상태**: 미착수 (Phase 3 완료 후)

| Subphase | 상태 | 설명 |
|---|---|---|
| 4.1 일일 리포트 | ⬜ 대기 | 매일 학습 요약 텔레그램 전송 |
| 4.2 주간 리포트 | ⬜ 대기 | 주간 통계 + 취약 분야 분석 |
| 4.3 난이도 자동 조정 | ⬜ 대기 | 정답률 기반 난이도 스케일링 |
| 4.4 추가 학습 자료 추천 | ⬜ 대기 | 취약 카테고리 기반 보충 자료 |

---

## Phase 5: 검증 & 배포 ⬜

**상태**: 미착수 (Phase 4 완료 후)

| Subphase | 상태 | 설명 |
|---|---|---|
| 5.1 유닛 테스트 | ⬜ 대기 | SRS, SessionBuilder, Grader 핵심 로직 |
| 5.2 통합 테스트 | ⬜ 대기 | Telegram Bot 시뮬레이션 |
| 5.3 E2E 테스트 | ⬜ 대기 | 전체 파이프라인 검증 |
| 5.4 Docker 배포 | ⬜ 대기 | VPS or 홈서버 배포 |
| 5.5 모니터링 | ⬜ 대기 | 헬스체크, 에러 알림 |

---

## 상태 범례

| 기호 | 의미 |
|---|---|
| ✅ | 완료 |
| 🔨 | 진행 중 |
| ⬜ | 미착수 |
| ❌ | 차단됨 (사유 기록) |
