import { ref } from 'vue';
import { AboutService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';

// Whether a newer release is available, refreshed periodically so the Settings tab and the
// About entry can show a badge without the About screen being open.
export const updateAvailable = ref(false);

let timer = null;

export async function refreshUpdateBadge() {
  try {
    const r = await AboutService.CheckUpdate();
    updateAvailable.value = r?.status === 'available';
  } catch {
    // backend not available (pure-vite preview) or offline -> leave as-is
  }
}

export function startUpdatePolling() {
  if (timer) return;
  refreshUpdateBadge();
  timer = setInterval(refreshUpdateBadge, 6 * 60 * 60 * 1000);
}
