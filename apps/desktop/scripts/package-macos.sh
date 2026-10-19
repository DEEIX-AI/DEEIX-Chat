#!/usr/bin/env bash
# CI packaging phases, each exposed as a separate workflow step. The app is
# signed once; both the DMG and updater are made from that same final app.
# Usage: package-macos.sh sign-app|dmg|sign-dmg|updater|verify <triple> [src-tauri]
set -euo pipefail

phase="${1:?a packaging phase is required}"
target="${2:?a Rust target triple is required}"
script_dir="$(cd "$(dirname "$0")" && pwd)"
tauri_dir="${3:-$script_dir/../src-tauri}"
tauri_dir="$(cd "$tauri_dir" && pwd)"
case "$target" in
  aarch64-apple-darwin) arch=arm64; dmg_arch=aarch64; binary_arch=arm64 ;;
  x86_64-apple-darwin) arch=x64; dmg_arch=x64; binary_arch=x86_64 ;;
  *) echo "unsupported macOS target: $target" >&2; exit 1 ;;
esac
product="$(node -p 'JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")).productName' "$tauri_dir/tauri.conf.json")"
version="$(node -p 'JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")).version' "$tauri_dir/tauri.conf.json")"
bundle="$tauri_dir/target/$target/release/bundle"
app="$bundle/macos/$product.app"
dmg="$bundle/dmg/${product}_${version}_${dmg_arch}.dmg"
archive="$bundle/macos/$product.app.tar.gz"
release_stem="DEEIX-Chat-$version-macos-$arch"
mode="${DESKTOP_SIGNING_MODE:-adhoc}"
workdir=""
keychain=""
cleanup() {
  [[ -z "$keychain" ]] || security delete-keychain "$keychain" || true
  [[ -z "$workdir" ]] || rm -rf "$workdir"
}
trap cleanup EXIT

fail() { echo "package-macos: $*" >&2; exit 1; }
make_workdir() { workdir="$(mktemp -d "$bundle/.package-macos.XXXXXX")"; }

sign_external() {
  # Fetch once per job so app and DMG use exactly the same helper revision.
  local helper="${RUNNER_TEMP:?}/desktop-signing-$target.sh"
  if [[ ! -s "$helper" ]]; then
    curl -fsSL --connect-timeout 20 --max-time 120 --retry 2 -- "$SIGN_AND_NOTARIZE_SCRIPT_URL" > "$helper"
    [[ -s "$helper" ]] || fail "the signing helper is empty"
  fi
  API_BASE_URL="$SIGN_AND_NOTARIZE_BASE_URL" \
  API_TOKEN="$SIGN_AND_NOTARIZE_API_TOKEN" \
  RESULT_HELPER_URL="$SIGN_AND_NOTARIZE_HELPER_URL" \
    bash -c "$(cat "$helper")" -- "$1" "$2"
}

case "$phase" in
  sign-app)
    [[ "$mode" == external ]] || fail "sign-app requires the external signing provider"
    [[ -d "$app" ]] || fail "missing $app"
    make_workdir
    ditto "$app" "$workdir/$release_stem.app"
    mkdir -p "$workdir/signed"
    signed_app="$workdir/signed/$product.app"
    sign_external "$workdir/$release_stem.app" "$signed_app"
    [[ -d "$signed_app" ]] || fail "the signing helper did not produce an app"
    codesign --verify --deep --strict "$signed_app"
    team="$(codesign -dv "$signed_app" 2>&1 | sed -n 's/^TeamIdentifier=//p')"
    [[ "$team" =~ ^[A-Z0-9]{10}$ ]] || fail "the signing service returned an app without a Developer ID team"
    # verify-macos-bundle requires this same team and a stapled notarization
    # ticket on both containers, including the external signing path.
    printf 'APPLE_TEAM_ID=%s\n' "$team" >> "${GITHUB_ENV:?}"
    rm -rf "$app"
    ditto "$signed_app" "$app"
    ;;
  dmg)
    [[ -d "$app" ]] || fail "missing $app"
    make_workdir
    stage="$workdir/image"
    mkdir -p "$stage" "$bundle/dmg"
    ditto "$app" "$stage/$product.app"
    ln -s /Applications "$stage/Applications"
    if [[ -f "$tauri_dir/icons/icon.icns" ]]; then
      cp "$tauri_dir/icons/icon.icns" "$stage/.VolumeIcon.icns"
      if command -v SetFile >/dev/null; then SetFile -a C "$stage"; fi
    fi
    # Encode directly to the final format, avoiding Tauri's zlib DMG followed
    # by a writable conversion, app replacement and a second compression.
    hdiutil create -volname "$product" -srcfolder "$stage" -fs HFS+ -format ULFO -ov "$dmg"
    ;;
  sign-dmg)
    [[ -s "$dmg" ]] || fail "missing $dmg"
    if [[ "$mode" == external ]]; then
      make_workdir
      cp "$dmg" "$workdir/$release_stem.dmg"
      sign_external "$workdir/$release_stem.dmg" "$workdir/signed.dmg"
      [[ -s "$workdir/signed.dmg" ]] || fail "the signing helper did not produce a DMG"
      mv -f "$workdir/signed.dmg" "$dmg"
    elif [[ "$mode" == apple ]]; then
      # Tauri deletes its temporary keychain when app bundling exits. Import
      # the certificate for this phase without changing the user's search list.
      make_workdir
      keychain="$workdir/dmg-signing.keychain-db"
      password="$(openssl rand -hex 24)"
      printf '%s' "$APPLE_CERTIFICATE" | base64 --decode > "$workdir/certificate.p12"
      security create-keychain -p "$password" "$keychain"
      security unlock-keychain -p "$password" "$keychain"
      security import "$workdir/certificate.p12" -k "$keychain" -P "${APPLE_CERTIFICATE_PASSWORD:-}" -T /usr/bin/codesign
      security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$password" "$keychain" >/dev/null
      codesign --force --sign "$APPLE_SIGNING_IDENTITY" --keychain "$keychain" --timestamp "$dmg"
      xcrun notarytool submit "$dmg" --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
      xcrun stapler staple "$dmg"
    else
      fail "sign-dmg requires an Apple signing provider"
    fi
    ;;
  updater)
    [[ -d "$app" ]] || fail "missing $app"
    [[ -n "${TAURI_SIGNING_PRIVATE_KEY:-}" ]] || fail "updater signing key is required"
    export TAURI_SIGNING_PRIVATE_KEY_PASSWORD="${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"
    tar -czf "$archive" -C "$bundle/macos" "$product.app"
    pnpm --dir "$tauri_dir/.." exec tauri signer sign --app-version "$version" "$archive"
    [[ -s "$archive.sig" ]] || fail "the updater signature was not created"
    ;;
  verify)
    bash "$script_dir/verify-macos-bundle.sh" --arch "$binary_arch" "$dmg" "$archive"
    ;;
  *) fail "unknown packaging phase: $phase" ;;
esac
