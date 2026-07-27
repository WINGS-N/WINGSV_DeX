#!/bin/sh
# Refresh the desktop MIME cache so the wingsv:// / vless:// scheme handlers from
# wingsv-dex.desktop take effect without a re-login. Best-effort: absent on minimal systems.
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi
