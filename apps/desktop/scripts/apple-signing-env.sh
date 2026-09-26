#!/usr/bin/env bash
# Derive APPLE_SIGNING_IDENTITY and APPLE_TEAM_ID from the Developer ID
# Application certificate, so the .p12 (+ password) is the only signing input
# locally and in CI. Reads APPLE_CERTIFICATE (base64) or APPLE_CERTIFICATE_PATH
# and APPLE_CERTIFICATE_PASSWORD; prints KEY=VALUE lines.
set -euo pipefail

if [[ -n "${APPLE_CERTIFICATE_PATH:-}" ]]; then
  p12="$APPLE_CERTIFICATE_PATH"
elif [[ -n "${APPLE_CERTIFICATE:-}" ]]; then
  p12="$(mktemp)"; trap 'rm -f "$p12"' EXIT
  printf '%s' "$APPLE_CERTIFICATE" | base64 --decode > "$p12"
else
  exit 0
fi

subject="$(openssl pkcs12 -in "$p12" -nokeys -clcerts -passin "pass:${APPLE_CERTIFICATE_PASSWORD:-}" -legacy 2>/dev/null \
  || openssl pkcs12 -in "$p12" -nokeys -clcerts -passin "pass:${APPLE_CERTIFICATE_PASSWORD:-}" 2>/dev/null)"
subject="$(printf '%s' "$subject" | openssl x509 -noout -subject -nameopt sep_multiline,utf8)"

identity="$(printf '%s\n' "$subject" | sed -n 's/^ *CN=//p' | head -1)"
team="$(printf '%s\n' "$subject" | sed -n 's/^ *OU=//p' | head -1)"
if [[ -z "$identity" || -z "$team" ]]; then
  echo "apple-signing-env: could not read CN/OU from certificate" >&2
  exit 1
fi
printf 'APPLE_SIGNING_IDENTITY=%s\nAPPLE_TEAM_ID=%s\n' "$identity" "$team"
