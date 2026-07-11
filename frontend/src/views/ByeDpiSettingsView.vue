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
            subtitle="Задавать аргументы ciadpi строкой вместо редактора шагов"
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

        <SamsungCard v-else kicker="Стратегия (шаги)" class="mt-5">
          <p class="mb-2 text-sm text-wings-muted">
            Шаги собираются в команду ciadpi по порядку. Каждый шаг - опция и её значение.
          </p>
          <div class="flex flex-col gap-2">
            <div v-for="(step, i) in form.desyncSteps" :key="i" class="flex items-center gap-2">
              <select
                v-model="step.flag"
                class="w-[46%] shrink-0 rounded-xl border border-wings-divider bg-wings-input px-2 py-2 text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
                @change="save"
              >
                <option v-for="opt in flagOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
              <input
                v-model="step.value"
                spellcheck="false"
                :placeholder="valuePlaceholder(step.flag)"
                class="min-w-0 flex-1 rounded-xl border border-wings-divider bg-wings-input px-3 py-2 font-mono text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
                @input="saveDebounced"
              />
              <button
                type="button"
                aria-label="Удалить шаг"
                class="shrink-0 p-1 text-wings-muted hover:text-wings-danger"
                @click="removeStep(i)"
              >
                <Trash2 :size="18" />
              </button>
            </div>
          </div>

          <div class="mt-3 flex items-center gap-3">
            <SamsungButton variant="secondary" @click="addStep">Добавить шаг</SamsungButton>
            <button type="button" class="text-[13px] text-wings-accent" @click="resetSteps">
              Сбросить к рекомендуемой
            </button>
          </div>

          <div class="mt-4 rounded-xl border border-wings-divider bg-wings-input px-3 py-2">
            <p class="mb-1 text-[12px] uppercase tracking-[0.12em] text-wings-kicker">Итоговая команда</p>
            <p class="break-words font-mono text-[12px] text-wings-muted">{{ preview }}</p>
          </div>
        </SamsungCard>

        <SamsungCard kicker="Подбор стратегий" class="mt-5">
          <div class="divide-y divide-wings-divider">
            <button
              type="button"
              class="flex w-full items-center justify-between py-3.5 text-left"
              @click="openOverlay('byedpi-strategies')"
            >
              <span class="flex flex-col">
                <span class="text-[17px]">Открыть подбор</span>
                <span class="mt-0.5 text-sm text-wings-muted">Протестировать стратегии и применить лучшую</span>
              </span>
              <ChevronRight :size="20" class="shrink-0 text-wings-muted" />
            </button>
            <OneuiInput label="SNI для теста" v-model="form.proxyTestSni" @update:model-value="saveDebounced" />
            <OneuiInput
              label="Таймаут, с"
              type="number"
              v-model="form.proxyTestTimeoutSeconds"
              @update:model-value="saveDebounced"
            />
            <OneuiInput
              label="Параллельно"
              type="number"
              v-model="form.proxyTestConcurrencyLimit"
              @update:model-value="saveDebounced"
            />
            <div class="py-3">
              <p class="mb-1 text-sm text-wings-muted">Тестовые домены (по одному на строку)</p>
              <textarea
                v-model="form.proxyTestTargets"
                rows="3"
                spellcheck="false"
                class="w-full resize-none rounded-xl border border-wings-divider bg-wings-input px-3 py-2 font-mono text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
                @input="saveDebounced"
              ></textarea>
            </div>
            <SwitchRow
              title="Свой список стратегий"
              v-model="form.proxyTestUseCustomStrategies"
              @update:model-value="save"
            />
            <div v-if="form.proxyTestUseCustomStrategies" class="py-3">
              <p class="mb-1 text-sm text-wings-muted">Стратегии (по одной на строку)</p>
              <textarea
                v-model="form.proxyTestCustomStrategies"
                rows="4"
                spellcheck="false"
                class="w-full resize-none rounded-xl border border-wings-divider bg-wings-input px-3 py-2 font-mono text-[13px] text-wings-text outline-none focus:border-wings-inputLine"
                @input="saveDebounced"
              ></textarea>
            </div>
          </div>
        </SamsungCard>
      </template>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive } from 'vue';
import { ChevronLeft, ChevronRight, Trash2 } from 'lucide-vue-next';
import { ProfilesService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import SwitchRow from '@/components/layout/SwitchRow.vue';
import { closeOverlay, openOverlay } from '@/stores/nav.js';
import { usePinnedScroll } from '@/composables/usePinnedScroll.js';
import { WARN, warnConfirm, isPasswordTooSimple } from '@/stores/proxyWarnings.js';

const rootEl = usePinnedScroll();

const DEFAULT_COMMAND =
  '-o1 -d1 -a1 -At,r,s -s1 -d1 -s5+s -s10+s -s15+s -s20+s -r1+s -S -a1 -As -s1 -d1 -s5+s -s10+s -s15+s -s20+s -S -a1';

// Every meaningful ciadpi option, exposed as a step flag with a friendly label. The
// connection flags (-i/-p/-I) and SOCKS auth are managed above, and the daemon/pidfile/
// transparent/debug flags are irrelevant to a foreground child, so they are omitted here;
// anything still needed can be typed in full in command mode.
const flagOptions = [
  { value: '-s', label: 'Split (-s)' },
  { value: '-d', label: 'Disorder (-d)' },
  { value: '-o', label: 'OOB (-o)' },
  { value: '-q', label: 'Disorder OOB (-q)' },
  { value: '-f', label: 'Fake (-f)' },
  { value: '-r', label: 'TLS-запись (-r)' },
  { value: '-A', label: 'Auto (-A)' },
  { value: '-L', label: 'Auto-режим (-L)' },
  { value: '-a', label: 'UDP-фейки (-a)' },
  { value: '-S', label: 'MD5-подпись (-S)' },
  { value: '-e', label: 'OOB-данные (-e)' },
  { value: '-t', label: 'TTL фейка (-t)' },
  { value: '-n', label: 'SNI фейка (-n)' },
  { value: '-O', label: 'Смещение фейка (-O)' },
  { value: '-l', label: 'Фейк-данные (-l)' },
  { value: '-Q', label: 'Мод фейк-TLS (-Q)' },
  { value: '-M', label: 'Мод HTTP (-M)' },
  { value: '-m', label: 'Минор TLS (-m)' },
  { value: '-K', label: 'Протоколы (-K)' },
  { value: '-H', label: 'Хосты (-H)' },
  { value: '-j', label: 'IP-список (-j)' },
  { value: '-V', label: 'Диапазон портов (-V)' },
  { value: '-R', label: 'Раунды (-R)' },
  { value: '-T', label: 'Таймаут (-T)' },
  { value: '-y', label: 'Кэш-файл (-y)' },
  { value: '-u', label: 'TTL кэша (-u)' },
  { value: '-N', label: 'Без резолва (-N)' },
  { value: '-U', label: 'Без UDP (-U)' },
  { value: '-g', label: 'TTL соединений (-g)' },
  { value: '-c', label: 'Лимит соединений (-c)' },
  { value: '-b', label: 'Буфер (-b)' },
  { value: '-F', label: 'TCP Fast Open (-F)' },
  { value: '-Y', label: 'Drop SACK (-Y)' },
];
// Flags that take no value.
const NO_VALUE = new Set(['-S', '-N', '-U', '-F', '-Y']);

const form = reactive({
  enabled: false,
  useCommandSettings: false,
  command: DEFAULT_COMMAND,
  proxyIp: '127.0.0.1',
  proxyPort: 1080,
  authEnabled: true,
  username: '',
  password: '',
  desyncSteps: [],
  proxyTestConcurrencyLimit: 20,
  proxyTestTimeoutSeconds: 5,
  proxyTestSni: 'max.ru',
  proxyTestUseCustomStrategies: false,
  proxyTestCustomStrategies: '',
  proxyTestTargets: '',
});

const preview = computed(() =>
  form.desyncSteps
    .filter((s) => s.flag)
    .map((s) => (NO_VALUE.has(s.flag) || !s.value ? s.flag : s.flag + s.value))
    .join(' '),
);

function valuePlaceholder(flag) {
  if (NO_VALUE.has(flag)) return 'без значения';
  if (flag === '-A' || flag === '-a') return 't,r,s';
  return '1';
}

let loaded = false;
let lastPw = '';

onMounted(async () => {
  try {
    Object.assign(form, await ProfilesService.ByeDPISettings());
    form.desyncSteps = (form.desyncSteps ?? []).map((s) => ({ flag: s.flag, value: s.value }));
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
    const payload = {
      ...form,
      proxyPort: Number(form.proxyPort) || 0,
      proxyTestConcurrencyLimit: Number(form.proxyTestConcurrencyLimit) || 0,
      proxyTestTimeoutSeconds: Number(form.proxyTestTimeoutSeconds) || 0,
      desyncSteps: form.desyncSteps
        .filter((s) => s.flag)
        .map((s) => ({ flag: s.flag, value: NO_VALUE.has(s.flag) ? '' : s.value })),
    };
    const saved = await ProfilesService.SetByeDPISettings(payload);
    if (saved) {
      const steps = (saved.desyncSteps ?? []).map((s) => ({ flag: s.flag, value: s.value }));
      Object.assign(form, saved);
      form.desyncSteps = steps;
    }
  } catch {
    // ignore persist failure
  }
}

let debounce = null;
function saveDebounced() {
  if (debounce) clearTimeout(debounce);
  debounce = setTimeout(save, 400);
}

function addStep() {
  form.desyncSteps.push({ flag: '-s', value: '1' });
  save();
}
function removeStep(i) {
  form.desyncSteps.splice(i, 1);
  save();
}
function resetSteps() {
  form.desyncSteps = DEFAULT_COMMAND.split(' ').map((tok) => ({ flag: tok.slice(0, 2), value: tok.slice(2) }));
  save();
}
function resetCommand() {
  form.command = DEFAULT_COMMAND;
  save();
}

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
