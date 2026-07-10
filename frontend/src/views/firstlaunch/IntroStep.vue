<template>
  <div class="flex h-full flex-col items-center px-6 pb-9 pt-5 text-center">
    <span class="rounded-full bg-white/90 px-5 py-2 text-sm font-bold text-[#173a5a]">Добро пожаловать</span>

    <h1
      class="mt-24 font-sharp text-[3.25rem] font-bold leading-none text-white [text-shadow:0_2px_16px_rgba(8,29,64,0.25)]"
    >
      WINGS V DeX
    </h1>

    <div class="flex-1"></div>

    <p
      class="max-w-md text-[1.15rem] font-medium leading-relaxed text-white/90 [text-shadow:0_1px_10px_rgba(8,29,64,0.2)]"
    >
      WINGS V DeX - десктопный клиент VK TURN с WireGuard, для борьбы с Российской Интернет-цензурой, включая белые
      списки
    </p>

    <SuwButton block class="mt-10 max-w-md" @click="$emit('next')">Начать</SuwButton>
  </div>
</template>

<script setup>
import SuwButton from '@/components/onboarding/SuwButton.vue';

defineEmits(['next']);
</script>
