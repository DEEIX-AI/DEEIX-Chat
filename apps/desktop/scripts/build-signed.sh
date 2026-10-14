#!/usr/bin/env bash
# Local signed build: loads deploy/secrets/desktop-signing.env, builds, packages
# the DMG and verifies what ships (verify-macos-bundle.sh). Same inputs as CI.
set -euo pipefail

root="$(cd "$(dirname "$0")/../../.." && pwd)"
env_file="$root/deploy/secrets/desktop-signing.env"
if [[ ! -f "$env_file" ]]; then
  echo "missing $env_file — copy desktop-signing.env.example and fill it in" >&2
  exit 1
fi
set -a; source "$env_file"; set +a

# Tauri treats a variable that merely *exists* as configured, so drop every
# key the env file left blank.
while IFS='=' read -r name _; do
  [[ "$name" =~ ^[A-Z_]+$ && -z "${!name:-}" ]] && unset "$name"
done < "$env_file"

# Tauri requires the password variable to exist even for a key without one.
export TAURI_SIGNING_PRIVATE_KEY_PASSWORD="${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"

# Tauri reads key and certificates from the environment as content, not paths.
if [[ -n "${TAURI_SIGNING_PRIVATE_KEY_PATH:-}" && -z "${TAURI_SIGNING_PRIVATE_KEY:-}" ]]; then
  export TAURI_SIGNING_PRIVATE_KEY="$(cat "$root/$TAURI_SIGNING_PRIVATE_KEY_PATH")"
fi
if [[ -n "${APPLE_CERTIFICATE_PATH:-}" && -z "${APPLE_CERTIFICATE:-}" ]]; then
  export APPLE_CERTIFICATE="$(base64 -i "$root/$APPLE_CERTIFICATE_PATH")"
fi
if [[ -n "${WINDOWS_CERTIFICATE_PATH:-}" && -z "${WINDOWS_CERTIFICATE:-}" ]]; then
  export WINDOWS_CERTIFICATE="$(base64 -i "$root/$WINDOWS_CERTIFICATE_PATH")"
fi
if [[ -n "${APPLE_CERTIFICATE:-}" ]]; then
  while IFS='=' read -r key value; do
    export "$key=$value"
  done < <(bash "$root/apps/desktop/scripts/apple-signing-env.sh")
elif [[ "$(uname)" == "Darwin" && -z "${APPLE_SIGNING_IDENTITY:-}" ]]; then
  # Tauri skips codesign without an identity, and the linker's own signature
  # is not a valid bundle signature: macOS reports the app as damaged.
  export APPLE_SIGNING_IDENTITY="-"
  echo "note: no Developer ID configured; the app will be ad-hoc signed and not notarized" >&2
fi

cd "$root"
bundle="$root/apps/desktop/src-tauri/target/release/bundle"
if [[ "$(uname)" == "Darwin" ]]; then
  # Tauri leaves the bundles of earlier builds in place; start clean so only
  # this build is packaged and verified.
  rm -rf "$bundle/dmg" "$bundle/macos"
fi

pnpm --filter @deeix/desktop build

if [[ "$(uname)" == "Darwin" ]]; then
  shopt -s nullglob
  dmgs=("$bundle"/dmg/*.dmg)
  archives=("$bundle"/macos/*.app.tar.gz)
  if ((${#dmgs[@]} != 1 || ${#archives[@]} != 1)); then
    echo "expected one DMG and one updater archive under $bundle, found ${#dmgs[@]} and ${#archives[@]}" >&2
    exit 1
  fi
  bash "$root/apps/desktop/scripts/package-dmg.sh" "${dmgs[0]}"
  bash "$root/apps/desktop/scripts/verify-macos-bundle.sh" --arch "$(uname -m)" "${dmgs[0]}" "${archives[0]}"
fi
