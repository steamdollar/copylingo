#!/usr/bin/env bash
set -euo pipefail

readonly DEFAULT_INTERVAL_SECONDS="2"

interval_seconds="$DEFAULT_INTERVAL_SECONDS"
run_once=false

usage() {
	cat <<'EOF'
Usage: scripts/monitor_resources.sh [--once] [--interval SECONDS]

Monitor host CPU, load, memory, swap, disk, and top processes.

Options:
  --once               Print one sample and exit.
  -i, --interval SEC   Sampling interval in seconds (default: 2).
  -h, --help           Show this help.
EOF
}

die() {
	printf '[resource-monitor] %s\n' "$*" >&2
	exit 2
}

while (($# > 0)); do
	case "$1" in
	--once)
		run_once=true
		shift
		;;
	-i | --interval)
		(($# >= 2)) || die "$1 requires a value"
		interval_seconds="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "unknown option: $1"
		;;
	esac
done

if ! awk -v value="$interval_seconds" 'BEGIN { exit !(value ~ /^[0-9]+([.][0-9]+)?$/ && value > 0) }'; then
	die "interval must be a positive number: $interval_seconds"
fi

for command_name in awk df free hostname ps; do
	command -v "$command_name" >/dev/null 2>&1 || die "required command not found: $command_name"
done

[[ -r /proc/stat && -r /proc/loadavg ]] || die "Linux /proc metrics are unavailable"

read_cpu_sample() {
	local label user nice system idle iowait irq softirq steal guest guest_nice
	read -r label user nice system idle iowait irq softirq steal guest guest_nice </proc/stat
	CPU_TOTAL=$((user + nice + system + idle + iowait + irq + softirq + steal))
	CPU_IDLE=$((idle + iowait))
}

render() {
	local cpu_percent="$1"
	local load_1 load_5 load_15 runnable
	read -r load_1 load_5 load_15 runnable _ </proc/loadavg

	if [[ -t 1 && "$run_once" == false ]]; then
		printf '\033[H\033[2J'
	fi

	printf 'Resource Monitor — %s — %s\n' "$(hostname)" "$(date '+%Y-%m-%d %H:%M:%S %Z')"
	printf 'CPU: %5.1f%%  Load: %s %s %s  Tasks: %s\n' "$cpu_percent" "$load_1" "$load_5" "$load_15" "$runnable"
	printf '\nMemory / Swap\n'
	free -h
	printf '\nFilesystems\n'
	df -h -x tmpfs -x devtmpfs --output=source,fstype,size,used,avail,pcent,target
	printf '\nTop CPU processes\n'
	ps -eo pid,user,comm,%cpu,%mem --sort=-%cpu | filter_monitor_processes
	printf '\nTop memory processes\n'
	ps -eo pid,user,comm,%cpu,%mem --sort=-%mem | filter_monitor_processes

	if [[ "$run_once" == false ]]; then
		printf '\nRefreshing every %ss — Ctrl-C to stop\n' "$interval_seconds"
	fi
}

filter_monitor_processes() {
	awk -v monitor_pid="$$" '
		NR == 1 { print; next }
		$1 != monitor_pid && $3 !~ /^(ps|awk|sleep)$/ && shown < 5 {
			print
			shown++
		}
	'
}

trap 'printf "\n"; exit 0' INT TERM

read_cpu_sample
previous_total="$CPU_TOTAL"
previous_idle="$CPU_IDLE"

while true; do
	sleep "$interval_seconds"
	read_cpu_sample

	total_delta=$((CPU_TOTAL - previous_total))
	idle_delta=$((CPU_IDLE - previous_idle))
	if ((total_delta > 0)); then
		cpu_percent="$(awk -v total="$total_delta" -v idle="$idle_delta" 'BEGIN { printf "%.1f", 100 * (total - idle) / total }')"
	else
		cpu_percent="0.0"
	fi

	render "$cpu_percent"
	[[ "$run_once" == true ]] && break

	previous_total="$CPU_TOTAL"
	previous_idle="$CPU_IDLE"
done
