<template>
  <div class="flex h-full flex-col px-6 pb-9 pt-5">
    <div class="flex justify-end">
      <SuwButton class="!px-6 !py-2.5 !text-base" @click="$emit('skip')">Пропустить</SuwButton>
    </div>

    <h1
      class="mt-3 font-sharp text-[2.6rem] font-bold leading-tight text-white [text-shadow:0_2px_16px_rgba(8,29,64,0.25)]"
    >
      Как вы хотите подключаться?
    </h1>
    <p class="mt-3 text-[1.2rem] font-medium leading-snug text-white/90 [text-shadow:0_1px_10px_rgba(8,29,64,0.2)]">
      Давайте вернём вас в привычный Интернет!
    </p>

    <div class="flex flex-1 items-center justify-center">
      <ConnectionAnimation class="[filter:drop-shadow(0_2px_12px_rgba(8,29,64,0.2))]" />
    </div>

    <div class="flex flex-col gap-3">
      <div class="flex gap-3">
        <SuwButton block @click="$emit('choice', 'paste')">
          <template #icon><ClipboardPaste :size="20" /></template>
          Вставить из буфера
        </SuwButton>
        <SuwButton block @click="$emit('choice', 'qr')">
          <template #icon><QrCode :size="20" /></template>
          Сканировать QR код
        </SuwButton>
      </div>
      <div class="flex gap-3">
        <SuwButton block @click="$emit('choice', 'vk_turn')">VK TURN</SuwButton>
        <SuwButton block @click="$emit('choice', 'xray')">Xray (VLESS)</SuwButton>
      </div>
      <SuwButton block @click="$emit('choice', 'auto_search')">Автопоиск</SuwButton>
    </div>
  </div>
</template>

<script setup>
import { ClipboardPaste, QrCode } from 'lucide-vue-next';
import SuwButton from '@/components/onboarding/SuwButton.vue';
import ConnectionAnimation from '@/views/firstlaunch/ConnectionAnimation.vue';

defineEmits(['choice', 'skip']);
</script>
