<template>
  <div class="flex h-full flex-col px-6 pb-9 pt-5">
    <h1
      class="mt-14 font-sharp text-[2.4rem] font-bold leading-tight text-white [text-shadow:0_2px_16px_rgba(8,29,64,0.25)]"
    >
      Настройки VK TURN
    </h1>
    <p class="mt-3 text-[1.15rem] font-medium leading-snug text-white/90 [text-shadow:0_1px_10px_rgba(8,29,64,0.2)]">
      Импортируйте готовую ссылку WINGSV из буфера обмена - остальные параметры можно настроить позже во вкладке
      «Профили»
    </p>

    <div class="flex flex-1 items-center justify-center">
      <KeyRound :size="96" class="text-white/85 [filter:drop-shadow(0_2px_14px_rgba(8,29,64,0.25))]" />
    </div>

    <div class="flex flex-col gap-3">
      <SuwButton block @click="$emit('import')">
        <template #icon><ClipboardPaste :size="20" /></template>
        Импортировать wingsv://
      </SuwButton>
      <SuwButton block @click="$emit('next')">Далее</SuwButton>
    </div>
  </div>
</template>

<script setup>
import { ClipboardPaste, KeyRound } from 'lucide-vue-next';
import SuwButton from '@/components/onboarding/SuwButton.vue';

defineEmits(['import', 'next']);
</script>
