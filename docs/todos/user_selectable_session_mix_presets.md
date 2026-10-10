# 사용자 선택형 세션 문제 조합 preset

## 배경 / 목적

현재 Quiz 세션(morning/evening)은 서버의 단일 정책으로 문제 조합을 결정한다.
`internal/service/session_builder.go`는 Vocabulary `ceil(total/3)`, Listening 1개, Reading 상한 1개를 예약하고
현재 레벨 문제를 약 4/5로 맞춘 뒤 나머지를 채운다(ADR-041/046/050).

사용자가 학습 목적에 따라 Vocabulary, Kana, Handwriting 비율을 선택할 수 있도록 preset 기반 설정을 추가한다.
임의 숫자 입력보다 preset을 우선하여 유효성 검증과 운영 복잡도를 제한한다.

## 실행 전 결정 필요

이 TODO는 구현 전 Case A 논의가 필요하다. 다음 항목을 사용자와 합의한 뒤 ADR에 기록한다.

1. preset 종류와 정확한 비율
2. 사용자별 기본 preset 및 변경 UX (`/menu` 설정 또는 세션 시작 직전 선택)
3. SRS due review와 선택 preset 충돌 시 우선순위
4. Vocabulary 재고 부족 시 fallback 정책

## 변경할 파일

Case A 결정 후 작성한다. 출발점:

- `internal/service/session_builder.go` — 현재 조합 규칙(`buildSession`의 예약 슬롯 계산)
- `internal/model/user.go` + `migrations/001_init.sql` — 사용자별 preset 저장
- `internal/bot/settings.go` — 설정 UI(`settings:` callback prefix)

## 검증 방법

```bash
make test
```

추가 검증:

1. 각 preset이 총 문제 수를 초과하지 않는지 확인
2. Vocabulary/Kana/Handwriting 재고 부족 시 fallback 확인
3. SRS review가 많은 저녁 세션에서 합의한 우선순위 확인
4. 기존 사용자가 migration 후 기본 preset으로 정상 동작하는지 확인

## 건드리면 안 되는 영역

- Question Seeder 데이터 자체를 preset 구현과 함께 변경하지 않는다.
- 임의 비율 입력 UI는 preset 운영 결과를 확인하기 전 추가하지 않는다.
