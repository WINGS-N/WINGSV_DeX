<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
    <header class="flex shrink-0 items-center gap-2 px-3 pb-3 pt-6">
      <button
        type="button"
        class="rounded-full p-1.5 text-wings-mutedStrong hover:text-wings-text"
        aria-label="Назад"
        @click="openOverlay('subscriptions')"
      >
        <ChevronLeft :size="24" />
      </button>
      <h1 class="font-sharp text-[22px] font-bold text-white">Передача HWID</h1>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-8">
      <SamsungCard class="mb-4 flex flex-col gap-1 py-1">
        <SwitchRow
          v-model="form.enabled"
          title="Передавать данные HWID"
          subtitle="Отправлять x-hwid, x-device-os, x-ver-os и x-device-model при обновлении подписок"
          @update:modelValue="save"
        />
        <div class="mx-4 border-t border-wings-divider"></div>
        <SwitchRow
          v-model="form.manual"
          title="Задать данные HWID вручную"
          subtitle="Использовать введённые вручную значения вместо автоматических"
          @update:modelValue="save"
        />
      </SamsungCard>

      <div class="mb-2 px-1 text-[12px] font-bold uppercase tracking-[0.14em] text-wings-kicker">Данные устройства</div>
      <SamsungCard class="flex flex-col gap-3 py-4">
        <OneuiInput
          v-model="form.hwid"
          label="HWID"
          :placeholder="autoHint(auto.hwid)"
          :disabled="!editable"
          :spellcheck="false"
          @change="save"
        />
        <OneuiInput
          v-model="form.deviceOs"
          label="DEVICE OS"
          :placeholder="autoHint(auto.deviceOs)"
          :disabled="!editable"
          @change="save"
        />
        <OneuiInput
          v-model="form.verOs"
          label="VER OS"
          :placeholder="autoHint(auto.verOs)"
          :disabled="!editable"
          @change="save"
        />
        <OneuiInput
          v-model="form.deviceModel"
          label="DEVICE MODEL"
          :placeholder="autoHint(auto.deviceModel)"
          :disabled="!editable"
          @change="save"
        />
      </SamsungCard>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive } from 'vue';
import { ChevronLeft } from '@lucide/vue';
import { SubscriptionService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SamsungCard from '@/components/layout/SamsungCard.vue';
import SwitchRow from '@/components/layout/SwitchRow.vue';
import OneuiInput from '@/components/controls/OneuiInput.vue';
import { openOverlay } from '@/stores/nav.js';

const form = reactive({ enabled: true, manual: false, hwid: '', deviceOs: '', verOs: '', deviceModel: '' });
const auto = reactive({ hwid: '', deviceOs: '', verOs: '', deviceModel: '' });

const editable = computed(() => form.enabled && form.manual);

function autoHint(value) {
  return value ? `Автоматически: ${value}` : 'Не задано';
}

onMounted(async () => {
  const v = await SubscriptionService.HwidSettings();
  form.enabled = v.enabled;
  form.manual = v.manual;
  form.hwid = v.hwid;
  form.deviceOs = v.deviceOs;
  form.verOs = v.verOs;
  form.deviceModel = v.deviceModel;
  auto.hwid = v.autoHwid;
  auto.deviceOs = v.autoDeviceOs;
  auto.verOs = v.autoVerOs;
  auto.deviceModel = v.autoDeviceModel;
});

async function save() {
  await SubscriptionService.SetHwidSettings(
    form.enabled,
    form.manual,
    form.hwid,
    form.deviceOs,
    form.verOs,
    form.deviceModel,
  );
}
</script>
