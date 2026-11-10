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
      <h1 class="font-sharp text-[22px] font-bold text-white">Приглашения</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <p class="mb-3 px-1 text-[15px] text-wings-muted">Кто отсканирует код, тот войдёт в федерацию под вами.</p>

      <SamsungCard class="mb-4 flex flex-col gap-3 py-4">
        <div
          v-for="inv in invites.list"
          :key="inv.token"
          class="flex flex-col gap-2 rounded-xl bg-wings-surface px-3 py-3"
        >
          <div class="flex items-center justify-between">
            <span class="font-mono text-[15px] font-bold text-white">{{ inv.token }}</span>
            <span class="text-sm text-wings-muted">{{ uses(inv) }}</span>
          </div>
          <button
            type="button"
            class="flex items-center gap-1.5 self-start text-sm text-wings-accent"
            @click="share(inv)"
          >
            <Copy :size="14" /> Поделиться ссылкой
          </button>
        </div>
        <SamsungButton v-if="invites.mayInvite" :busy="busy" @click="create">Создать код</SamsungButton>
        <p v-else-if="invites.reason" class="text-sm text-wings-muted">{{ invites.reason }}</p>
      </SamsungCard>

      <div class="mb-2 px-1 text-sm font-semibold text-wings-muted">Вас пригласили</div>
      <SamsungCard class="flex flex-col gap-3 py-4">
        <OneuiInput v-model="redeemCode" placeholder="Код приглашения" :spellcheck="false" />
        <SamsungButton variant="secondary" :busy="redeeming" @click="redeem">Применить код</SamsungButton>
        <p v-if="redeemError" class="text-sm text-red-400">{{ redeemError }}</p>
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { ChevronLeft, Copy } from '@lucide/vue';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import { openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';

const invites = ref({ mayInvite: false, reason: '', list: [] });
const redeemCode = ref('');
const redeemError = ref('');
const busy = ref(false);
const redeeming = ref(false);

function uses(inv) {
  return inv.maxUses > 0 ? `${inv.useCount} / ${inv.maxUses}` : `${inv.useCount} / ∞`;
}

async function load() {
  try {
    invites.value = await AccountService.Invites();
  } catch {
    /* keep prior */
  }
}

async function create() {
  busy.value = true;
  try {
    await AccountService.CreateInvite();
    await load();
  } catch (e) {
    showToast(String(e?.message ?? e ?? 'Что-то пошло не так'), { type: 'warn' });
  } finally {
    busy.value = false;
  }
}

async function share(inv) {
  try {
    await navigator.clipboard.writeText(inv.link || inv.token);
    showToast('Ссылка скопирована', { type: 'success' });
  } catch {
    showToast('Не удалось скопировать', { type: 'warn' });
  }
}

async function redeem() {
  redeemError.value = '';
  const code = redeemCode.value.trim();
  if (!code) return;
  redeeming.value = true;
  try {
    await AccountService.RedeemInvite(code);
    redeemCode.value = '';
    showToast('Приглашение принято', { type: 'success' });
    await load();
  } catch (e) {
    redeemError.value = String(e?.message ?? e ?? 'Не удалось применить код');
  } finally {
    redeeming.value = false;
  }
}

onMounted(load);
</script>
