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
      <h1 class="font-sharp text-[22px] font-bold text-white">Подписки</h1>
      <button
        type="button"
        class="ml-auto rounded-full border border-wings-divider px-3 py-1.5 text-[13px] text-wings-text transition-colors hover:border-wings-accent hover:text-wings-accent"
        :disabled="busy"
        @click="refreshAll"
      >
        Обновить все
      </button>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard kicker="Добавить подписку">
        <div class="flex flex-col gap-2">
          <OneuiInput label="Название" v-model="newTitle" placeholder="Моя подписка" />
          <OneuiInput label="Ссылка" v-model="newUrl" placeholder="https://..." />
          <OneuiInput label="Запасные ссылки" v-model="newMirrors" placeholder="через запятую" />
          <div class="rounded-xl border border-wings-divider bg-white/[0.03] p-3">
            <OneuiInput label="Local HTTP proxy (optional)" v-model="newProxyUrl" placeholder="http://127.0.0.1:2080" />
            <p class="mt-1.5 text-[12px] leading-relaxed text-wings-muted">
              Leave blank for normal fetch. Enter a running local proxy address, such as Throne.
            </p>
          </div>
          <div class="flex justify-end">
            <SamsungButton variant="primary" :busy="busy" :disabled="!newUrl.trim()" @click="add"
              >Добавить</SamsungButton
            >
          </div>
          <p v-if="error" class="text-sm text-wings-danger">{{ error }}</p>
        </div>
      </SamsungCard>

      <button
        type="button"
        class="mt-4 flex w-full items-center gap-3 rounded-2xl bg-wings-surface px-4 py-3.5 text-left hover:bg-white/[0.06]"
        @click="openOverlay('hwid-settings')"
      >
        <span class="flex h-9 w-9 items-center justify-center rounded-full bg-slate-500 text-white">
          <Fingerprint :size="18" />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block text-[17px] text-wings-text">Передача HWID</span>
          <span class="block text-sm text-wings-muted">{{ hwidSummary }}</span>
        </span>
        <ChevronRight :size="18" class="shrink-0 text-wings-mutedStrong" />
      </button>

      <div class="mt-6">
        <div class="mb-2 px-1 text-[12px] font-bold uppercase tracking-[0.14em] text-wings-kicker">Подписки</div>
        <p v-if="subs.length === 0" class="py-8 text-center text-sm text-wings-muted">Подписок пока нет</p>

        <div v-else class="flex flex-col gap-3">
          <SamsungCard v-for="sub in subs" :key="sub.id">
            <div class="flex items-start gap-3">
              <div class="min-w-0 flex-1">
                <div class="truncate text-[17px] text-wings-text">{{ sub.title || 'Без названия' }}</div>
                <div class="mt-0.5 truncate text-sm text-wings-muted">{{ sub.url }}</div>
                <div class="mt-1 text-[13px] text-wings-muted">Обновлено: {{ lastUpdated(sub) }}</div>
                <div v-if="hasQuota(sub)" class="mt-2">
                  <div class="h-1.5 w-full overflow-hidden rounded-full bg-white/10">
                    <div class="h-full rounded-full bg-wings-accent" :style="{ width: usedPct(sub) + '%' }" />
                  </div>
                  <div class="mt-1 flex justify-between text-[12px] text-wings-muted">
                    <span>{{ usedLabel(sub) }}</span>
                    <span v-if="sub.advertisedExpireAt">до {{ expireLabel(sub) }}</span>
                  </div>
                </div>
              </div>
              <div class="flex shrink-0 flex-col items-end gap-2">
                <button
                  type="button"
                  aria-label="Обновить"
                  class="p-1 text-wings-muted hover:text-wings-accent"
                  :disabled="busy || refreshing === sub.id"
                  @click="refresh(sub.id)"
                >
                  <SamsungSpinner v-if="refreshing === sub.id" />
                  <RefreshCw v-else :size="18" />
                </button>
                <button
                  type="button"
                  aria-label="Удалить"
                  class="p-1 text-wings-muted hover:text-wings-danger"
                  @click="remove(sub)"
                >
                  <Trash2 :size="18" />
                </button>
              </div>
            </div>
            <label class="mt-3 flex items-center justify-between gap-3 border-t border-wings-divider pt-3">
              <span class="text-[15px] text-wings-muted">Автообновление</span>
              <OneuiSwitch :model-value="sub.autoUpdate" @update:model-value="(v) => setAutoUpdate(sub.id, v)" />
            </label>
            <div class="mt-3 border-t border-wings-divider pt-3">
              <OneuiInput
                :model-value="proxyDrafts[sub.id] ?? sub.proxyUrl ?? ''"
                label="Local HTTP proxy"
                placeholder="http://127.0.0.1:2080"
                @update:model-value="(value) => (proxyDrafts[sub.id] = value)"
              />
              <div class="mt-2 flex items-center justify-between gap-3">
                <p class="text-[12px] leading-relaxed text-wings-muted">
                  Leave blank for normal fetch; enter a running proxy, such as Throne. Save the proxy, then refresh to
                  use it.
                </p>
                <button
                  type="button"
                  class="shrink-0 rounded-full border border-wings-divider px-3 py-1.5 text-[12px] text-wings-text transition-colors hover:border-wings-accent hover:text-wings-accent"
                  :disabled="
                    savingProxy === sub.id || (proxyDrafts[sub.id] ?? sub.proxyUrl ?? '') === (sub.proxyUrl ?? '')
                  "
                  @click="setProxy(sub)"
                >
                  {{ savingProxy === sub.id ? 'Saving...' : 'Save' }}
                </button>
              </div>
              <p v-if="proxyErrors[sub.id]" class="mt-2 text-sm text-wings-danger">{{ proxyErrors[sub.id] }}</p>
            </div>
          </SamsungCard>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ChevronLeft, ChevronRight, RefreshCw, Trash2, Fingerprint } from '@lucide/vue';
import { Events } from '@wailsio/runtime';
import { SubscriptionService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import SamsungSpinner from '@/components/layout/SamsungSpinner.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import OneuiSwitch from '@/components/controls/OneuiSwitch.vue';
import { closeOverlay, openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';
import { usePinnedScroll } from '@/composables/usePinnedScroll.js';

const rootEl = usePinnedScroll();

const subs = ref([]);
const newTitle = ref('');
const newUrl = ref('');
const newMirrors = ref('');
const newProxyUrl = ref('');
const proxyDrafts = ref({});
const proxyErrors = ref({});
const savingProxy = ref('');
const busy = ref(false);
const refreshing = ref('');
const error = ref('');
const hwidSummary = ref('Автоматическая передача HWID и данных устройства при обновлении подписок');

function apply(list) {
  subs.value = list ?? [];
}

async function load() {
  try {
    apply(await SubscriptionService.List());
  } catch {
    // backend not available (pure-vite preview)
  }
  try {
    const h = await SubscriptionService.HwidSettings();
    hwidSummary.value = !h.enabled
      ? 'Передача HWID выключена'
      : h.manual
        ? 'Ручные значения HWID и данных устройства при обновлении подписок'
        : 'Автоматическая передача HWID и данных устройства при обновлении подписок';
  } catch {
    // backend not available
  }
}

async function add() {
  const url = newUrl.value.trim();
  if (!url) return;
  busy.value = true;
  error.value = '';
  try {
    const mirrors = newMirrors.value
      .split(/[\s,]+/)
      .map((candidate) => candidate.trim())
      .filter(Boolean);
    const res = await SubscriptionService.Add(newTitle.value.trim(), url, mirrors, newProxyUrl.value.trim());
    apply(res.subscriptions);
    if (res.error) {
      error.value = `Подписка сохранена, но узлы не загружены: ${safeError(res.error)}`;
    } else {
      showToast(`Добавлено узлов: ${res.updated}`, { type: 'success' });
      newTitle.value = '';
      newUrl.value = '';
      newMirrors.value = '';
      newProxyUrl.value = '';
    }
  } catch (e) {
    error.value = safeError(e?.message ?? e ?? 'Could not add subscription');
  } finally {
    busy.value = false;
  }
}

async function refresh(id) {
  refreshing.value = id;
  try {
    const res = await SubscriptionService.Refresh(id);
    apply(res.subscriptions);
    if (res.error) showToast(`Не удалось обновить подписку: ${safeError(res.error)}`, { type: 'warn', duration: 7000 });
    else showToast(`Обновлено узлов: ${res.updated}`, { type: 'success' });
  } finally {
    refreshing.value = '';
  }
}

async function refreshAll() {
  busy.value = true;
  try {
    const res = await SubscriptionService.RefreshAll();
    apply(res.subscriptions);
    if (res.error) showToast(`Не все подписки обновлены: ${safeError(res.error)}`, { type: 'warn', duration: 7000 });
    else showToast(`Обновлено узлов: ${res.updated}`, { type: 'success' });
  } finally {
    busy.value = false;
  }
}

async function setAutoUpdate(id, on) {
  apply(await SubscriptionService.SetAutoUpdate(id, on));
}

function safeError(message) {
  let text = String(message ?? 'Не удалось загрузить узлы');
  for (const secret of [
    newUrl.value,
    newProxyUrl.value,
    ...newMirrors.value.split(/[\s,]+/).filter(Boolean),
    ...subs.value.map((sub) => sub.url),
    ...subs.value.map((sub) => sub.proxyUrl),
    ...subs.value.flatMap((sub) => sub.mirrorUrls ?? []),
  ]) {
    if (secret) text = text.split(secret).join('[hidden]');
  }
  return text;
}

async function setProxy(sub) {
  savingProxy.value = sub.id;
  proxyErrors.value[sub.id] = '';
  try {
    apply(await SubscriptionService.SetProxy(sub.id, (proxyDrafts.value[sub.id] ?? sub.proxyUrl ?? '').trim()));
  } catch (e) {
    proxyErrors.value[sub.id] = safeError(e?.message ?? e ?? 'Could not save proxy');
  } finally {
    savingProxy.value = '';
  }
}

async function remove(sub) {
  apply(await SubscriptionService.Remove(sub.id));
  showToast('Подписка удалена', { type: 'success' });
}

function hasQuota(sub) {
  return sub.advertisedTotalBytes > 0 || sub.advertisedDownloadBytes > 0 || sub.advertisedUploadBytes > 0;
}

function used(sub) {
  return (sub.advertisedUploadBytes || 0) + (sub.advertisedDownloadBytes || 0);
}

function usedPct(sub) {
  if (!sub.advertisedTotalBytes) return 0;
  return Math.min(100, Math.round((used(sub) / sub.advertisedTotalBytes) * 100));
}

function usedLabel(sub) {
  const u = fmtBytes(used(sub));
  return sub.advertisedTotalBytes ? `${u} из ${fmtBytes(sub.advertisedTotalBytes)}` : u;
}

function fmtBytes(n) {
  if (!n) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function lastUpdated(sub) {
  if (!sub.lastUpdatedAt) return 'никогда';
  return new Date(sub.lastUpdatedAt * 1000).toLocaleString();
}

function expireLabel(sub) {
  return new Date(sub.advertisedExpireAt * 1000).toLocaleDateString();
}

let off = null;
onMounted(async () => {
  await load();
  off = Events.On('subscriptions:updated', (ev) => apply(ev?.data));
});
onBeforeUnmount(() => {
  if (off) off();
});
</script>
