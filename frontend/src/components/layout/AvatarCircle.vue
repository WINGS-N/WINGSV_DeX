<template>
  <span
    class="relative inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full"
    :style="{ width: `${size}px`, height: `${size}px`, backgroundColor: color }"
  >
    <span
      v-if="!resolved || failed"
      class="font-bold leading-none text-white"
      :style="{ fontSize: `${Math.round(size * 0.36)}px` }"
    >
      {{ initials }}
    </span>
    <img
      v-else
      :src="resolved"
      alt=""
      loading="lazy"
      class="absolute inset-0 h-full w-full"
      :class="contain ? 'object-contain p-[18%]' : 'object-cover'"
      @error="failed = true"
    />
  </span>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue';
import { AvatarService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';

const props = defineProps({
  // Either a direct src (local asset / data URL) or a GitHub `username` resolved through
  // the Go disk cache. On failure it falls back to the initials disc.
  src: { type: String, default: '' },
  username: { type: String, default: '' },
  initials: { type: String, default: '' },
  color: { type: String, default: '#3a3a4a' },
  size: { type: Number, default: 44 },
  // Fit the image inside the circle with padding instead of cropping it to fill.
  contain: { type: Boolean, default: false },
});

const failed = ref(false);
const resolved = ref('');

async function load() {
  failed.value = false;
  if (props.username) {
    resolved.value = '';
    try {
      resolved.value = (await AvatarService.Get(props.username)) || '';
    } catch {
      resolved.value = '';
    }
    if (!resolved.value) failed.value = true;
    return;
  }
  resolved.value = props.src;
}

onMounted(load);
watch(() => [props.src, props.username], load);
</script>
