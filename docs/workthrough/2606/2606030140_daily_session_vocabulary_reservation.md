# Daily Session Vocabulary 최소 1/3 예약 정책 적용

## 배경

기존 Random Slot Relay는 `kana -> handwriting -> vocabulary -> grammar -> fallback` 순서로
새 문항 슬롯을 배분했다. 앞 카테고리와 낮은 `difficulty` fallback 때문에 Vocabulary 문항이
세션에서 지나치게 적게 노출될 수 있었다.

## 변경 파일

- `internal/service/session_builder.go`
- `internal/service/session_builder_test.go`
- `docs/adr/ADR_from_21_to_40.md`
- `docs/todos/user_selectable_session_mix_presets.md`
- `STATUS.md`

## 결정 사항

- Daily Session은 총 문제 수의 `ceil(1/3)`을 신규 Vocabulary 슬롯으로 먼저 예약한다.
- 예약 Vocabulary가 부족하면 기존 Random Slot Relay가 빈 슬롯을 채운다.
- 총 문제 수를 유지하기 위해 review 상한을 예약 슬롯만큼 줄인다.
- 기존 `remainingNew` 최소값 재보정은 예약 슬롯과 중복되어 총 문제 수를 초과할 수 있으므로 제거했다.
- Morning Session은 기존 review 최대 6개를 유지한다.
- Evening Session은 review 최대 8개에서 6개로 줄이고 Vocabulary 4개를 우선한다.
- Review 전용 Session은 변경하지 않는다.
- 사용자 선택형 세션 조합 preset은 별도 TODO로 분리했다.

## 테스트

- Vocabulary가 충분한 Evening Session에서 `review 6 + vocabulary 4` 검증
- Vocabulary가 부족한 Evening Session에서 Relay fallback으로 총 10문제 유지 검증

## 검증 결과

- `go test ./internal/service -run 'TestBuild(Morning|Evening|Review|Session)' -count=10 -v`: PASS
- `make test`: PASS
