#!/usr/bin/env bash
# Build an AppImage carrying the GUI and its vkturn/xray/byedpi children, and nothing else:
# GTK4 and WebKitGTK come from the host, exactly like the .deb's depends.
#
# Bundling them is not an option. WebKitGTK spawns WebKitNetworkProcess / WebKitWebProcess
# from a libexec path baked into libwebkitgtk at compile time, and distro builds drop the
# WEBKIT_EXEC_PATH override that used to relocate it, so a bundled WebKit looks for its
# helpers under the *build* host's path (Ubuntu: /usr/lib/x86_64-linux-gnu/webkitgtk-6.0)
# and aborts on any distro that lays them out elsewhere. Bundling the rest of the stack
# around a host WebKit only mixes ABIs, so nothing is deployed at all.
#
# Requires on the host: gtk4 and webkitgtk-6.0 (see build/linux/nfpm.yaml depends).
#
# Usage: build/linux/appimage/build-appimage.sh <arch> <bindir> <outdir>
#   arch:   x86_64 | aarch64
#   bindir: directory containing the built wingsv-dex, vkturn, xray and byedpi
#   outdir: where the .AppImage is written
set -euxo pipefail

# Tool arch (appimagetool requires x86_64/aarch64); the output file is named with the
# friendly amd64/arm64 instead.
ARCH="${1:-x86_64}"
BINDIR="$(readlink -f "${2:-bin}")"
OUTDIR="$(readlink -f "${3:-dist}")"
case "$ARCH" in
  x86_64) PKGARCH=amd64 ;;
  aarch64) PKGARCH=arm64 ;;
  *) PKGARCH="$ARCH" ;;
esac
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"

export APPIMAGE_EXTRACT_AND_RUN=1   # AppImage tools cannot FUSE-mount in CI/sandboxes
export ARCH

WORK="$HERE/build"
AD="$WORK/wingsv-dex-${ARCH}.AppDir"
rm -rf "$AD"
mkdir -p "$AD/usr/bin"

cp "$BINDIR/wingsv-dex" "$AD/usr/bin/wingsv-dex"
cp "$BINDIR/vkturn" "$AD/usr/bin/vkturn"
cp "$BINDIR/xray" "$AD/usr/bin/xray"
cp "$BINDIR/byedpi" "$AD/usr/bin/byedpi"
cp "$ROOT/build/appicon.png" "$AD/wingsv-dex.png"
cp "$ROOT/build/linux/wingsv-dex.desktop" "$AD/wingsv-dex.desktop"
mkdir -p "$AD/usr/share/applications" "$AD/usr/share/icons/hicolor/512x512/apps"
cp "$AD/wingsv-dex.desktop" "$AD/usr/share/applications/"
cp "$AD/wingsv-dex.png" "$AD/usr/share/icons/hicolor/512x512/apps/"

# helperBinaryPath resolves the children next to the executable, so AppRun must exec the
# real binary in usr/bin rather than copy it to the AppDir root.
cat > "$AD/AppRun" <<'RUN'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/wingsv-dex" "$@"
RUN
chmod +x "$AD/AppRun"

cd "$WORK"
curl -fsSL -o "appimagetool-${ARCH}.AppImage" \
  "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage"
chmod +x "appimagetool-${ARCH}.AppImage"

mkdir -p "$OUTDIR"
"$WORK/appimagetool-${ARCH}.AppImage" "$AD" "$OUTDIR/wingsv-dex-${PKGARCH}.AppImage"
echo "AppImage: $OUTDIR/wingsv-dex-${PKGARCH}.AppImage"
