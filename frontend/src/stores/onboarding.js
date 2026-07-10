import { ref } from 'vue';
import { MusicService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';

// First-launch onboarding visibility. `music` is set only by the easter egg (5x tap on
// the About app icon), which replays the SUW intro with the Over the Horizon track.
export const showOnboarding = ref(false);
export const onboardingMusic = ref(false);

export function openOnboarding({ music = false } = {}) {
  onboardingMusic.value = music;
  showOnboarding.value = true;
  // Played natively (Go): the WebKitGTK build here cannot decode in-page <audio>.
  if (music) MusicService.Play().catch(() => {});
}

export function closeOnboarding() {
  showOnboarding.value = false;
  if (onboardingMusic.value) MusicService.Stop().catch(() => {});
  onboardingMusic.value = false;
}
