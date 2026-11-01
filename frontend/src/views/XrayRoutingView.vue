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
      <h1 class="font-sharp text-[22px] font-bold text-white">Маршрутизация</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard title="Базы geoip и geosite" :subtitle="geoSummary" collapsible default-collapsed>
        <template #actions>
          <SamsungPill :variant="geoReady ? 'online' : 'offline'">{{ geoReady ? 'Готовы' : 'Нет баз' }}</SamsungPill>
        </template>
        <div class="geo-sources">
          <OneuiInput
            label="Свой адрес geoip.dat"
            v-model="sources.geoipUrl"
            :placeholder="state.defaultGeoipUrl"
            :spellcheck="false"
            @update:model-value="saveSourcesDebounced"
          />
          <OneuiInput
            label="Свой адрес geosite.dat"
            v-model="sources.geositeUrl"
            :placeholder="state.defaultGeositeUrl"
            :spellcheck="false"
            @update:model-value="saveSourcesDebounced"
          />
        </div>
        <SwitchRow
          class="mt-1"
          title="Обновлять автоматически"
          subtitle="Качать базы при запуске и раз в сутки"
          :model-value="autoUpdate"
          @update:model-value="setAutoUpdate"
        />
        <div class="actions-row mt-3 flex flex-wrap gap-2">
          <SamsungButton variant="secondary" :busy="busy === 'geoip'" @click="download('geoip')">
            Скачать geoip
          </SamsungButton>
          <SamsungButton variant="secondary" :busy="busy === 'geosite'" @click="download('geosite')">
            Скачать geosite
          </SamsungButton>
        </div>
      </SamsungCard>

      <SamsungCard kicker="Правила" class="mt-5">
        <div class="add-rule-row">
          <SamsungButton variant="primary" @click="openEditor(-1)">
            <template #icon><Plus class="button-icon" aria-hidden="true" /></template>
            Добавить правило
          </SamsungButton>
        </div>
        <p v-if="!rules.length" class="py-6 text-center text-sm text-wings-muted">Правил нет.</p>
        <ul v-else class="divide-y divide-wings-divider">
          <li v-for="(rule, index) in rules" :key="rule.id || index">
            <button type="button" class="rule-item" @click="openEditor(index)">
              <span class="rule-item-text">
                <span class="rule-item-title">{{ ruleTitle(rule) }}</span>
                <span class="rule-item-sub">{{ actionLabel(rule.action) }}</span>
              </span>
              <span class="rule-item-trailing">
                <SamsungPill v-if="!rule.enabled">выкл</SamsungPill>
                <ChevronRight :size="20" class="shrink-0 text-wings-muted" />
              </span>
            </button>
          </li>
        </ul>
      </SamsungCard>

      <p v-if="error" class="mt-4 text-sm text-wings-danger">{{ error }}</p>
    </div>

    <SamsungModal v-model="editorOpen" :title="editorIndex < 0 ? 'Новое правило' : 'Правило'">
      <div class="editor-fields">
        <OneuiSelect v-model="draft.match" label="Что сравнивать" :options="matchOptions" />
        <OneuiSelect v-model="draft.action" label="Куда отправлять" :options="actionOptions" />
      </div>
      <OneuiInput class="mt-3" :label="codeLabel(draft.match)" v-model="draft.code" :spellcheck="false" />
      <SwitchRow class="mt-2" title="Включено" v-model="draft.enabled" />
      <template #actions>
        <SamsungButton v-if="editorIndex >= 0" variant="secondary" @click="removeRule">
          <template #icon><Trash2 class="button-icon" aria-hidden="true" /></template>
          Удалить
        </SamsungButton>
        <SamsungButton variant="primary" :disabled="!draft.code.trim()" @click="commitRule">Сохранить</SamsungButton>
      </template>
    </SamsungModal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { ChevronLeft, ChevronRight, Plus, Trash2 } from '@lucide/vue';
import { RoutingService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import SamsungPill from '@/components/layout/SamsungPill.vue';
import SamsungModal from '@/components/layout/SamsungModal.vue';
import SwitchRow from '@/components/layout/SwitchRow.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import OneuiSelect from '@/components/controls/OneuiSelect.vue';
import { closeOverlay } from '@/stores/nav.js';

const matchOptions = [
  { value: 'geoip', label: 'geoip' },
  { value: 'geosite', label: 'geosite' },
  { value: 'domain', label: 'домен' },
  { value: 'ip', label: 'адрес' },
  { value: 'port', label: 'порт' },
  { value: 'network', label: 'сеть' },
  { value: 'protocol', label: 'протокол' },
];
const actionOptions = [
  { value: 'proxy', label: 'через VPN' },
  { value: 'direct', label: 'напрямую' },
  { value: 'block', label: 'блок' },
];

const rootEl = ref(null);
const error = ref('');
const busy = ref('');
const rules = ref([]);
const autoUpdate = ref(false);
const sources = reactive({ geoipUrl: '', geositeUrl: '' });
const state = ref({ defaultGeoipUrl: '', defaultGeositeUrl: '', geo: {} });

const editorOpen = ref(false);
const editorIndex = ref(-1);
const draft = reactive({ id: '', match: 'geoip', code: '', action: 'direct', enabled: true });

const geoReady = computed(() => !!state.value.geo?.geoip?.present && !!state.value.geo?.geosite?.present);
const geoSummary = computed(
  () => `${describe('geoip', state.value.geo?.geoip)}, ${describe('geosite', state.value.geo?.geosite)}`,
);

function describe(name, file) {
  if (!file?.present) return `${name}: нет`;
  return `${name}: ${(file.bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function labelOf(options, value) {
  return options.find((option) => option.value === value)?.label ?? value;
}

function ruleTitle(rule) {
  const match = labelOf(matchOptions, rule.match);
  return `${match}: ${rule.code}`;
}

function actionLabel(action) {
  return labelOf(actionOptions, action);
}

function codeLabel(match) {
  if (match === 'geoip' || match === 'geosite') return 'Код без префикса, например ru';
  if (match === 'protocol') return 'Протоколы через запятую';
  if (match === 'network') return 'tcp, udp';
  return 'Значения через запятую';
}

// Ответ сервера НЕ подменяет редактируемый список: стор выкидывает правила с пустым
// кодом, и только что добавленная строка исчезала бы при первом же касании
function applyGeo(next) {
  if (!next) return;
  state.value = next;
  autoUpdate.value = !!next.autoUpdate;
}

function applyAll(next) {
  if (!next) return;
  applyGeo(next);
  rules.value = Array.isArray(next.rules) ? next.rules.map((rule) => ({ ...rule })) : [];
  sources.geoipUrl = next.sources?.geoipUrl ?? '';
  sources.geositeUrl = next.sources?.geositeUrl ?? '';
}

function openEditor(index) {
  editorIndex.value = index;
  const rule = index < 0 ? { id: '', match: 'geoip', code: '', action: 'direct', enabled: true } : rules.value[index];
  Object.assign(draft, { ...rule });
  editorOpen.value = true;
}

function commitRule() {
  if (!draft.code.trim()) return;
  const next = { ...draft, code: draft.code.trim() };
  if (editorIndex.value < 0) {
    rules.value.push(next);
  } else {
    rules.value[editorIndex.value] = next;
  }
  editorOpen.value = false;
  save();
}

function removeRule() {
  if (editorIndex.value >= 0) {
    rules.value.splice(editorIndex.value, 1);
  }
  editorOpen.value = false;
  save();
}

let sourcesTimer = null;
function saveSourcesDebounced() {
  clearTimeout(sourcesTimer);
  sourcesTimer = setTimeout(saveSources, 600);
}

async function save() {
  try {
    applyGeo(await RoutingService.SetRules(rules.value));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  }
}

async function saveSources() {
  try {
    applyGeo(await RoutingService.SetSources({ geoipUrl: sources.geoipUrl, geositeUrl: sources.geositeUrl }));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  }
}

async function setAutoUpdate(value) {
  try {
    applyGeo(await RoutingService.SetGeoAutoUpdate(value));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  }
}

async function download(kind) {
  busy.value = kind;
  try {
    applyGeo(await RoutingService.DownloadGeo(kind));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  } finally {
    busy.value = '';
  }
}

onMounted(async () => {
  try {
    applyAll(await RoutingService.State());
  } catch {
    // backend not available (pure-vite preview)
  }
});
</script>

<style scoped>
/* Трей узкий, окно на рабочем столе широкое - одна разметка обслуживает оба.
   Брейкпоинт тот же, что в styles.css */
.add-rule-row {
  display: flex;
  justify-content: center;
  padding: 12px 0;
}
.rule-item {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 0;
  text-align: left;
}
.rule-item-text {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.rule-item-title {
  font-size: 17px;
  overflow-wrap: anywhere;
}
.rule-item-sub {
  margin-top: 2px;
  font-size: 14px;
  color: rgba(252, 252, 252, 0.62);
}
.rule-item-trailing {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
}
.geo-sources,
.editor-fields {
  display: grid;
  gap: 4px;
}

@media (min-width: 768px) {
  /* Широкое окно: поля встают парами, чтобы не тянуться лестницей из огрызков */
  .geo-sources,
  .editor-fields {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: 16px;
  }
}
</style>
