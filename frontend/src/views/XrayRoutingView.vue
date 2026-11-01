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
      <SamsungCard kicker="Базы">
        <template #actions>
          <SamsungPill v-if="geoReady">Готовы</SamsungPill>
          <SamsungPill v-else>Нет баз</SamsungPill>
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
        <div class="actions-row mt-3 flex flex-wrap gap-2">
          <SamsungButton variant="secondary" :busy="busy === 'geoip'" @click="download('geoip')">
            Скачать geoip
          </SamsungButton>
          <SamsungButton variant="secondary" :busy="busy === 'geosite'" @click="download('geosite')">
            Скачать geosite
          </SamsungButton>
        </div>
        <p class="pb-2 pt-3 text-sm text-wings-muted">{{ geoSummary }}</p>
      </SamsungCard>

      <SamsungCard kicker="Правила" class="mt-5">
        <p v-if="!rules.length" class="py-6 text-center text-sm text-wings-muted">Правил нет.</p>
        <ul v-else class="divide-y divide-wings-divider">
          <li v-for="(rule, index) in rules" :key="rule.id || index" class="rule-row py-3">
            <div class="rule-selects">
              <OneuiSelect v-model="rule.match" :options="matchOptions" @update:model-value="save" />
              <OneuiSelect v-model="rule.action" :options="actionOptions" @update:model-value="save" />
              <div class="rule-trailing">
                <OneuiSwitch v-model="rule.enabled" @update:model-value="save" />
                <button
                  type="button"
                  class="rounded-full p-1.5 text-wings-mutedStrong hover:text-wings-danger"
                  aria-label="Удалить правило"
                  @click="removeRule(index)"
                >
                  <Trash2 :size="18" />
                </button>
              </div>
            </div>
            <OneuiInput
              class="rule-code"
              :label="codeLabel(rule.match)"
              v-model="rule.code"
              :spellcheck="false"
              @update:model-value="saveDebounced"
            />
          </li>
        </ul>
        <div class="actions-row mt-3">
          <SamsungButton variant="primary" @click="addRule">
            <template #icon><Plus class="button-icon" aria-hidden="true" /></template>
            Добавить правило
          </SamsungButton>
        </div>
      </SamsungCard>

      <p v-if="error" class="mt-4 text-sm text-wings-danger">{{ error }}</p>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { ChevronLeft, Plus, Trash2 } from '@lucide/vue';
import { RoutingService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import SamsungPill from '@/components/layout/SamsungPill.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import OneuiSelect from '@/components/controls/OneuiSelect.vue';
import OneuiSwitch from '@/components/controls/OneuiSwitch.vue';
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
const sources = reactive({ geoipUrl: '', geositeUrl: '' });
const state = ref({ defaultGeoipUrl: '', defaultGeositeUrl: '', geo: {} });

const geoReady = computed(() => !!state.value.geo?.geoip?.present && !!state.value.geo?.geosite?.present);

const geoSummary = computed(() => {
  const geo = state.value.geo || {};
  return [describe('geoip', geo.geoip), describe('geosite', geo.geosite)].join(', ');
});

function describe(name, file) {
  if (!file?.present) return `${name}: нет`;
  return `${name}: ${(file.bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function codeLabel(match) {
  if (match === 'geoip' || match === 'geosite') return 'Код без префикса, например ru';
  if (match === 'protocol') return 'Протоколы через запятую';
  if (match === 'network') return 'tcp, udp';
  return 'Значения через запятую';
}

function applyState(next) {
  if (!next) return;
  state.value = next;
  rules.value = Array.isArray(next.rules) ? next.rules.map((rule) => ({ ...rule })) : [];
  sources.geoipUrl = next.sources?.geoipUrl ?? '';
  sources.geositeUrl = next.sources?.geositeUrl ?? '';
}

function addRule() {
  rules.value.push({ id: '', match: 'geoip', code: '', action: 'direct', enabled: true });
}

function removeRule(index) {
  rules.value.splice(index, 1);
  save();
}

let saveTimer = null;
function saveDebounced() {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(save, 600);
}

let sourcesTimer = null;
function saveSourcesDebounced() {
  clearTimeout(sourcesTimer);
  sourcesTimer = setTimeout(saveSources, 600);
}

async function save() {
  try {
    applyState(await RoutingService.SetRules(rules.value));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  }
}

async function saveSources() {
  try {
    applyState(await RoutingService.SetSources({ geoipUrl: sources.geoipUrl, geositeUrl: sources.geositeUrl }));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  }
}

async function download(kind) {
  busy.value = kind;
  try {
    applyState(await RoutingService.DownloadGeo(kind));
    error.value = '';
  } catch (err) {
    error.value = String(err?.message ?? err);
  } finally {
    busy.value = '';
  }
}

onMounted(async () => {
  try {
    applyState(await RoutingService.State());
  } catch {
    // backend not available (pure-vite preview)
  }
});
</script>

<style scoped>
/* Трей узкий, окно на рабочем столе широкое - одна разметка обслуживает оба.
   Брейкпоинт тот же, что в styles.css */
.rule-selects {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.rule-trailing {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}
.rule-code {
  margin-top: 8px;
}
.geo-sources {
  display: grid;
  gap: 4px;
}

@media (min-width: 768px) {
  /* Широкое окно: селекторы и код встают в одну строку, список перестаёт быть
     лестницей из коротких полей */
  .rule-row {
    display: grid;
    grid-template-columns: minmax(0, auto) minmax(0, 1fr);
    align-items: center;
    column-gap: 16px;
  }
  .rule-code {
    margin-top: 0;
  }
  .geo-sources {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: 16px;
  }
}
</style>
