#!/usr/bin/env bash
# Select one signing provider and export only the credentials Tauri should use.
set -euo pipefail

: "${GITHUB_ENV:?GITHUB_ENV is required}"
script_dir="$(cd "$(dirname "$0")" && pwd)"

remote_count=0
for name in SIGN_AND_NOTARIZE_SCRIPT_URL SIGN_AND_NOTARIZE_BASE_URL SIGN_AND_NOTARIZE_API_TOKEN SIGN_AND_NOTARIZE_HELPER_URL; do
  [[ -z "${!name:-}" ]] || remote_count=$((remote_count + 1))
done
if ((remote_count > 0 && remote_count < 4)); then
  echo "::error::External signing is partly configured; set all four SIGN_AND_NOTARIZE_* secrets."
  exit 1
fi
if ((remote_count == 4)); then
  # The remote service signs the final app. Tauri only supplies an ad-hoc app,
  # even when local Apple credentials also exist, so it does not notarize twice.
  printf '%s\n' 'DESKTOP_SIGNING_MODE=external' 'APPLE_SIGNING_IDENTITY=-' >> "$GITHUB_ENV"
  exit 0
fi

present=0
missing=()
for name in APPLE_CERTIFICATE APPLE_CERTIFICATE_PASSWORD APPLE_ID APPLE_PASSWORD; do
  if [[ -n "${!name:-}" ]]; then
    present=$((present + 1))
  elif [[ "$name" != APPLE_CERTIFICATE_PASSWORD ]]; then
    missing+=("$name")
  fi
done
if ((present == 0)); then
  printf '%s\n' 'DESKTOP_SIGNING_MODE=adhoc' 'APPLE_SIGNING_IDENTITY=-' >> "$GITHUB_ENV"
  echo "::warning::No Apple signing provider configured; building an ad-hoc signed app."
  exit 0
fi
if ((${#missing[@]} > 0)); then
  echo "::error::Apple signing is partly configured; missing ${missing[*]}."
  exit 1
fi

echo 'DESKTOP_SIGNING_MODE=apple' >> "$GITHUB_ENV"
for name in APPLE_CERTIFICATE APPLE_CERTIFICATE_PASSWORD APPLE_ID APPLE_PASSWORD; do
  delimiter=__DESKTOP_SIGNING__
  while [[ "${!name:-}" == *"$delimiter"* ]]; do delimiter+=_; done
  printf '%s<<%s\n%s\n%s\n' "$name" "$delimiter" "${!name:-}" "$delimiter" >> "$GITHUB_ENV"
done
bash "$script_dir/apple-signing-env.sh" >> "$GITHUB_ENV"
