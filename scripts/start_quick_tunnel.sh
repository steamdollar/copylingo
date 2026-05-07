#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${COPYLINGO_ENV_FILE:-$PROJECT_ROOT/.env}"
TARGET_URL="${1:-${COPYLINGO_TUNNEL_TARGET_URL:-http://localhost:8080}}"
ENV_KEY="COPYLINGO_SERVER_PUBLIC_BASE_URL"
URL_PATTERN='https://[[:alnum:]-]+\.trycloudflare\.com'

if ! command -v cloudflared >/dev/null 2>&1; then
	echo "[copylingo] cloudflared is not installed or not in PATH" >&2
	exit 1
fi

update_env() {
	local public_url="$1"
	local tmp_file

	touch "$ENV_FILE"
	chmod 600 "$ENV_FILE" 2>/dev/null || true
	tmp_file="$(mktemp)"

	awk -v key="$ENV_KEY" -v value="$public_url" '
		BEGIN { found = 0 }
		$0 ~ "^" key "=" {
			print key "=" value
			found = 1
			next
		}
		{ print }
		END {
			if (!found) {
				print key "=" value
			}
		}
	' "$ENV_FILE" >"$tmp_file"

	mv "$tmp_file" "$ENV_FILE"
}

found_url_file="$(mktemp)"
trap 'rm -f "$found_url_file"' EXIT

echo "[copylingo] starting Cloudflare Quick Tunnel -> $TARGET_URL"
echo "[copylingo] waiting for trycloudflare.com URL..."

cloudflared tunnel --url "$TARGET_URL" 2>&1 | while IFS= read -r line; do
	echo "$line"

	if [ -s "$found_url_file" ]; then
		continue
	fi

	public_url="$(printf '%s\n' "$line" | grep -Eo "$URL_PATTERN" | head -n 1 || true)"
	if [ -z "$public_url" ]; then
		continue
	fi

	update_env "$public_url"
	printf '%s' "$public_url" >"$found_url_file"

	echo "[copylingo] updated $ENV_FILE"
	echo "[copylingo] $ENV_KEY=$public_url"
	echo "[copylingo] restart CopyLingo server to load the updated .env"
done
