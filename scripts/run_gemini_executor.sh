#!/usr/bin/env bash
# run_gemini_executor.sh — Gemini CLI external executor wrapper.
#
# Makes Gemini CLI delegation behavior consistent:
#   - Retries only safe, temporary provider errors (429 / 503 / capacity).
#   - Switches default -> fallback model after the retry sequence is exhausted.
#   - Detects response-level Tool Call errors but never auto-retries them,
#     because a previous run may have left valid partial edits.
#
# Contract SSOT:
#   - docs/GEMINI_CLI_DELEGATION.md  (retry / recovery policy)
#   - docs/GEMINI_CLI_EXECUTION.md   (executor contract)
#
# Usage:
#   scripts/run_gemini_executor.sh docs/todos/<task>.md
#
# Test override:
#   SUBAGENT_RETRY_SLEEP_SCALE=0 scripts/run_gemini_executor.sh ...
#     scales every retry sleep (default 1). 0 disables waiting in tests.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# --- Configuration (Decisions Already Made) ---------------------------------
DEFAULT_MODEL="gemini-3.1-flash-lite"
FALLBACK_MODEL="gemini-2.5-flash"
EXECUTION_CONTRACT="docs/GEMINI_CLI_EXECUTION.md"
DELEGATION_DOC="docs/GEMINI_CLI_DELEGATION.md"

# Retry waits per model (seconds): first failure 5s, second failure 10s.
RETRY_WAITS=(5 10)
SLEEP_SCALE="${SUBAGENT_RETRY_SLEEP_SCALE:-1}"

# Distinct exit codes for diagnosis.
EXIT_USAGE=2          # missing / unreadable TODO path
EXIT_PROVIDER=3       # temporary provider errors exhausted on both models
EXIT_TOOLCALL=4       # response-level Tool Call error (no auto-retry)
EXIT_GEMINI=5         # plain Gemini CLI failure

log() { printf '[gemini-executor] %s\n' "$*" >&2; }

# Temporary file holding the most recent Gemini CLI combined output.
OUTPUT_FILE=""
cleanup() {
	if [ -n "$OUTPUT_FILE" ]; then
		rm -f "$OUTPUT_FILE"
	fi
}
trap cleanup EXIT

# --- Argument validation ----------------------------------------------------
if [ "$#" -lt 1 ] || [ -z "${1:-}" ]; then
	log "usage: $(basename "$0") docs/todos/<task>.md"
	exit "$EXIT_USAGE"
fi

TODO_PATH="$1"
if [ ! -f "$TODO_PATH" ] || [ ! -r "$TODO_PATH" ]; then
	log "TODO path is missing or unreadable: $TODO_PATH"
	exit "$EXIT_USAGE"
fi

PROMPT="Read ${EXECUTION_CONTRACT} and ${TODO_PATH}. Execute the task exactly as specified."

# --- Error pattern detection ------------------------------------------------
# Temporary provider errors: safe to retry.
PROVIDER_ERROR_RE='429|MODEL_CAPACITY_EXHAUSTED|503|UNAVAILABLE'
# Response-level Tool Call errors: never auto-retry (partial edits may exist).
TOOLCALL_ERROR_RE='Invalid stream|malformed tool call|required property|could not find the string to replace'

is_provider_error() {
	grep -Eq "$PROVIDER_ERROR_RE" "$1"
}

is_toolcall_error() {
	grep -Eq "$TOOLCALL_ERROR_RE" "$1"
}

# --- Sleep with test override -----------------------------------------------
retry_sleep() {
	local base="$1"
	local scaled
	# Allow fractional scales (e.g. 0) without requiring bc.
	scaled="$(awk -v b="$base" -v s="$SLEEP_SCALE" 'BEGIN { printf "%g", b * s }')"
	if awk -v v="$scaled" 'BEGIN { exit !(v > 0) }'; then
		log "waiting ${scaled}s before retry"
		sleep "$scaled"
	fi
}

# --- Recovery guidance for Tool Call errors ---------------------------------
print_recovery_guidance() {
	cat >&2 <<EOF
[gemini-executor] Response-level Tool Call error detected. NOT auto-retrying.
[gemini-executor] A previous invocation may have left valid partial edits; a
[gemini-executor] blind retry can duplicate or corrupt work.
[gemini-executor]
[gemini-executor] Main agent: inspect the scoped state before any recovery session:
[gemini-executor]     git status --short
[gemini-executor]     git diff
[gemini-executor] Preserve valid existing changes. Do not revert them automatically.
[gemini-executor]
[gemini-executor] Recovery prompt (see ${DELEGATION_DOC} "Response Or Tool Call Errors"):
[gemini-executor] ---8<---
Read ${EXECUTION_CONTRACT} and ${TODO_PATH}.
The worktree may contain partial edits from a previous execution.
Inspect existing changes first. Do not duplicate completed work.
Complete only the remaining work and run the required verification commands.
[gemini-executor] --->8---
EOF
}

# --- Single Gemini CLI invocation -------------------------------------------
# Runs gemini for the given model, capturing combined output to OUTPUT_FILE for
# pattern inspection while still streaming it live (stdout/stderr preserved for
# diagnosis). Returns the gemini exit status.
run_gemini_once() {
	local model="$1"
	local status=0
	set +e
	gemini --skip-trust --approval-mode yolo \
		-m "$model" \
		-p "$PROMPT" >"$OUTPUT_FILE" 2>&1
	status=$?
	set -e
	cat "$OUTPUT_FILE"
	return "$status"
}

# --- Run one model through its retry sequence -------------------------------
# Returns:
#   0   -> success
#   10  -> provider errors exhausted for this model (caller may fall back)
#   $EXIT_TOOLCALL -> Tool Call error (terminal, already reported)
#   $EXIT_GEMINI   -> plain Gemini CLI failure (terminal)
RUN_MODEL_PROVIDER_EXHAUSTED=10
run_model_sequence() {
	local model="$1"
	local attempt
	# Attempts: initial + len(RETRY_WAITS) retries on provider errors.
	local max_attempts=$(( ${#RETRY_WAITS[@]} + 1 ))

	for (( attempt = 0; attempt < max_attempts; attempt++ )); do
		OUTPUT_FILE="$(mktemp)"
		local status=0
		run_gemini_once "$model" || status=$?

		# Tool Call errors take priority: terminal, no retry.
		if is_toolcall_error "$OUTPUT_FILE"; then
			log "model=$model tool-call error (no auto-retry)"
			print_recovery_guidance
			return "$EXIT_TOOLCALL"
		fi

		if [ "$status" -eq 0 ] && ! is_provider_error "$OUTPUT_FILE"; then
			log "model=$model succeeded"
			return 0
		fi

		if is_provider_error "$OUTPUT_FILE"; then
			if [ "$attempt" -lt "${#RETRY_WAITS[@]}" ]; then
				log "model=$model temporary provider error (attempt $((attempt + 1)))"
				retry_sleep "${RETRY_WAITS[$attempt]}"
				continue
			fi
			log "model=$model provider errors exhausted after $max_attempts attempts"
			return "$RUN_MODEL_PROVIDER_EXHAUSTED"
		fi

		# Non-zero exit without a recognized pattern: plain CLI failure.
		log "model=$model Gemini CLI failed (exit=$status)"
		return "$EXIT_GEMINI"
	done

	# Loop fell through only on provider errors.
	return "$RUN_MODEL_PROVIDER_EXHAUSTED"
}

# --- Main: default model, then fallback -------------------------------------
log "delegating TODO: $TODO_PATH"

status=0
run_model_sequence "$DEFAULT_MODEL" || status=$?
case "$status" in
	0) exit 0 ;;
	"$EXIT_TOOLCALL") exit "$EXIT_TOOLCALL" ;;
	"$EXIT_GEMINI") exit "$EXIT_GEMINI" ;;
	"$RUN_MODEL_PROVIDER_EXHAUSTED")
		log "switching to fallback model: $FALLBACK_MODEL"
		;;
	*)
		log "unexpected status from default model: $status"
		exit "$EXIT_GEMINI"
		;;
esac

status=0
run_model_sequence "$FALLBACK_MODEL" || status=$?
case "$status" in
	0) exit 0 ;;
	"$EXIT_TOOLCALL") exit "$EXIT_TOOLCALL" ;;
	"$EXIT_GEMINI") exit "$EXIT_GEMINI" ;;
	"$RUN_MODEL_PROVIDER_EXHAUSTED")
		log "provider errors exhausted on both models; reporting blocker"
		exit "$EXIT_PROVIDER"
		;;
	*)
		log "unexpected status from fallback model: $status"
		exit "$EXIT_GEMINI"
		;;
esac
