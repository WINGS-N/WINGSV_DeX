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
      <h1 class="font-sharp text-[22px] font-bold text-white">WINGS Account</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <!-- Signed out: approve this device in a browser, or sign in with a password. -->
      <div v-if="!status.signedIn" class="flex flex-col gap-4">
        <SamsungCard class="flex flex-col gap-3 py-5 text-center">
          <p class="text-[15px] text-wings-muted">Войдите, чтобы получить бесплатные серверы федерации</p>
          <SamsungButton :busy="busy" @click="browserLogin">Войти через браузер</SamsungButton>
        </SamsungCard>

        <SamsungCard class="flex flex-col gap-3 py-5">
          <OneuiInput v-model="login" label="Логин" :spellcheck="false" />
          <OneuiInput v-model="password" label="Пароль" type="password" />
          <SamsungButton variant="secondary" :busy="busy" @click="passwordLogin">Войти</SamsungButton>
          <p v-if="error" class="text-sm text-red-400">{{ error }}</p>
        </SamsungCard>
      </div>

      <!-- Signed in: the account dashboard. -->
      <div v-else class="flex flex-col gap-4">
        <div class="flex items-center justify-between">
          <span class="font-sharp text-[34px] font-bold lowercase text-white">{{ status.username }}</span>
          <img v-if="status.avatarUrl" :src="status.avatarUrl" alt="" class="h-16 w-16 rounded-full object-cover" />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div class="rounded-2xl bg-wings-surface px-4 py-4">
            <div class="text-sm text-wings-muted">серверов выдано</div>
            <div class="mt-1 text-[22px] font-bold text-white">{{ access.servers.length }}</div>
          </div>
          <div class="rounded-2xl bg-wings-surface px-4 py-4">
            <div class="text-sm text-wings-muted">Доступно</div>
            <div class="mt-1 text-[22px] font-bold text-white">{{ used }} / {{ limit }}</div>
          </div>
          <div class="rounded-2xl bg-wings-surface px-4 py-4">
            <div class="flex items-center gap-1.5 text-sm text-emerald-400"><ArrowDown :size="15" /> Downlink</div>
            <div class="mt-1 text-[22px] font-bold text-white">{{ downlink }}</div>
          </div>
          <div class="rounded-2xl bg-wings-surface px-4 py-4">
            <div class="flex items-center gap-1.5 text-sm text-sky-400"><ArrowUp :size="15" /> Uplink</div>
            <div class="mt-1 text-[22px] font-bold text-white">{{ uplink }}</div>
          </div>
        </div>

        <div class="rounded-full bg-emerald-600 py-3 text-center text-[15px] font-bold text-white">
          Доверие {{ access.trustConfidence }} / 100
        </div>
        <div class="rounded-full bg-emerald-600 py-3 text-center text-[15px] font-bold text-white">
          {{ used }} / {{ limit }}
        </div>

        <SamsungCard class="flex flex-col gap-0 py-1">
          <AccountRow
            :icon="User"
            tint="bg-blue-500"
            title="Информация профиля"
            subtitle="Логин и пароль"
            @click="openOverlay('account-profile')"
          />
          <div class="mx-14 border-t border-wings-divider"></div>
          <AccountRow
            :icon="CalendarCheck"
            tint="bg-indigo-500"
            title="Подписка"
            subtitle="Ссылка, по которой приложение обновляется само"
            @click="openOverlay('account-subscription')"
          />
          <div class="mx-14 border-t border-wings-divider"></div>
          <AccountRow
            :icon="Users"
            tint="bg-emerald-500"
            title="Приглашения"
            subtitle="Ваш код для приглашения"
            @click="openOverlay('account-invites')"
          />
        </SamsungCard>

        <SamsungCard class="flex flex-col gap-0 py-1">
          <AccountRow
            :icon="Wifi"
            tint="bg-blue-500"
            title="Мои серверы"
            subtitle="Что вы отдали в федерацию"
            @click="openOverlay('account-donor')"
          />
          <div class="mx-14 border-t border-wings-divider"></div>
          <AccountRow
            :icon="Smartphone"
            tint="bg-violet-500"
            title="Управление клиентами"
            subtitle="Свои клиенты и их устройства"
            @click="openClients"
          />
        </SamsungCard>

        <SamsungButton variant="secondary" :busy="busy" @click="logout">
          <span class="flex items-center justify-center gap-2"><LogOut :size="18" /> Выйти</span>
        </SamsungButton>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ChevronLeft, User, CalendarCheck, Users, Wifi, Smartphone, LogOut, ArrowDown, ArrowUp } from '@lucide/vue';
import { Events } from '@wailsio/runtime';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import AccountRow from '@/components/account/AccountRow.vue';
import { closeOverlay, openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';
import { usePinnedScroll } from '@/composables/usePinnedScroll.js';

const rootEl = usePinnedScroll();

const status = ref({ signedIn: false, username: '', panel: '', avatarUrl: '' });
const access = ref({ servers: [], usedBytes: 0, limitBytes: 0, uplinkBps: 0, downlinkBps: 0, trustConfidence: 0 });
const login = ref('');
const password = ref('');
const error = ref('');
const busy = ref(false);

const used = computed(() => fmtBytes(access.value.usedBytes));
const limit = computed(() => (access.value.limitBytes > 0 ? fmtBytes(access.value.limitBytes) : '∞'));
const downlink = computed(() => fmtSpeed(access.value.downlinkBps));
const uplink = computed(() => fmtSpeed(access.value.uplinkBps));

function fmtBytes(n) {
  if (!n || n < 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(i > 0 && v < 100 ? 1 : 0)} ${u[i]}`;
}
function fmtSpeed(bps) {
  return `${(bps / 1e6).toFixed(1)} MB/s`;
}

async function refresh() {
  status.value = await AccountService.Status();
  if (status.value.signedIn) {
    try {
      access.value = await AccountService.Access();
    } catch {
      /* keep prior figures */
    }
  }
}

async function browserLogin() {
  error.value = '';
  busy.value = true;
  try {
    await AccountService.BrowserLogin();
    showToast('Подтвердите вход в браузере', { type: 'info' });
  } catch {
    showToast('Не удалось открыть браузер', { type: 'warn' });
  } finally {
    busy.value = false;
  }
}

async function passwordLogin() {
  error.value = '';
  busy.value = true;
  try {
    await AccountService.LoginPassword(login.value.trim(), password.value);
    password.value = '';
    await refresh();
  } catch (e) {
    error.value = String(e?.message ?? e ?? 'Не удалось войти');
  } finally {
    busy.value = false;
  }
}

async function openClients() {
  try {
    await AccountService.OpenPanelSection('/admin/clients');
  } catch {
    showToast('Не найдено приложение для открытия ссылки', { type: 'warn' });
  }
}

async function logout() {
  busy.value = true;
  try {
    await AccountService.Logout();
    await refresh();
  } finally {
    busy.value = false;
  }
}

let off = null;
onMounted(() => {
  refresh();
  off = Events.On('account:status', (ev) => {
    if (ev?.data) status.value = ev.data;
    if (status.value.signedIn) refresh();
  });
});
onBeforeUnmount(() => {
  if (off) off();
});
</script>
