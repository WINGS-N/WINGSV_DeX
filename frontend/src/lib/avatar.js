// sizedAvatar appends the panel's avatar size hint (?s=) so the webview is handed a
// right-sized image instead of the full 460px one. WebKitGTK downscales a big PNG into a tiny
// box in one pass and turns it into a pixelated mess at header/dashboard sizes; letting the
// panel resize server-side (it snaps to its nearest canonical size: 64 / 128 / 460) fixes it.
// px is the CSS display size; the device pixel ratio is folded in so HiDPI stays crisp.
export function sizedAvatar(url, px) {
  if (!url) return url;
  const dpr = Math.min(Math.max(window.devicePixelRatio || 1, 1), 3);
  const target = Math.round(px * dpr);
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}s=${target}`;
}
