import { createApp } from 'vue';
import App from './App.vue';
import './stores/theme.js';
import './styles.css';

createApp(App).mount('#app');

// Warm the onboarding sky assets so they are decoded before the first-launch overlay
// mounts - the large gradient otherwise paints blank on the very first (uncached) show.
for (const f of ['suw_intro_bg.webp', 'suw_intro_in.webp']) {
  const img = new Image();
  img.src = `/onboarding/${f}`;
  if (img.decode) img.decode().catch(() => {});
}

// Reveal the app and fade out the pre-mount boot loader once Vue has taken over.
const bootLoader = document.getElementById('boot-loader');
if (bootLoader) {
  bootLoader.classList.add('is-hidden');
  bootLoader.addEventListener('transitionend', () => bootLoader.remove(), { once: true });
}
