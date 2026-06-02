# 가타카나 `ユ`·`ヲ` 손글씨 문항 제외

## 배경

가타카나 `ユ`, `ヲ`는 현재 손글씨 채점 경로에서 안정적으로 판별하기 어렵다.
객관식과 역방향 문항은 유지하고, Mini App 손글씨 문항 생성 대상에서만 제외했다.

## 변경 파일

- `cmd/ja/kana_seeder/main.go`
  - Type 3 손글씨 문항 생성 전에 `shouldSeedHandwritingQuestion` 필터를 적용했다.
  - 가타카나 `ユ`, `ヲ`만 제외하고 나머지 문자는 유지한다.
- `cmd/ja/kana_seeder/main_test.go`
  - 제외 대상과 유지 대상에 대한 회귀 테스트를 추가했다.
- `STATUS.md`
  - 최근 완료 항목을 추가했다.

## 검증 결과

```bash
make test
make build
```

모두 통과했다.
