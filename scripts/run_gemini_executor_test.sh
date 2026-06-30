#!/usr/bin/env bash
# run_gemini_executor_test.sh — self-test for run_gemini_executor.sh.
#
# Uses a fake `gemini` executable placed first in PATH. The real Gemini CLI is
# never invoked. Each test runs in an isolated temporary directory cleaned with
# trap. Retry sleeps are disabled with SUBAGENT_RETRY_SLEEP_SCALE=0.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRAPPER="$SCRIPT_DIR/run_gemini_executor.sh"

PASS=0
FAIL=0

# Distinct exit codes mirrored from the wrapper for assertions.
EXIT_USAGE=2
EXIT_PROVIDER=3
EXIT_TOOLCALL=4
EXIT_GEMINI=5

TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT

# --- Fake gemini factory ----------------------------------------------------
# Builds a sandbox directory containing:
#   - bin/gemini   : fake executable driven by env files in the sandbox
#   - docs/...     : the contract docs the prompt references (content irrelevant)
#   - docs/todos/task.md : a readable TODO path
#
# The fake reads control variables from $sandbox/control.env:
#   FAKE_MODE        = success | provider | toolcall | fail
#   FAKE_FAIL_COUNT  = number of leading provider failures (provider mode)
#   FAKE_TOOLCALL_MSG= message body for toolcall mode
# Attempt counting is persisted per (model) in $sandbox/state/<model>.count so
# "fail N times then succeed" works across wrapper retries.
make_sandbox() {
	local sandbox
	sandbox="$(mktemp -d "$TEST_ROOT/sb.XXXXXX")"
	mkdir -p "$sandbox/bin" "$sandbox/docs/todos" "$sandbox/state"
	printf 'contract\n' >"$sandbox/docs/GEMINI_CLI_EXECUTION.md"
	printf 'delegation\n' >"$sandbox/docs/GEMINI_CLI_DELEGATION.md"
	printf '# task\n' >"$sandbox/docs/todos/task.md"

	cat >"$sandbox/bin/gemini" <<'FAKE'
#!/usr/bin/env bash
# Fake gemini. Behavior is driven by $SANDBOX/control.env and per-model counters.
set -u
sandbox="${SANDBOX:?SANDBOX must be set}"
# shellcheck disable=SC1090
. "$sandbox/control.env"

# Extract the requested model from "-m <model>".
model="unknown"
prev=""
for arg in "$@"; do
	if [ "$prev" = "-m" ]; then
		model="$arg"
	fi
	prev="$arg"
done

count_file="$sandbox/state/${model}.count"
attempt=0
[ -f "$count_file" ] && attempt="$(cat "$count_file")"
attempt=$((attempt + 1))
printf '%s' "$attempt" >"$count_file"

case "${FAKE_MODE:-success}" in
	success)
		echo "fake gemini: applied edits successfully"
		exit 0
		;;
	provider)
		# FAKE_FAIL_MODEL (optional): restrict failures to one model so a test
		# can exhaust the default model and still let the fallback succeed.
		if [ -n "${FAKE_FAIL_MODEL:-}" ] && [ "$model" != "$FAKE_FAIL_MODEL" ]; then
			echo "fake gemini: applied edits successfully"
			exit 0
		fi
		if [ "$attempt" -le "${FAKE_FAIL_COUNT:-0}" ]; then
			echo "Error: 429 MODEL_CAPACITY_EXHAUSTED" >&2
			exit 1
		fi
		echo "fake gemini: recovered and applied edits"
		exit 0
		;;
	toolcall)
		echo "${FAKE_TOOLCALL_MSG:-malformed tool call}" >&2
		# Tool Call errors may surface with a zero or non-zero exit; the
		# wrapper must classify by pattern, not exit code. Use 0 here to prove
		# pattern detection wins.
		exit 0
		;;
	fail)
		echo "fake gemini: unexpected internal failure" >&2
		exit 1
		;;
	*)
		echo "fake gemini: unknown FAKE_MODE" >&2
		exit 99
		;;
esac
FAKE
	chmod +x "$sandbox/bin/gemini"
	printf '%s' "$sandbox"
}

# Runs the wrapper inside a sandbox with the fake gemini first in PATH.
# Echoes the wrapper exit status; sandbox path passed in $1, control vars in env.
run_wrapper() {
	local sandbox="$1"
	(
		cd "$sandbox" || exit 127
		PATH="$sandbox/bin:$PATH" \
		SANDBOX="$sandbox" \
		SUBAGENT_RETRY_SLEEP_SCALE=0 \
			bash "$WRAPPER" "${WRAPPER_ARGS[@]}" >"$sandbox/out.log" 2>&1
	)
}

assert_exit() {
	local name="$1" expected="$2" actual="$3" sandbox="$4"
	if [ "$actual" -eq "$expected" ]; then
		printf 'PASS  %-48s exit=%s\n' "$name" "$actual"
		PASS=$((PASS + 1))
	else
		printf 'FAIL  %-48s expected=%s actual=%s\n' "$name" "$expected" "$actual"
		[ -f "$sandbox/out.log" ] && sed 's/^/      | /' "$sandbox/out.log"
		FAIL=$((FAIL + 1))
	fi
}

# Asserts the fake gemini was called exactly N times for a model.
assert_calls() {
	local name="$1" model="$2" expected="$3" sandbox="$4"
	local actual=0
	[ -f "$sandbox/state/${model}.count" ] && actual="$(cat "$sandbox/state/${model}.count")"
	if [ "$actual" -eq "$expected" ]; then
		printf 'PASS  %-48s %s calls=%s\n' "$name" "$model" "$actual"
		PASS=$((PASS + 1))
	else
		printf 'FAIL  %-48s %s expected=%s actual=%s\n' "$name" "$model" "$expected" "$actual"
		FAIL=$((FAIL + 1))
	fi
}

write_control() {
	local sandbox="$1"; shift
	: >"$sandbox/control.env"
	for kv in "$@"; do
		printf '%s\n' "$kv" >>"$sandbox/control.env"
	done
}

DEFAULT_MODEL="gemini-3.1-flash-lite"
FALLBACK_MODEL="gemini-2.5-flash"

# === Test 1: Missing TODO path fails before invocation ======================
{
	sandbox="$(make_sandbox)"
	write_control "$sandbox" "FAKE_MODE=success"
	WRAPPER_ARGS=("docs/todos/does_not_exist.md")
	run_wrapper "$sandbox"; rc=$?
	assert_exit "missing-todo-path" "$EXIT_USAGE" "$rc" "$sandbox"
	# gemini must never have been called.
	assert_calls "missing-todo-no-invoke" "$DEFAULT_MODEL" 0 "$sandbox"
}

# === Test 2: Successful call exits zero =====================================
{
	sandbox="$(make_sandbox)"
	write_control "$sandbox" "FAKE_MODE=success"
	WRAPPER_ARGS=("docs/todos/task.md")
	run_wrapper "$sandbox"; rc=$?
	assert_exit "success-exit-zero" 0 "$rc" "$sandbox"
	assert_calls "success-single-call" "$DEFAULT_MODEL" 1 "$sandbox"
}

# === Test 3: Two provider failures then success, same model =================
{
	sandbox="$(make_sandbox)"
	write_control "$sandbox" "FAKE_MODE=provider" "FAKE_FAIL_COUNT=2"
	WRAPPER_ARGS=("docs/todos/task.md")
	run_wrapper "$sandbox"; rc=$?
	assert_exit "two-provider-fail-then-success" 0 "$rc" "$sandbox"
	# initial + 2 retries = 3 calls, all on default model; no fallback.
	assert_calls "retry-same-model" "$DEFAULT_MODEL" 3 "$sandbox"
	assert_calls "no-fallback-on-recovery" "$FALLBACK_MODEL" 0 "$sandbox"
}

# === Test 4: Three provider failures switch to fallback =====================
{
	sandbox="$(make_sandbox)"
	# Default model fails every attempt; fallback model succeeds immediately.
	write_control "$sandbox" "FAKE_MODE=provider" "FAKE_FAIL_COUNT=99" \
		"FAKE_FAIL_MODEL=$DEFAULT_MODEL"
	WRAPPER_ARGS=("docs/todos/task.md")
	run_wrapper "$sandbox"; rc=$?
	assert_exit "three-provider-fail-switch-fallback" 0 "$rc" "$sandbox"
	# Default model: initial + 2 retries = 3 attempts, all provider errors.
	assert_calls "default-exhausted" "$DEFAULT_MODEL" 3 "$sandbox"
	# Fallback model: invoked once and succeeds.
	assert_calls "fallback-invoked" "$FALLBACK_MODEL" 1 "$sandbox"
}

# === Test 5: Provider failures exhausted on both models exit non-zero =======
{
	sandbox="$(make_sandbox)"
	# Large fail count so neither model ever recovers.
	write_control "$sandbox" "FAKE_MODE=provider" "FAKE_FAIL_COUNT=99"
	WRAPPER_ARGS=("docs/todos/task.md")
	run_wrapper "$sandbox"; rc=$?
	assert_exit "both-models-exhausted" "$EXIT_PROVIDER" "$rc" "$sandbox"
	assert_calls "default-3-attempts" "$DEFAULT_MODEL" 3 "$sandbox"
	assert_calls "fallback-3-attempts" "$FALLBACK_MODEL" 3 "$sandbox"
}

# === Test 6: Each known Tool Call error exits non-zero, no auto-retry =======
for msg in \
	"Invalid stream" \
	"malformed tool call" \
	"error: required property replace.old_string is missing" \
	"could not find the string to replace"; do
	sandbox="$(make_sandbox)"
	write_control "$sandbox" "FAKE_MODE=toolcall" "FAKE_TOOLCALL_MSG=$msg"
	WRAPPER_ARGS=("docs/todos/task.md")
	run_wrapper "$sandbox"; rc=$?
	label="toolcall:[$msg]"
	assert_exit "$label" "$EXIT_TOOLCALL" "$rc" "$sandbox"
	# No auto-retry: exactly one call, no fallback.
	assert_calls "toolcall-no-retry" "$DEFAULT_MODEL" 1 "$sandbox"
	assert_calls "toolcall-no-fallback" "$FALLBACK_MODEL" 0 "$sandbox"
	# Recovery prompt must be surfaced.
	if grep -q "Inspect existing changes first" "$sandbox/out.log"; then
		printf 'PASS  %-48s recovery-prompt-printed\n' "$label"
		PASS=$((PASS + 1))
	else
		printf 'FAIL  %-48s recovery-prompt-missing\n' "$label"
		FAIL=$((FAIL + 1))
	fi
done

# === Test 7: Normal non-zero Gemini failure exits non-zero ==================
{
	sandbox="$(make_sandbox)"
	write_control "$sandbox" "FAKE_MODE=fail"
	WRAPPER_ARGS=("docs/todos/task.md")
	run_wrapper "$sandbox"; rc=$?
	assert_exit "plain-cli-failure" "$EXIT_GEMINI" "$rc" "$sandbox"
	# Plain failure is terminal on the first model; no retry, no fallback.
	assert_calls "plain-fail-no-retry" "$DEFAULT_MODEL" 1 "$sandbox"
	assert_calls "plain-fail-no-fallback" "$FALLBACK_MODEL" 0 "$sandbox"
}

# --- Summary ----------------------------------------------------------------
echo "----------------------------------------"
printf 'TOTAL  pass=%s  fail=%s\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
