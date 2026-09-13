# N4 catalog 및 인접 level Study·Quiz 지원

## 배경

- 기존 local DB의 일본어 N5는 868 materials, 2,998 questions로 N5 학습 범위를 충분히 제공하고 있었다.
- 실제 user progress는 material 458건, question 768건이어서 전체 catalog가 소진된 것은 아니지만, 2026년 12월 N3/N2 응시 준비를 위해 N4 원본 catalog를 선행 확충했다.
- JLPT 공식 출제 형식 기준으로 N4의 15개 item type을 모두 포함했다. 공식 기관은 type별 고정 문항 수나 vocab/grammar 목록을 공개하지 않으므로 자체 범위와 검수 기준을 적용했다.

## 결정

- Study와 Quiz 모두 사용자 현재 level과 인접한 `±1` level을 동일한 후보군으로 사용한다.
- 경계 level은 존재하는 level만 포함한다. 예: N5=`N5,N4`, N4=`N5,N4,N3`, N1=`N2,N1`.
- level별 비율 제어는 도입하지 않았다. 상세 결정은 ADR-046에 기록했다.
- 현재 N3 catalog는 아직 없으므로 N4 사용자는 당장은 N5/N4를 학습하며, N3 데이터 추가 시 코드 변경 없이 자동 포함된다.

## 구현

- `cmd/ja/catalog/data/`에 N4 원본 데이터를 추가했다.
  - vocab 600, grammar 100, reading 60, listening 80 materials
  - vocab 600, grammar 300, reading 60, listening 80 questions
  - 총 760 materials / 1,040 questions / 15 item types
- `cmd/ja/catalog`, `cmd/ja/seeder`가 N4 dataset을 검증하고 stable key 기반으로 idempotent upsert하도록 확장했다.
- `LevelCatalog` registry에 level별 dataset과 question generation policy를 모으고, material/seeder 조립을 registry loop로 전환했다. `N4ProficiencyLevel`, `N5Words` 같은 level-specific Go symbol과 level별 `switch`는 제거했다.
- `internal/service/level_policy.go`는 JLPT ordered slice 하나에서 index `±1` scope를 계산하고 Study/Quiz의 신규·복습 조회에 공통 적용했다.
- repository의 level filter를 단일 값에서 PostgreSQL `ANY($3)` 기반 배열 조건으로 변경했다.
- listening TTS CLI에 `-batch-delay`를 추가해 Gemini TTS 10 RPM quota에 맞춘 batch 처리를 지원했다.
- 관련 service/repository/bot/scheduler mock과 unit test를 변경된 interface에 맞췄다.

## 콘텐츠 검수

- 별도 audit에서 의미·정답·형식·중복을 점검했다.
- reading 1건의 어색한 표현과 orthography 6건의 kana-only 배치를 수정했다.
- listening 1건은 TTS가 짧은 발화를 audio로 반환하지 않아 자연스러운 문맥을 추가했다.
- 외부 Gemini CLI content executor는 `UNSUPPORTED_CLIENT/IneligibleTierError`로 실행할 수 없어 native executor agents로 생성·교차 검수했다.

## DB 적용 및 복구

- 대상: local Docker `copylingo-db`, database/user `copylingo`, region 없음.
- 적용 전 backup: `tmp/db-backups/copylingo_pre_n4_seed_260902.dump`
- SHA-256: `8650d73c91d25544f754a342c7ab9842ddff25530bf73b98e4e72f16eed08391`
- 복구 시 app을 중지한 뒤 별도 DB에 `pg_restore --clean --if-exists`로 검증하고 대상 DB를 교체한다.
- seeder를 2회 실행해 합계가 materials 1,628 / questions 4,038로 유지됨을 확인했다.
- 기존 progress는 material 458건, question 768건, total serves 2,776으로 적용 전후 동일했다.
- user 1명의 일본어 level을 N5에서 N4로 변경했다.
- MinIO `copylingo-audio` bucket에 N4 listening audio 80/80건을 생성했다.

## 검증

- `make test` PASS
- generic registry lookup·정규화·unknown-level fallback 및 additional-level deterministic assembly test PASS
- N4 DB 집계: materials 760, questions 1,040, item types 15, listening audio 80/80
- N5 DB 집계: materials 868, questions 2,998
- idempotent seeder 재실행 후 row/progress 집계 불변
- `make restart-app` 실행 후 `GET http://localhost:8080/health` → `healthy`
- 문서 마감 변경은 runtime behavior를 바꾸지 않아 추가 restart는 생략했다.

## 변경 파일

- Catalog/seeder: `cmd/ja/catalog/**`, `cmd/ja/seeder/**`
- TTS CLI: `cmd/admin/generate_listening_audio/main.go`, `main_test.go`
- Level scope: `internal/service/**`, `internal/repository/**`, 관련 bot/scheduler tests
- Model/decision: `internal/model/question.go`, `docs/adr/ADR_from_41_to_60.md`
