#!/usr/bin/env bash
# Post-process a DMG produced by `tauri build`: re-encode with lzfse (ULFO;
# Tauri only emits zlib). When an external signing service is configured,
# sign and notarize the app before repackaging, then the final image;
# otherwise use the original Apple signing flow for the DMG.
set -euo pipefail

dmg="${1:?usage: package-dmg.sh <file.dmg>}"

# Prefer the remote service when all its settings are available, even when
# Apple credentials are also configured. Otherwise use the original flow.
if [[ -n "${SIGN_AND_NOTARIZE_SCRIPT_URL:-}" &&
      -n "${SIGN_AND_NOTARIZE_BASE_URL:-}" &&
      -n "${SIGN_AND_NOTARIZE_API_TOKEN:-}" &&
      -n "${SIGN_AND_NOTARIZE_HELPER_URL:-}" ]]; then
  fail() {
    echo "package-dmg: $*" >&2
    exit 1
  }

  [[ -f "$dmg" ]] || fail "$dmg does not exist"
  dmg="$(cd "$(dirname "$dmg")" && pwd)/$(basename "$dmg")"
  # Match rename-release-assets.mjs, using the artifact's version and target.
  # The remote script derives its upload ZIP name from the input basename.
  dmg_name="$(basename "$dmg")"
  if [[ "$dmg_name" =~ ^(.+)_([^_]+)_(aarch64|arm64|x64|x86_64|amd64)\.dmg$ ]]; then
    version="${BASH_REMATCH[2]}"
    case "${BASH_REMATCH[3]}" in
      aarch64|arm64) arch=arm64 ;;
      x64|x86_64|amd64) arch=x64 ;;
    esac
    release_stem="DEEIX-Chat-$version-macos-$arch"
  elif [[ "$dmg_name" =~ ^DEEIX-Chat-(.+)-macos-(arm64|x64)\.dmg$ ]]; then
    release_stem="${dmg_name%.dmg}"
  else
    fail "cannot derive a versioned release name from $dmg_name"
  fi
  # Keep the final rename on the same filesystem, and leave the original intact
  # until both signing requests and the repackaging have succeeded.
  workdir="$(mktemp -d "$(dirname "$dmg")/.package-dmg.XXXXXX")"
  mount="$workdir/mount"
  mounted=false
  cleanup() {
    if $mounted; then
      if ! hdiutil detach -quiet "$mount" && ! hdiutil detach -force -quiet "$mount"; then
        echo "package-dmg: could not unmount $mount; keeping $workdir" >&2
        return
      fi
    fi
    rm -rf "$workdir"
  }
  trap cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  # Fetch once so both requests use the same script. A failed download must stop
  # here rather than passing an empty command to bash -c and reporting success.
  signing_script="$(curl -fsSL -- "$SIGN_AND_NOTARIZE_SCRIPT_URL")"
  [[ -n "$signing_script" ]] || fail "the signing script is empty"
  sign_and_notarize() {
    API_BASE_URL="$SIGN_AND_NOTARIZE_BASE_URL" \
    API_TOKEN="$SIGN_AND_NOTARIZE_API_TOKEN" \
    RESULT_HELPER_URL="$SIGN_AND_NOTARIZE_HELPER_URL" \
    bash -c "$signing_script" -- "$1" "$2"
  }

  # Edit a writable copy to preserve Tauri's volume name and Finder layout.
  mkdir -p "$mount" "$workdir/signed"
  hdiutil convert "$dmg" -format UDRW -o "$workdir/writable.dmg" -quiet
  hdiutil attach -nobrowse -noautoopen -quiet -mountpoint "$mount" "$workdir/writable.dmg"
  mounted=true
  shopt -s nullglob
  apps=("$mount"/*.app)
  ((${#apps[@]} == 1)) || fail "expected one .app in $dmg, found ${#apps[@]}"
  app="${apps[0]}"
  [[ -d "$app" ]] || fail "$app is not an app bundle"
  staged_app="$workdir/$release_stem.app"
  signed_app="$workdir/signed/$(basename "$app")"

  ditto "$app" "$staged_app"
  echo "Signing and notarizing $(basename "$staged_app")"
  sign_and_notarize "$staged_app" "$signed_app"
  [[ -d "$signed_app" ]] || fail "the signing script did not produce $signed_app"
  rm -rf "$app"
  ditto "$signed_app" "$app"
  hdiutil detach -quiet "$mount"
  mounted=false

  unsigned_dmg="$workdir/$release_stem.dmg"
  hdiutil convert "$workdir/writable.dmg" -format ULFO -o "$unsigned_dmg" -quiet
  signed_dmg="$workdir/signed/$(basename "$dmg")"
  echo "Signing and notarizing $(basename "$unsigned_dmg")"
  sign_and_notarize "$unsigned_dmg" "$signed_dmg"
  [[ -s "$signed_dmg" ]] || fail "the signing script did not produce $signed_dmg"
  mv -f "$signed_dmg" "$dmg"
else
  tmp="${dmg%.dmg}.ulfo.dmg"
  hdiutil convert "$dmg" -format ULFO -o "$tmp" -quiet
  mv -f "$tmp" "$dmg"

  # An ad-hoc identity ("-") cannot be timestamped or notarized.
  if [[ -n "${APPLE_TEAM_ID:-}" ]]; then
    codesign --force --sign "$APPLE_SIGNING_IDENTITY" --timestamp "$dmg"
    xcrun notarytool submit "$dmg" --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
    xcrun stapler staple "$dmg"
  fi
fi
