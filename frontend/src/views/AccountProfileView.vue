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
      <h1 class="font-sharp text-[22px] font-bold text-white">Информация профиля</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard class="mb-4 py-4">
        <div class="text-sm text-wings-muted">Логин</div>
        <div class="mt-1 text-[17px] text-white">{{ username }}</div>
      </SamsungCard>

      <div class="mb-2 px-1 text-sm font-semibold text-wings-muted">Пароль</div>
      <SamsungCard class="flex flex-col gap-3 py-4">
        <OneuiInput v-model="current" placeholder="Текущий пароль" type="password" />
        <OneuiInput v-model="next" placeholder="Новый пароль" type="password" />
        <OneuiInput v-model="repeat" placeholder="Повторите новый" type="password" />
        <SamsungButton :busy="busy" @click="save">Сменить пароль</SamsungButton>
        <p v-if="error" class="text-sm text-red-400">{{ error }}</p>
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { ChevronLeft } from '@lucide/vue';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import { openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';

const username = ref('');
const current = ref('');
const next = ref('');
const repeat = ref('');
const error = ref('');
const busy = ref(false);

onMounted(async () => {
  username.value = (await AccountService.Status()).username;
});

async function save() {
  error.value = '';
  if (next.value !== repeat.value) {
    error.value = 'Новый пароль и повтор не совпадают';
    return;
  }
  busy.value = true;
  try {
    await AccountService.ChangePassword(current.value, next.value);
    current.value = '';
    next.value = '';
    repeat.value = '';
    showToast('Пароль изменён', { type: 'success' });
  } catch (e) {
    error.value = String(e?.message ?? e ?? 'Не удалось сменить пароль');
  } finally {
    busy.value = false;
  }
}
</script>
