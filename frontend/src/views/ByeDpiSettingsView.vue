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
      <h1 class="font-sharp text-[22px] font-bold text-white">ByeDPI</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard kicker="ByeDPI">
        <div class="divide-y divide-wings-divider">
          <SwitchRow
            title="Включить ByeDPI"
            subtitle="Пускать трафик Xray через локальный обход DPI"
            v-model="form.enabled"
            @update:model-value="save"
          />
          <SwitchRow
            v-if="form.enabled"
            title="Ручная команда"
            subtitle="Задавать аргументы ciadpi строкой вместо редактора"
            v-model="form.useCommandSettings"
            @update:model-value="save"
          />
        </div>
      </SamsungCard>

      <template v-if="form.enabled">
        <SamsungCard kicker="Локальный прокси" class="mt-5">
          <div class="divide-y divide-wings-divider">
            <OneuiInput label="Адрес" v-model="form.proxyIp" @update:model-value="saveDebounced" />
            <OneuiInput label="Порт" type="number" v-model="form.proxyPort" @update:model-value="saveDebounced" />
            <SwitchRow
              title="Пароль"
              subtitle="Защитить локальный прокси логином и паролем"
              :model-value="form.authEnabled"
              @update:model-value="onAuthToggle"
            />
            <template v-if="form.authEnabled">
              <OneuiInput label="Логин" v-model="form.username" @update:model-value="saveDebounced" />
              <OneuiInput label="Пароль" :model-value="form.password" @update:model-value="onPassword" />
            </template>
          </div>
        </SamsungCard>

        <SamsungCard v-if="form.useCommandSettings" kicker="Команда" class="mt-5">
          <textarea
            v-model="form.command"
            rows="4"
            spellcheck="false"
            class="w-full resize-none rounded-xl border border-wings-divider bg-wings-input px-3 py-2 font-mono text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
            @input="saveDebounced"
          ></textarea>
          <button type="button" class="mt-2 text-[13px] text-wings-accent" @click="resetCommand">
            Сбросить к рекомендуемой
          </button>
        </SamsungCard>

        <template v-else>
          <SamsungCard kicker="Обход" class="mt-5">
            <div class="divide-y divide-wings-divider">
              <OneuiSelect
                label="Метод десинхронизации"
                v-model="form.desyncMethod"
                :options="desyncOptions"
                @update:model-value="save"
              />
              <template v-if="form.desyncMethod !== 'none'">
                <OneuiInput
                  label="Позиция разбиения"
                  type="number"
                  v-model="form.splitPosition"
                  @update:model-value="saveDebounced"
                />
                <SwitchRow title="Разбивать по хосту" v-model="form.splitAtHost" @update:model-value="save" />
              </template>
              <template v-if="form.desyncMethod === 'fake'">
                <OneuiInput
                  label="TTL фейка"
                  type="number"
                  v-model="form.fakeTtl"
                  @update:model-value="saveDebounced"
                />
                <OneuiInput label="SNI фейка" v-model="form.fakeSni" @update:model-value="saveDebounced" />
                <OneuiInput
                  label="Смещение фейка"
                  type="number"
                  v-model="form.fakeOffset"
                  @update:model-value="saveDebounced"
                />
              </template>
              <OneuiInput
                v-if="form.desyncMethod === 'oob' || form.desyncMethod === 'disoob'"
                label="OOB-данные"
                v-model="form.oobData"
                @update:model-value="saveDebounced"
              />
            </div>
          </SamsungCard>

          <SamsungCard kicker="Протоколы" class="mt-5">
            <div class="divide-y divide-wings-divider">
              <SwitchRow title="Обход HTTPS (TLS)" v-model="form.desyncHttps" @update:model-value="save" />
              <SwitchRow title="Обход HTTP" v-model="form.desyncHttp" @update:model-value="save" />
              <SwitchRow title="Обход UDP (QUIC)" v-model="form.desyncUdp" @update:model-value="save" />
              <OneuiInput
                v-if="form.desyncUdp"
                label="Фейков UDP"
                type="number"
                v-model="form.udpFakeCount"
                @update:model-value="saveDebounced"
              />
            </div>
          </SamsungCard>

          <SamsungCard kicker="TLS-запись" class="mt-5">
            <div class="divide-y divide-wings-divider">
              <SwitchRow title="Разбивать TLS-запись" v-model="form.tlsRecordSplit" @update:model-value="save" />
              <template v-if="form.tlsRecordSplit">
                <OneuiInput
                  label="Позиция"
                  type="number"
                  v-model="form.tlsRecordSplitPosition"
                  @update:model-value="saveDebounced"
                />
                <SwitchRow title="По SNI" v-model="form.tlsRecordSplitAtSni" @update:model-value="save" />
              </template>
            </div>
          </SamsungCard>

          <SamsungCard kicker="Модификация HTTP" class="mt-5">
            <div class="divide-y divide-wings-divider">
              <SwitchRow title="Смешанный регистр Host" v-model="form.hostMixedCase" @update:model-value="save" />
              <SwitchRow title="Смешанный регистр домена" v-model="form.domainMixedCase" @update:model-value="save" />
              <SwitchRow
                title="Убирать пробелы в заголовке"
                v-model="form.hostRemoveSpaces"
                @update:model-value="save"
              />
            </div>
          </SamsungCard>

          <SamsungCard kicker="Хосты" class="mt-5">
            <div class="divide-y divide-wings-divider">
              <OneuiSelect
                label="Режим списка"
                v-model="form.hostsMode"
                :options="hostsModeOptions"
                @update:model-value="save"
              />
              <div v-if="form.hostsMode === 'blacklist'" class="py-3">
                <p class="mb-1 text-sm text-wings-muted">Чёрный список (по одному хосту на строку)</p>
                <textarea
                  v-model="form.hostsBlacklist"
                  rows="3"
                  spellcheck="false"
                  class="w-full resize-none rounded-xl border border-wings-divider bg-wings-input px-3 py-2 font-mono text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
                  @input="saveDebounced"
                ></textarea>
              </div>
              <div v-else-if="form.hostsMode === 'whitelist'" class="py-3">
                <p class="mb-1 text-sm text-wings-muted">Белый список (по одному хосту на строку)</p>
                <textarea
                  v-model="form.hostsWhitelist"
                  rows="3"
                  spellcheck="false"
                  class="w-full resize-none rounded-xl border border-wings-divider bg-wings-input px-3 py-2 font-mono text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
                  @input="saveDebounced"
                ></textarea>
              </div>
            </div>
          </SamsungCard>

          <SamsungCard kicker="Дополнительно" class="mt-5">
            <div class="divide-y divide-wings-divider">
              <OneuiInput
                label="TTL по умолчанию (0 - выкл)"
                type="number"
                v-model="form.defaultTtl"
                @update:model-value="saveDebounced"
              />
              <SwitchRow title="Запрет резолва доменов" v-model="form.noDomain" @update:model-value="save" />
              <OneuiInput
                label="Лимит соединений"
                type="number"
                v-model="form.maxConnections"
                @update:model-value="saveDebounced"
              />
              <OneuiInput
                label="Размер буфера"
                type="number"
                v-model="form.bufferSize"
                @update:model-value="saveDebounced"
              />
              <SwitchRow title="TCP Fast Open" v-model="form.tcpFastOpen" @update:model-value="save" />
              <SwitchRow title="Отбрасывать SACK" v-model="form.dropSack" @update:model-value="save" />
            </div>
          </SamsungCard>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive } from 'vue';
import { ChevronLeft } from 'lucide-vue-next';
import { ProfilesService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import OneuiSelect from '@/components/controls/OneuiSelect.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import SwitchRow from '@/components/layout/SwitchRow.vue';
import { closeOverlay } from '@/stores/nav.js';
import { usePinnedScroll } from '@/composables/usePinnedScroll.js';
import { WARN, warnConfirm, isPasswordTooSimple } from '@/stores/proxyWarnings.js';

const rootEl = usePinnedScroll();

const DEFAULT_COMMAND =
  '-o1 -d1 -a1 -At,r,s -s1 -d1 -s5+s -s10+s -s15+s -s20+s -r1+s -S -a1 -As -s1 -d1 -s5+s -s10+s -s15+s -s20+s -S -a1';

const desyncOptions = [
  { value: 'none', label: 'Нет' },
  { value: 'split', label: 'Split' },
  { value: 'disorder', label: 'Disorder' },
  { value: 'fake', label: 'Fake' },
  { value: 'oob', label: 'OOB' },
  { value: 'disoob', label: 'Disorder OOB' },
];
const hostsModeOptions = [
  { value: 'disable', label: 'Выключено' },
  { value: 'blacklist', label: 'Чёрный список' },
  { value: 'whitelist', label: 'Белый список' },
];

const form = reactive({
  enabled: false,
  useCommandSettings: false,
  command: DEFAULT_COMMAND,
  proxyIp: '127.0.0.1',
  proxyPort: 1080,
  authEnabled: true,
  username: '',
  password: '',
  maxConnections: 512,
  bufferSize: 16384,
  defaultTtl: 0,
  noDomain: false,
  tcpFastOpen: false,
  dropSack: false,
  desyncHttp: true,
  desyncHttps: true,
  desyncUdp: true,
  desyncMethod: 'oob',
  splitPosition: 1,
  splitAtHost: false,
  fakeTtl: 8,
  fakeSni: 'www.iana.org',
  fakeOffset: 0,
  oobData: 'a',
  udpFakeCount: 1,
  hostMixedCase: false,
  domainMixedCase: false,
  hostRemoveSpaces: false,
  tlsRecordSplit: true,
  tlsRecordSplitPosition: 1,
  tlsRecordSplitAtSni: true,
  hostsMode: 'disable',
  hostsBlacklist: '',
  hostsWhitelist: '',
  proxyTestDelaySeconds: 1,
  proxyTestRequests: 1,
  proxyTestConcurrencyLimit: 20,
  proxyTestTimeoutSeconds: 5,
  proxyTestSni: 'max.ru',
  proxyTestUseCustomStrategies: false,
  proxyTestCustomStrategies: '',
});

const INT_FIELDS = [
  'proxyPort',
  'maxConnections',
  'bufferSize',
  'defaultTtl',
  'splitPosition',
  'fakeTtl',
  'fakeOffset',
  'udpFakeCount',
  'tlsRecordSplitPosition',
  'proxyTestDelaySeconds',
  'proxyTestRequests',
  'proxyTestConcurrencyLimit',
  'proxyTestTimeoutSeconds',
];

let loaded = false;
let lastPw = '';

onMounted(async () => {
  try {
    Object.assign(form, await ProfilesService.ByeDPISettings());
    lastPw = form.password;
  } catch {
    // backend not available (pure-vite preview)
  } finally {
    loaded = true;
  }
});

async function save() {
  if (!loaded) return;
  try {
    const payload = { ...form };
    INT_FIELDS.forEach((f) => {
      payload[f] = Number(form[f]) || 0;
    });
    const saved = await ProfilesService.SetByeDPISettings(payload);
    if (saved) Object.assign(form, saved);
  } catch {
    // ignore persist failure
  }
}

let debounce = null;
function saveDebounced() {
  if (debounce) clearTimeout(debounce);
  debounce = setTimeout(save, 400);
}

function resetCommand() {
  form.command = DEFAULT_COMMAND;
  save();
}

// ByeDPI is a local SOCKS proxy, so disabling its auth gets the same security warning as
// the xray SOCKS inbound.
async function onAuthToggle(v) {
  if (!v && !(await warnConfirm(WARN.socksAuthDisable))) return;
  form.authEnabled = v;
  save();
}

let pwTimer = null;
function onPassword(v) {
  form.password = v;
  if (pwTimer) clearTimeout(pwTimer);
  pwTimer = setTimeout(async () => {
    if (form.authEnabled && v && isPasswordTooSimple(form.username, v)) {
      if (!(await warnConfirm(WARN.socksWeak))) {
        form.password = lastPw;
        return;
      }
    }
    lastPw = v;
    save();
  }, 500);
}

onBeforeUnmount(() => {
  if (debounce) clearTimeout(debounce);
  if (pwTimer) clearTimeout(pwTimer);
});
</script>
