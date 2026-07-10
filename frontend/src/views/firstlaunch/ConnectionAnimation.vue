<template>
  <canvas ref="cv" class="w-full max-w-[460px]" style="height: clamp(240px, 36vh, 400px)"></canvas>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';

const cv = ref(null);
let raf = 0;
let ctx = null;
let W = 0;
let H = 0;
const LOOP = 2800;

function resize() {
  const c = cv.value;
  if (!c) return;
  const dpr = window.devicePixelRatio || 1;
  W = c.clientWidth;
  H = c.clientHeight;
  c.width = Math.round(W * dpr);
  c.height = Math.round(H * dpr);
  ctx = c.getContext('2d');
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function rr(x, y, w, h, r) {
  const rad = Math.min(r, w / 2, h / 2);
  ctx.beginPath();
  ctx.moveTo(x + rad, y);
  ctx.arcTo(x + w, y, x + w, y + h, rad);
  ctx.arcTo(x + w, y + h, x, y + h, rad);
  ctx.arcTo(x, y + h, x, y, rad);
  ctx.arcTo(x, y, x + w, y, rad);
  ctx.closePath();
}

function drawLaptop(cx, cy, size, phase) {
  const bob = size * 0.018 * Math.sin(phase);
  const sw = size * 0.34; // screen width
  const sh = size * 0.24; // screen height
  const gap = size * 0.035; // gap between screen and base line
  const baseW = sw * 0.86; // base line is a bit narrower than the screen
  const totalH = sh + gap;
  const screenY = cy - totalH / 2 + bob;
  const baseY = screenY + sh + gap;
  ctx.lineJoin = 'round';
  ctx.lineCap = 'round';
  ctx.lineWidth = size * 0.013;
  ctx.strokeStyle = 'rgba(255,255,255,0.92)';

  // Lid / screen, generously rounded.
  rr(cx - sw / 2, screenY, sw, sh, size * 0.05);
  ctx.fillStyle = 'rgba(255,255,255,0.16)';
  ctx.fill();
  ctx.stroke();
  ctx.fillStyle = 'rgba(255,255,255,0.5)';
  ctx.beginPath();
  ctx.arc(cx, screenY + size * 0.028, size * 0.009, 0, Math.PI * 2);
  ctx.fill();

  // Base: a simple rounded line below the screen.
  ctx.beginPath();
  ctx.moveTo(cx - baseW / 2, baseY);
  ctx.lineTo(cx + baseW / 2, baseY);
  ctx.stroke();
}

function drawDots(cy, phase) {
  const startX = W * 0.43;
  const gap = W * 0.08;
  for (let i = 0; i < 3; i++) {
    const local = 0.5 + 0.5 * Math.sin(phase - i * 0.8);
    const r = W * (0.018 + 0.008 * local);
    ctx.fillStyle = `rgba(255,255,255,${(96 + 130 * local) / 255})`;
    ctx.beginPath();
    ctx.arc(startX + gap * i, cy, r, 0, Math.PI * 2);
    ctx.fill();
  }
}

function drawQuestion(cx, cy, size, phase) {
  const bob = size * 0.016 * Math.cos(phase + 0.6);
  const r = size * 0.15;
  ctx.beginPath();
  ctx.arc(cx, cy + bob, r, 0, Math.PI * 2);
  ctx.fillStyle = 'rgba(255,255,255,0.18)';
  ctx.fill();
  ctx.lineWidth = size * 0.012;
  ctx.strokeStyle = 'rgba(255,255,255,0.875)';
  ctx.stroke();
  ctx.fillStyle = '#fff';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.font = `bold ${size * 0.2}px sans-serif`;
  ctx.fillText('?', cx, cy + bob);
}

function frame(t) {
  if (!ctx) {
    raf = requestAnimationFrame(frame);
    return;
  }
  const phase = Math.PI * 2 * ((t % LOOP) / LOOP);
  const cy = H * 0.52;
  const size = Math.min(W, H);
  ctx.clearRect(0, 0, W, H);
  drawLaptop(W * 0.25, cy, size, phase);
  drawDots(cy, phase);
  drawQuestion(W * 0.76, cy, size, phase);
  raf = requestAnimationFrame(frame);
}

onMounted(() => {
  resize();
  window.addEventListener('resize', resize);
  raf = requestAnimationFrame(frame);
});
onBeforeUnmount(() => {
  cancelAnimationFrame(raf);
  window.removeEventListener('resize', resize);
});
</script>
