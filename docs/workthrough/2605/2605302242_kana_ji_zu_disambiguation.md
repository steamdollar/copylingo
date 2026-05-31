# kana ji/zu 역방향 문항 행 힌트 추가

## 배경

가나 역방향 문항은 Romaji 발음을 보고 문자를 입력하거나 손글씨로 쓰게 한다.
현대 표준 일본어에서 `じ/ぢ`와 `ジ/ヂ`는 각각 `ji`, `ず/づ`와 `ズ/ヅ`는 각각 `zu`로
표기되므로, 기존 prompt만으로는 사용자가 정답 문자를 구분할 수 없었다.

## 변경 사항

### `cmd/ja/kana_seeder/main.go`

- `kanaDisambiguationHint` helper를 추가했다.
- `ji/zu`가 중복되는 8개 문자에만 원형 행과 탁점 힌트를 추가했다.
  - `じ/ず`: `さ행에 탁점`
  - `ぢ/づ`: `た행에 탁점`
  - `ジ/ズ`: `サ행에 탁점`
  - `ヂ/ヅ`: `タ행에 탁점`
- Type 2 Romaji -> Kana 문항과 Type 3 손글씨 문항에 동일한 규칙을 적용했다.
- 특정 원형 문자를 직접 제시하지 않아 퀴즈의 학습 가치를 유지했다.

### `cmd/ja/kana_seeder/main_test.go`

- 8개 예외 문자와 일반 kana의 hint helper 결과를 검증했다.
- Type 2와 Type 3 prompt에 행 힌트가 포함되는지 검증했다.
- 일반 kana prompt에는 힌트가 추가되지 않는지 검증했다.

## 로컬 DB 보정

기존 `questions` row를 삭제하거나 재-seed하지 않고 `prompt`만 `UPDATE`했다.
따라서 기존 `question_id`, SRS 상태, 완료 세션 참조는 유지된다.

- 갱신 row: 16개
- 진행 중 세션: 0개
- Redis working set flush: 불필요

## 검증

```bash
go test ./cmd/ja/kana_seeder
make test
git diff --check
```

결과: 모두 통과.
