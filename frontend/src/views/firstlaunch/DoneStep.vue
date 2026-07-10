<template>
  <div class="flex h-full flex-col items-center px-6 pb-9 pt-5 text-center">
    <div class="flex flex-1 flex-col items-center justify-center">
      <span
        class="flex h-24 w-24 items-center justify-center rounded-full bg-white/90 [box-shadow:0_8px_28px_rgba(8,29,64,0.25)]"
      >
        <Check :size="56" class="text-[#173a5a]" :stroke-width="3" />
      </span>
      <h1
        class="mt-8 font-sharp text-[3rem] font-bold leading-none text-white [text-shadow:0_2px_16px_rgba(8,29,64,0.25)]"
      >
        Готово!
      </h1>
      <p
        class="mt-4 max-w-md text-[1.2rem] font-medium leading-relaxed text-white/90 [text-shadow:0_1px_10px_rgba(8,29,64,0.2)]"
      >
        Приятного пользования свободным Интернетом!
      </p>
    </div>

    <SuwButton block class="max-w-md" @click="$emit('finish')">Начать использовать</SuwButton>
  </div>
</template>

<script setup>
import { Check } from 'lucide-vue-next';
import SuwButton from '@/components/onboarding/SuwButton.vue';

defineEmits(['finish']);
</script>
