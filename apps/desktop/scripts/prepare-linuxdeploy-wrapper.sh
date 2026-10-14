#!/usr/bin/env bash
# Replaces tauri-cli's cached linuxdeploy with a wrapper that appends
# --exclude-library for each libwayland-*. tauri-cli would otherwise bundle
# the build container's Wayland libraries into AppImages, which shadow the
# host's libraries at runtime and break EGL on newer Wayland distros
# (blank window). The wrapper is installed at tauri-cli's cache path
# (~/.cache/tauri/linuxdeploy-<arch>.AppImage) and reused verbatim; if a
# future tauri-cli stops reusing it, the verification step in
# desktop-release.yml fails loudly.
#
# Usage: prepare-linuxdeploy-wrapper.sh [arch]   (default: x86_64)
set -euo pipefail

arch="${1:-x86_64}"
linuxdeploy_name="linuxdeploy-${arch}.AppImage"

tools_root="${XDG_CACHE_HOME:-$HOME/.cache}/tauri"
wrapper_tools="$tools_root/.linuxdeploy-wrapper-tools"
cache_path="$tools_root/$linuxdeploy_name"

mkdir -p "$tools_root" "$wrapper_tools"

# Retry flaky downloads and pre-populate the files tauri-cli would
# otherwise fetch itself (it reuses existing cache entries).
curl_retry=(curl -fsSL --retry 3 --retry-delay 2 --retry-all-errors)

"${curl_retry[@]}" -o "$tools_root/AppRun-${arch}" \
  "https://github.com/tauri-apps/binary-releases/releases/download/apprun-old/AppRun-${arch}"
chmod +x "$tools_root/AppRun-${arch}"

"${curl_retry[@]}" -o "$tools_root/linuxdeploy-plugin-gtk.sh" \
  "https://raw.githubusercontent.com/tauri-apps/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh"
"${curl_retry[@]}" -o "$tools_root/linuxdeploy-plugin-gstreamer.sh" \
  "https://raw.githubusercontent.com/tauri-apps/linuxdeploy-plugin-gstreamer/master/linuxdeploy-plugin-gstreamer.sh"
"${curl_retry[@]}" -o "$tools_root/linuxdeploy-plugin-appimage.AppImage" \
  "https://github.com/linuxdeploy/linuxdeploy-plugin-appimage/releases/download/continuous/linuxdeploy-plugin-appimage-${arch}.AppImage"
chmod +x "$tools_root/linuxdeploy-plugin-gtk.sh" \
  "$tools_root/linuxdeploy-plugin-gstreamer.sh" \
  "$tools_root/linuxdeploy-plugin-appimage.AppImage"

real_linuxdeploy="$wrapper_tools/real-linuxdeploy.AppImage"

# The same rolling URL tauri-cli 2.11.x downloads.
"${curl_retry[@]}" -o "$real_linuxdeploy" \
  "https://github.com/tauri-apps/binary-releases/releases/download/linuxdeploy/${linuxdeploy_name}"
chmod +x "$real_linuxdeploy"

# Fall back to the appimagetool bundled inside linuxdeploy-plugin-appimage
# when the continuous asset download fails.
appimagetool="$wrapper_tools/appimagetool.AppImage"
appimagetool_cmd=(env APPIMAGE_EXTRACT_AND_RUN=1 "$appimagetool" --appimage-extract-and-run)
if ! "${curl_retry[@]}" -o "$appimagetool" \
  "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${arch}.AppImage"; then
  echo "appimagetool continuous download failed; using the bundled copy" >&2
  extract_dir="$wrapper_tools/plugin-appimage-extract"
  rm -rf "$extract_dir" && mkdir -p "$extract_dir"
  (cd "$extract_dir" && "$tools_root/linuxdeploy-plugin-appimage.AppImage" --appimage-extract >/dev/null)
  appimagetool="$extract_dir/squashfs-root/usr/bin/appimagetool"
  chmod +x "$appimagetool"
  appimagetool_cmd=("$appimagetool")
else
  chmod +x "$appimagetool"
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

appdir="$workdir/wrapper.AppDir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/applications" \
  "$appdir/usr/share/icons/hicolor/256x256/apps"
cp "$real_linuxdeploy" "$appdir/usr/bin/real-linuxdeploy.AppImage"

# The inner AppImage runtime points $APPIMAGE at usr/bin/real-linuxdeploy.
# AppImage and linuxdeploy look for their plugins next to that file, so ship
# the same plugin files tauri-cli downloads (reuse the cached copies).
cp "$tools_root/linuxdeploy-plugin-gtk.sh" "$appdir/usr/bin/"
cp "$tools_root/linuxdeploy-plugin-gstreamer.sh" "$appdir/usr/bin/"
cp "$tools_root/linuxdeploy-plugin-appimage.AppImage" "$appdir/usr/bin/"
chmod +x "$appdir/usr/bin/linuxdeploy-plugin-gtk.sh" \
  "$appdir/usr/bin/linuxdeploy-plugin-gstreamer.sh"

# Both plugin scripts re-invoke "$LINUXDEPLOY" to deploy the libraries they
# collected; that second invocation bypasses the wrapper's AppRun and its
# --exclude-library arguments, so patch the same excludes into the scripts.
excludes="--exclude-library=libwayland-client.so.0 --exclude-library=libwayland-cursor.so.0 --exclude-library=libwayland-egl.so.1 --exclude-library=libwayland-server.so.0"
sed -i "s| \"\${LIBRARIES\[@\]}\"$| \"\${LIBRARIES[@]}\" ${excludes}|" \
  "$appdir/usr/bin/linuxdeploy-plugin-gtk.sh"
sed -i "s|--appdir \"\$APPDIR\"$|--appdir \"\$APPDIR\" ${excludes}|" \
  "$appdir/usr/bin/linuxdeploy-plugin-gstreamer.sh"

# 1x1 transparent PNG for the desktop entry.
printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==' \
  | base64 -d > "$appdir/usr/share/icons/hicolor/256x256/apps/wrapper.png"
cp "$appdir/usr/share/icons/hicolor/256x256/apps/wrapper.png" "$appdir/wrapper.png"
ln -sf wrapper.png "$appdir/.DirIcon"

cat > "$appdir/usr/share/applications/wrapper.desktop" <<'EOF'
[Desktop Entry]
Name=linuxdeploy-wrapper
Exec=wrapper
Icon=wrapper
Type=Application
Categories=Utility;
EOF

# appimagetool requires a desktop file at the AppDir root (same layout
# tauri's own bundler produces).
ln -sf usr/share/applications/wrapper.desktop "$appdir/wrapper.desktop"

# $APPDIR is set by the AppImage runtime and points at the wrapper's
# extracted root; capture it before the inner linuxdeploy overwrites it.
cat > "$appdir/AppRun" <<'EOF'
#!/bin/sh
# Forward every argument to the real linuxdeploy and additionally exclude
# the Wayland client libraries from dependency collection. The bundled
# plugins live in usr/bin/ next to the real linuxdeploy (the inner
# AppImage runtime repoints $APPIMAGE there); prepend that directory to
# PATH as well so plugin lookup works in both locations.
if [ -n "${LINUXDEPLOY_DEBUG_LOG:-}" ]; then
  env APPIMAGE_EXTRACT_AND_RUN=1 PATH="$APPDIR/usr/bin:$PATH" \
    "$APPDIR/usr/bin/real-linuxdeploy.AppImage" --appimage-extract-and-run "$@" \
    --exclude-library=libwayland-client.so.0 \
    --exclude-library=libwayland-cursor.so.0 \
    --exclude-library=libwayland-egl.so.1 \
    --exclude-library=libwayland-server.so.0 \
    >"$LINUXDEPLOY_DEBUG_LOG" 2>&1
  status=$?
  cat "$LINUXDEPLOY_DEBUG_LOG" >&2
  exit $status
fi
exec env APPIMAGE_EXTRACT_AND_RUN=1 PATH="$APPDIR/usr/bin:$PATH" \
  "$APPDIR/usr/bin/real-linuxdeploy.AppImage" --appimage-extract-and-run "$@" \
  --exclude-library=libwayland-client.so.0 \
  --exclude-library=libwayland-cursor.so.0 \
  --exclude-library=libwayland-egl.so.1 \
  --exclude-library=libwayland-server.so.0
EOF
chmod +x "$appdir/AppRun"

"${appimagetool_cmd[@]}" "$appdir" "$cache_path" >/dev/null

echo "linuxdeploy wrapper installed at $cache_path"
