# Host resource monitor script

## 변경 내용

- `scripts/monitor_resources.sh`를 추가했다.
- Linux `/proc/stat` delta로 전체 CPU 사용률을 계산한다.
- 1/5/15분 load average, runnable task, memory/swap, filesystem 사용량을 함께 표시한다.
- CPU·memory 기준 상위 process를 각각 5개 표시하되 monitor 자체와 측정용 process는 제외한다.
- 기본 2초 갱신, `--interval`, 자동 검증용 `--once`, `--help`를 지원한다.

## 검증

- `bash -n scripts/monitor_resources.sh` — 통과
- `scripts/monitor_resources.sh --once --interval 0.1` — 실제 host metric 출력 확인
- invalid interval `0` — exit code 2로 거부 확인
- `shellcheck` — 현재 machine에 설치되지 않아 생략
- `make test` — 통과
