<template>
  <button
    type="button"
    class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-white/[0.03]"
    @click="$emit('click')"
  >
    <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full" :class="tint">
      <component :is="icon" :size="18" class="text-white" />
    </span>
    <span class="min-w-0 flex-1">
      <span class="block truncate text-[15px] font-semibold text-white">{{ title }}</span>
      <span class="block truncate text-sm text-wings-muted">{{ subtitle }}</span>
    </span>
    <ChevronRight :size="18" class="shrink-0 text-wings-mutedStrong" />
  </button>
</template>

<script setup>
import { ChevronRight } from '@lucide/vue';

defineProps({
  icon: { type: [Object, Function], required: true },
  tint: { type: String, default: 'bg-blue-500' },
  title: { type: String, default: '' },
  subtitle: { type: String, default: '' },
});
defineEmits(['click']);
</script>
