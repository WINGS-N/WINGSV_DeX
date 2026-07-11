<template>
  <div ref="rootEl" class="flex min-h-0 flex-1 flex-col overflow-hidden">
    <header class="flex shrink-0 items-center gap-2 px-3 pb-3 pt-6">
      <button
        type="button"
        class="rounded-full p-1.5 text-wings-mutedStrong hover:text-wings-text"
        aria-label="Назад"
        @click="closeOverlay"
      >
        <ChevronLeft :size="24" />
      </button>
      <h1 class="font-sharp text-[22px] font-bold text-white">Подбор стратегий</h1>
      <div class="ml-auto flex items-center gap-2">
        <SamsungSpinner v-if="running" />
        <SamsungButton variant="secondary" :disabled="running" @click="run">{{
          running ? `${done}/${total}` : 'Запустить'
        }}</SamsungButton>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <p class="mb-3 text-sm text-wings-muted">
        Каждая стратегия проверяется через локальный ByeDPI на доступность тестовых доменов. Нажмите на результат, чтобы
        применить его как команду.
      </p>

      <p v-if="!running && results.length === 0" class="py-8 text-center text-sm text-wings-muted">
        Запустите подбор, чтобы увидеть результаты
      </p>

      <div v-else class="flex flex-col gap-2">
        <button
          v-for="r in sorted"
          :key="r.command"
          type="button"
          class="rounded-2xl border border-wings-divider bg-wings-surface px-4 py-3 text-left transition-colors hover:border-wings-accent"
          @click="apply(r.command)"
        >
          <div class="flex items-center gap-3">
            <span class="shrink-0 rounded-full px-2.5 py-1 text-[13px] font-semibold" :class="ratioClass(r)">
              {{ r.success }}/{{ r.total }}
            </span>
            <span class="flex-1 truncate font-mono text-[12px] text-wings-muted">{{ r.command }}</span>
            <span v-if="r.delayMs >= 0" class="shrink-0 text-[13px] text-wings-muted">{{ r.delayMs }} ms</span>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ChevronLeft } from 'lucide-vue-next';
import { Events } from '@wailsio/runtime';
import { ByeDpiStrategyService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import SamsungSpinner from '@/components/layout/SamsungSpinner.vue';
import { closeOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';
import { usePinnedScroll } from '@/composables/usePinnedScroll.js';

const rootEl = usePinnedScroll();

const results = ref([]);
const running = ref(false);
const total = ref(0);
const done = ref(0);

// Best first: more successful probes, then lower latency.
const sorted = computed(() =>
  [...results.value].sort((a, b) => b.success - a.success || rank(a.delayMs) - rank(b.delayMs)),
);
function rank(d) {
  return d < 0 ? Number.MAX_SAFE_INTEGER : d;
}

function ratioClass(r) {
  if (r.success === 0) return 'bg-red-500/20 text-red-400';
  if (r.success >= r.total) return 'bg-emerald-500/20 text-emerald-400';
  return 'bg-amber-500/20 text-amber-400';
}

async function run() {
  results.value = [];
  done.value = 0;
  try {
    total.value = await ByeDpiStrategyService.Start();
    running.value = total.value > 0;
  } catch {
    showToast('Не удалось запустить подбор', { type: 'warn' });
  }
}

async function apply(command) {
  try {
    await ByeDpiStrategyService.Apply(command);
    showToast('Стратегия применена', { type: 'success' });
  } catch {
    showToast('Не удалось применить', { type: 'warn' });
  }
}

let offResult = null;
let offDone = null;
onMounted(() => {
  offResult = Events.On('byedpi:strategy:result', (ev) => {
    const d = ev?.data;
    if (!d) return;
    results.value.push(d);
    done.value += 1;
  });
  offDone = Events.On('byedpi:strategy:done', () => {
    running.value = false;
  });
});
onBeforeUnmount(() => {
  if (offResult) offResult();
  if (offDone) offDone();
});
</script>
