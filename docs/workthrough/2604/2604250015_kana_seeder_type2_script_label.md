# kana seeder Type 2 문항 Script Label 추가

## 배경

Type 2 문항은 Romaji 발음을 보고 일본어 문자를 입력하거나 고르는 문제다. 같은 발음이 히라가나와 가타카나에 모두 존재하기 때문에, 기존 문구인 `발음 'a'에 해당하는 문자를 입력하세요`만으로는 사용자가 어떤 script를 답해야 하는지 알 수 없었다.

## 변경 사항

- `cmd/ja/kana_seeder/main.go`
  - `kanaScriptLabel` helper를 추가해 정답 문자가 히라가나인지 가타카나인지 판별한다.
  - Type 2 prompt에 `히라가나 문자` 또는 `가타카나 문자`를 포함한다.
  - Type 2 explanation에도 동일한 script label을 포함한다.
- `.gitignore`
  - root binary ignore 의도를 유지하면서 `cmd/ja/kana_seeder/main_test.go`가 숨겨지지 않도록 `server`, `kana_seeder` 패턴을 root anchored 패턴으로 변경했다.
- `cmd/ja/kana_seeder/main_test.go`
  - script 판별 helper 테스트를 추가했다.
  - Type 2 prompt/explanation에 script label이 포함되는지 검증했다.

## 검증

```bash
go test ./cmd/ja/kana_seeder
go test ./...
```

결과: 통과
