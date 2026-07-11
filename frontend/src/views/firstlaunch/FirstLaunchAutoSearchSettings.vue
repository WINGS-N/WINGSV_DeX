<template>
  <div class="flex h-full flex-col px-6 pb-9 pt-5">
    <h1
      class="mt-10 font-sharp text-[2.4rem] font-bold leading-tight text-white [text-shadow:0_2px_16px_rgba(8,29,64,0.25)]"
    >
      Автопоиск
    </h1>
    <p class="mt-3 text-[1.15rem] font-medium leading-snug text-white/90 [text-shadow:0_1px_10px_rgba(8,29,64,0.2)]">
      Настройте параметры проверки профилей перед запуском
    </p>

    <div class="mt-6 flex flex-1 flex-col gap-3 overflow-y-auto">
      <div v-for="f in fields" :key="f.key" class="rounded-3xl border border-white/15 bg-white/10 px-5 py-3">
        <label class="text-[15px] text-white/80">{{ f.label }}</label>
        <input
          v-model="settings[f.key]"
          type="number"
          class="w-full bg-transparent text-[22px] font-semibold text-white outline-none"
        />
      </div>
    </div>

    <SuwButton block class="mt-5" @click="next">Далее</SuwButton>
  </div>
</template>

<script setup>
import { onMounted, reactive } from 'vue';
import { AutoSearchService } from '@bindings/github.com/WINGS-N/wingsv-dex/internal/services';
import SuwButton from '@/components/onboarding/SuwButton.vue';

const emit = defineEmits(['next']);

const fields = [
  { key: 'targetCount', label: 'Сколько искать' },
  { key: 'tcpingTimeoutMs', label: 'Таймаут TCPing' },
  { key: 'downloadSizeMb', label: 'Размер тестового файла' },
  { key: 'downloadTimeoutSeconds', label: 'Таймаут скачивания' },
  { key: 'downloadAttempts', label: 'Количество прогонов' },
];
const settings = reactive({
  targetCount: 5,
  tcpingTimeoutMs: 1000,
  downloadSizeMb: 5,
  downloadTimeoutSeconds: 20,
  downloadAttempts: 2,
  useBuiltInSubscription: true,
});

onMounted(async () => {
  try {
    Object.assign(settings, await AutoSearchService.Settings());
  } catch {
    // backend not available (pure-vite preview)
  }
});

async function next() {
  try {
    const payload = { ...settings };
    fields.forEach((f) => (payload[f.key] = Number(settings[f.key]) || 0));
    await AutoSearchService.SetSettings(payload);
  } catch {
    // ignore
  }
  emit('next');
}
</script>
