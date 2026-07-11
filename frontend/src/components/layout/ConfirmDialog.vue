<template>
  <!-- Security warning: a full-screen screen (not a dialog) matching the app. -->
  <Teleport to="body">
    <div
      v-if="confirmState.open && confirmState.icon === 'warning'"
      class="fixed inset-0 z-[60] flex flex-col bg-wings-page"
    >
      <header class="flex shrink-0 items-center gap-2 px-3 pb-3 pt-6">
        <button
          type="button"
          class="rounded-full p-1.5 text-wings-accent hover:opacity-80"
          aria-label="Назад"
          @click="settleConfirm(false)"
        >
          <ChevronLeft :size="24" />
        </button>
        <h1 class="font-sharp text-[22px] font-bold text-wings-accent">{{ confirmState.title }}</h1>
      </header>

      <div class="flex flex-1 flex-col items-center justify-center px-8 text-center">
        <AlertTriangle :size="76" class="text-amber-400" />
        <p class="mt-7 max-w-[22rem] text-[19px] font-bold leading-snug text-white">{{ confirmState.message }}</p>
      </div>

      <div class="shrink-0 px-6 pb-8 text-center">
        <p class="text-[19px] text-wings-muted">Вы уверены, что хотите продолжить?</p>
        <p class="mt-2 h-5 text-[15px] text-wings-mutedStrong">
          {{ remaining > 0 ? `Продолжить можно через ${remaining} с` : '' }}
        </p>
        <div class="mt-5 flex gap-3">
          <button
            type="button"
            class="flex-1 rounded-full bg-white/10 py-4 text-[17px] font-semibold text-wings-text transition-colors hover:bg-white/15"
            @click="settleConfirm(false)"
          >
            {{ confirmState.cancelText || 'Отмена' }}
          </button>
          <button
            type="button"
            class="flex-1 rounded-full py-4 text-[17px] font-semibold transition-colors"
            :class="
              remaining > 0 ? 'cursor-default bg-white/70 text-neutral-500' : 'bg-white text-red-600 hover:bg-white/90'
            "
            :disabled="remaining > 0"
            @click="settleConfirm(true)"
          >
            {{ confirmState.confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>

  <!-- Regular confirmation: a modal dialog. -->
  <SamsungModal
    v-if="confirmState.icon !== 'warning'"
    :model-value="confirmState.open"
    :title="confirmState.title"
    @update:model-value="settleConfirm(false)"
  >
    <p class="body-copy confirm-message">{{ confirmState.message }}</p>
    <template #actions>
      <SamsungButton :variant="confirmState.danger ? 'danger' : 'primary'" @click="settleConfirm(true)">
        {{ confirmState.confirmText }}
      </SamsungButton>
      <SamsungButton v-if="confirmState.cancelText" variant="secondary" @click="settleConfirm(false)">
        {{ confirmState.cancelText }}
      </SamsungButton>
    </template>
  </SamsungModal>
</template>

<script setup>
import { onBeforeUnmount, ref, watch } from 'vue';
import { AlertTriangle, ChevronLeft } from 'lucide-vue-next';
import SamsungModal from '@/components/layout/SamsungModal.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import { confirmState, settleConfirm } from '@/stores/confirm.js';

// Counts down while a countdown dialog is open, keeping the confirm button disabled (grey)
// until it reaches zero, at which point it turns its characteristic red.
const remaining = ref(0);
let timer = null;

function stop() {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
}

watch(
  () => confirmState.open,
  (open) => {
    stop();
    if (open && confirmState.countdown > 0) {
      remaining.value = confirmState.countdown;
      timer = setInterval(() => {
        remaining.value -= 1;
        if (remaining.value <= 0) stop();
      }, 1000);
    } else {
      remaining.value = 0;
    }
  },
);

onBeforeUnmount(stop);
</script>

<style scoped>
.confirm-message {
  white-space: pre-line;
}
</style>
