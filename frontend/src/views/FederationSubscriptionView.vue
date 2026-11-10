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
      <h1 class="font-sharp text-[22px] font-bold text-white">Подписка</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard class="flex flex-col gap-3 py-4">
        <p class="text-[15px] text-wings-muted">
          {{ present ? 'Подписка добавлена в приложение' : 'Серверы пока не выданы' }}
        </p>
        <div
          v-if="present"
          class="break-all rounded-xl bg-wings-surface px-3 py-3 font-mono text-[13px] text-wings-text"
        >
          {{ url }}
        </div>
        <SamsungButton v-if="present" variant="secondary" @click="copy">Скопировать ссылку</SamsungButton>
      </SamsungCard>

      <div v-if="present" class="mt-4 flex gap-3 rounded-2xl border border-amber-500/30 bg-amber-500/10 px-4 py-3.5">
        <TriangleAlert :size="20" class="mt-0.5 shrink-0 text-amber-400" />
        <p class="text-[13px] leading-relaxed text-amber-200/90">
          Ни в коем случае не копируйте эту подписку в другие программы, помимо WINGS V и WINGS V DeX. Не используйте
          эту подписку на другом WINGS Account.
        </p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { ChevronLeft, TriangleAlert } from '@lucide/vue';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import { openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';

const url = ref('');
const present = ref(false);

onMounted(async () => {
  try {
    const access = await AccountService.Access();
    url.value = access.subscriptionUrl || '';
    present.value = !!url.value;
  } catch {
    present.value = false;
  }
});

async function copy() {
  try {
    await navigator.clipboard.writeText(url.value);
    showToast('Ссылка скопирована', { type: 'success' });
  } catch {
    showToast('Не удалось скопировать', { type: 'warn' });
  }
}
</script>
