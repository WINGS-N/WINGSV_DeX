<template>
  <header class="flex items-center justify-between px-6 pb-4 pt-7">
    <h1 class="font-sharp text-[26px] font-bold leading-none tracking-tight text-white">WINGS V DeX</h1>
    <div class="flex items-center gap-2">
      <slot name="action" />
      <button
        type="button"
        class="flex h-9 w-9 items-center justify-center overflow-hidden rounded-full bg-wings-surface text-wings-mutedStrong hover:text-wings-text"
        aria-label="WINGS Account"
        @click="openOverlay('account')"
      >
        <img v-if="avatarUrl" :src="avatarUrl" alt="" class="h-full w-full object-cover" />
        <UserRound v-else :size="20" />
      </button>
    </div>
  </header>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { UserRound } from '@lucide/vue';
import { Events } from '@wailsio/runtime';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import { openOverlay } from '@/stores/nav.js';

const avatarUrl = ref('');
let off = null;

onMounted(async () => {
  try {
    avatarUrl.value = (await AccountService.Status()).avatarUrl || '';
  } catch {
    /* panel unreachable: show the generic icon */
  }
  off = Events.On('account:status', (ev) => {
    avatarUrl.value = ev?.data?.avatarUrl || '';
  });
});
onBeforeUnmount(() => {
  if (off) off();
});
</script>
