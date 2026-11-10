<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
    <header class="flex shrink-0 items-center gap-2 px-3 pb-3 pt-6">
      <button
        type="button"
        class="rounded-full p-1.5 text-wings-mutedStrong hover:text-wings-text"
        aria-label="Назад"
        @click="openOverlay('account')"
      >
        <ChevronLeft :size="24" />
      </button>
      <h1 class="font-sharp text-[22px] font-bold text-white">Аватар</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard class="flex flex-col items-center gap-5 py-6">
        <img v-if="status.avatarUrl" :src="status.avatarUrl" alt="" class="h-28 w-28 rounded-full object-cover" />
        <div v-else class="flex h-28 w-28 items-center justify-center rounded-full bg-wings-surface">
          <UserRound :size="48" class="text-wings-mutedStrong" />
        </div>
        <div class="flex w-full flex-col gap-3">
          <SamsungButton :busy="busy" @click="change">Сменить аватар</SamsungButton>
          <SamsungButton v-if="status.avatarUrl" variant="secondary" :busy="busy" @click="remove">
            Убрать картинку
          </SamsungButton>
        </div>
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ChevronLeft, UserRound } from '@lucide/vue';
import { Events } from '@wailsio/runtime';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import { openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';

const status = ref({ signedIn: false, username: '', panel: '', avatarUrl: '' });
const busy = ref(false);

async function refresh() {
  status.value = await AccountService.Status();
}

async function change() {
  busy.value = true;
  try {
    await AccountService.ChangeAvatar();
    showToast('Аватар обновлён', { type: 'success' });
  } catch (e) {
    showToast(String(e?.message ?? e ?? 'Что-то пошло не так'), { type: 'warn' });
  } finally {
    busy.value = false;
  }
}

async function remove() {
  busy.value = true;
  try {
    await AccountService.RemoveAvatar();
  } catch (e) {
    showToast(String(e?.message ?? e ?? 'Что-то пошло не так'), { type: 'warn' });
  } finally {
    busy.value = false;
  }
}

let off = null;
onMounted(() => {
  refresh();
  off = Events.On('account:status', (ev) => {
    if (ev?.data) status.value = ev.data;
  });
});
onBeforeUnmount(() => {
  if (off) off();
});
</script>
