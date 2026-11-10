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
      <h1 class="font-sharp text-[22px] font-bold text-white">Мои серверы</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <div class="mb-4 grid grid-cols-2 gap-3">
        <div class="rounded-2xl bg-wings-surface px-4 py-4">
          <div class="text-sm text-wings-muted">серверов отдано</div>
          <div class="mt-1 text-[22px] font-bold text-white">{{ donor.nodes }}</div>
        </div>
        <div class="rounded-2xl bg-wings-surface px-4 py-4">
          <div class="text-sm text-wings-muted">пронесено</div>
          <div class="mt-1 text-[22px] font-bold text-white">{{ fmtBytes(donor.usedBytes) }}</div>
          <div v-if="donor.probeBytes > 0" class="mt-0.5 text-xs text-wings-muted">
            из них {{ fmtBytes(donor.probeBytes) }} проверок
          </div>
        </div>
      </div>

      <!-- Connect a new server: pick an optional monthly budget, get the command to run on it. -->
      <SamsungCard class="mb-4 flex flex-col gap-3 py-4">
        <div class="text-sm font-semibold text-wings-muted">Месячный лимит, GB</div>
        <OneuiInput
          v-model="budgetGb"
          type="number"
          placeholder="Оставьте пустым, чтобы взять общий потолок площадки"
        />
        <SamsungButton :busy="enrolling" @click="connect">Подключить сервер</SamsungButton>
        <template v-if="command">
          <div class="text-sm text-wings-muted">Выполните на сервере, который отдаёте</div>
          <div class="break-all rounded-xl bg-wings-surface px-3 py-3 font-mono text-[13px] text-wings-text">
            {{ command }}
          </div>
          <button
            type="button"
            class="flex items-center gap-1.5 self-start text-sm text-wings-accent"
            @click="copyCommand"
          >
            <Copy :size="14" /> Скопировать ссылку
          </button>
        </template>
      </SamsungCard>

      <div class="mb-2 px-1 text-sm font-semibold text-wings-muted">Серверы</div>
      <SamsungCard v-if="donor.list && donor.list.length" class="flex flex-col gap-3 py-4">
        <div
          v-for="node in donor.list"
          :key="node.nodeId"
          class="flex flex-col gap-2 rounded-xl bg-wings-surface px-3 py-3"
        >
          <div class="flex items-center justify-between">
            <span class="text-[15px] font-bold text-white">{{ node.hostname || node.nodeId }}</span>
            <span :class="node.online ? 'text-emerald-400' : 'text-wings-muted'" class="text-sm">
              {{ node.online ? 'онлайн' : 'оффлайн' }}
            </span>
          </div>
          <div class="text-sm text-wings-muted">
            {{ fmtBytes(node.usedBytes) }} из {{ fmtBytes(node.declaredBudgetBytes) }}
          </div>
          <template v-if="editing === node.nodeId">
            <OneuiInput v-model="editBudgetGb" type="number" placeholder="Месячный лимит, GB" />
            <p v-if="editError" class="text-sm text-red-400">{{ editError }}</p>
            <div class="flex gap-2">
              <SamsungButton class="flex-1" :busy="savingBudget" @click="saveBudget(node)">Сохранить</SamsungButton>
              <SamsungButton class="flex-1" variant="secondary" @click="editing = ''">Отмена</SamsungButton>
            </div>
          </template>
          <button
            v-else
            type="button"
            class="flex items-center gap-1.5 self-start text-sm text-wings-accent"
            @click="startEdit(node)"
          >
            <Pencil :size="14" /> Изменить месячный лимит
          </button>
        </div>
      </SamsungCard>
      <SamsungCard v-else class="py-5 text-center">
        <p class="text-[15px] text-wings-muted">Вы пока не отдали ни одного сервера</p>
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { ChevronLeft, Copy, Pencil } from '@lucide/vue';
import { AccountService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SamsungButton from '@/components/layout/SamsungButton.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import { openOverlay } from '@/stores/nav.js';
import { showToast } from '@/stores/toast.js';

const donor = ref({ nodes: 0, usedBytes: 0, probeBytes: 0, declaredBudgetBytes: 0, list: [] });
const budgetGb = ref('');
const command = ref('');
const enrolling = ref(false);
const editing = ref('');
const editBudgetGb = ref('');
const editError = ref('');
const savingBudget = ref(false);

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

async function load() {
  try {
    donor.value = await AccountService.Donor();
  } catch {
    /* keep prior */
  }
}

async function connect() {
  enrolling.value = true;
  command.value = '';
  try {
    const gb = parseInt(budgetGb.value, 10);
    command.value = await AccountService.EnrollNode(Number.isFinite(gb) && gb > 0 ? gb : 0);
  } catch (e) {
    showToast(String(e?.message ?? e ?? 'Что-то пошло не так'), { type: 'warn' });
  } finally {
    enrolling.value = false;
  }
}

async function copyCommand() {
  try {
    await navigator.clipboard.writeText(command.value);
    showToast('Ссылка скопирована', { type: 'success' });
  } catch {
    showToast('Не удалось скопировать', { type: 'warn' });
  }
}

function startEdit(node) {
  editing.value = node.nodeId;
  editError.value = '';
  editBudgetGb.value = String(Math.round(node.declaredBudgetBytes / (1024 * 1024 * 1024)));
}

async function saveBudget(node) {
  editError.value = '';
  const gb = parseInt(editBudgetGb.value, 10);
  if (!Number.isFinite(gb) || gb <= 0) {
    editError.value = 'Нулевой лимит не бывает: с ним ноду нельзя выдать никому';
    return;
  }
  savingBudget.value = true;
  try {
    await AccountService.SetNodeBudget(node.nodeId, gb);
    editing.value = '';
    await load();
  } catch (e) {
    editError.value = String(e?.message ?? e ?? 'Что-то пошло не так');
  } finally {
    savingBudget.value = false;
  }
}

onMounted(load);
</script>
