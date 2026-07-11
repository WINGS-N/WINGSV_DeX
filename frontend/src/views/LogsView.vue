<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
    <header class="flex shrink-0 items-center gap-2 px-3 pb-3 pt-6">
      <button
        type="button"
        class="rounded-full p-1.5 text-wings-mutedStrong hover:text-wings-text"
        aria-label="Назад"
        @click="closeOverlay"
      >
        <ChevronLeft :size="24" />
      </button>
      <h1 class="font-sharp text-[22px] font-bold text-white">Журнал</h1>
    </header>

    <div class="flex min-h-0 flex-1 flex-col px-4 pb-6">
      <SamsungCard class="flex min-h-0 flex-1 flex-col">
        <div class="flex flex-wrap items-center gap-3">
          <SamsungPill :variant="connected ? 'online' : 'offline'">
            {{ connected ? 'Подключено' : 'Отключено' }}
          </SamsungPill>

          <div class="ml-auto flex items-center gap-1">
            <SamsungIconButton size="small" aria-label="Скопировать лог" title="Скопировать лог" @click="copyText">
              <Copy :size="18" />
            </SamsungIconButton>
            <SamsungIconButton
              size="small"
              variant="danger"
              aria-label="Очистить лог"
              title="Очистить лог"
              @click="requestClear"
            >
              <Trash2 :size="18" />
            </SamsungIconButton>
          </div>
        </div>

        <div class="mt-3 flex items-center gap-2 rounded-full border border-wings-divider bg-wings-surface p-1">
          <button
            v-for="option in channels"
            :key="option.value"
            type="button"
            class="flex-1 rounded-full px-3 py-1.5 text-[13px] transition-colors"
            :class="channel === option.value ? 'bg-wings-accent text-white' : 'text-wings-muted hover:text-wings-text'"
            @click="channel = option.value"
          >
            {{ option.label }}
          </button>
        </div>

        <label class="mt-1 flex cursor-pointer items-center justify-between gap-3 py-3.5">
          <span class="text-[15px] text-wings-muted">Автопрокрутка в конец</span>
          <OneuiSwitch v-model="autoscroll" />
        </label>

        <div class="min-h-0 flex-1 overflow-auto border-t border-wings-divider pt-3">
          <p v-if="!lines.length" class="py-6 text-center text-sm text-wings-muted">Пока нет записей.</p>
          <pre
            v-else
            ref="logEl"
            class="h-full overflow-auto whitespace-pre-wrap break-words font-mono text-[12px] leading-5 text-wings-text"
            >{{ displayText }}</pre>
        </div>
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ChevronLeft, Copy, Trash2 } from 'lucide-vue-next';
import { Clipboard, Events } from '@wailsio/runtime';
import { ConnectionService, LogsService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungIconButton from '@/components/layout/SamsungIconButton.vue';
import SamsungPill from '@/components/layout/SamsungPill.vue';
import OneuiSwitch from '@/components/controls/OneuiSwitch.vue';
import { closeOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';

// Keep the on-screen buffer bounded like the on-disk store; drop the oldest in chunks so
// trimming does not run on every appended line.
const MAX_LINES = 4000;
const TRIM_CHUNK = 500;

const channels = [
  { value: 'runtime', label: 'Runtime' },
  { value: 'proxy', label: 'Proxy' },
];

const channel = ref('runtime');
const autoscroll = ref(true);
const lines = ref([]);
const connected = ref(false);
const logEl = ref(null);
let alive = false;
let requestSeq = 0;

const displayText = computed(() => lines.value.join('\n'));

async function scrollToEnd() {
  if (!autoscroll.value) return;
  await nextTick();
  if (alive) logEl.value?.scrollTo?.(0, logEl.value.scrollHeight);
}

async function loadSnapshot({ notify = false } = {}) {
  const requestId = ++requestSeq;
  const requested = channel.value;
  try {
    const snap = await LogsService.Snapshot(requested);
    if (!alive || requestId !== requestSeq || requested !== channel.value) return;
    lines.value = snap.lines || [];
    await scrollToEnd();
  } catch {
    if (notify && alive && requestId === requestSeq) showToast('Журнал недоступен', { type: 'warn' });
  }
}

// Live push: append each new line for the visible channel without re-fetching the file.
function onLine(ev) {
  const d = ev?.data;
  if (!d || d.channel !== channel.value) return;
  lines.value.push(d.line);
  if (lines.value.length > MAX_LINES) lines.value.splice(0, lines.value.length - (MAX_LINES - TRIM_CHUNK));
  scrollToEnd();
}

async function copyText() {
  try {
    await Clipboard.SetText(displayText.value);
    showToast('Журнал скопирован', { type: 'success' });
  } catch {
    showToast('Не удалось скопировать', { type: 'warn' });
  }
}

async function requestClear() {
  const requested = channel.value;
  try {
    await LogsService.Clear(requested);
    if (requested === channel.value) lines.value = [];
    showToast('Журнал очищен', { type: 'success' });
  } catch {
    showToast('Не удалось очистить журнал', { type: 'warn' });
  }
}

watch(channel, () => {
  lines.value = [];
  loadSnapshot({ notify: true });
});

let offLine = null;
let offState = null;
onMounted(async () => {
  alive = true;
  await loadSnapshot({ notify: true });
  offLine = Events.On('logs:line', onLine);
  offState = Events.On('connection:state', (ev) => {
    connected.value = ev?.data?.status === 'connected';
  });
  try {
    const st = await ConnectionService.State();
    connected.value = st?.status === 'connected';
  } catch {
    // backend not available (pure-vite preview)
  }
});

onBeforeUnmount(() => {
  alive = false;
  requestSeq++;
  if (offLine) offLine();
  if (offState) offState();
});
</script>
