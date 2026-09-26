#!/usr/bin/env bash
# Post-process a DMG produced by `tauri build`: re-encode with lzfse (ULFO;
# Tauri only emits zlib) and, when APPLE_* is configured, sign, notarize and
# staple the new image. Used by CI and local builds alike.
set -euo pipefail

dmg="${1:?usage: package-dmg.sh <file.dmg>}"
tmp="${dmg%.dmg}.ulfo.dmg"

hdiutil convert "$dmg" -format ULFO -o "$tmp" -quiet
mv -f "$tmp" "$dmg"

if [[ -n "${APPLE_SIGNING_IDENTITY:-}" ]]; then
  codesign --force --sign "$APPLE_SIGNING_IDENTITY" --timestamp "$dmg"
  xcrun notarytool submit "$dmg" --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
  xcrun stapler staple "$dmg"
fi
