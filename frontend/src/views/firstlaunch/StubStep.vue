<template>
  <div class="flex h-full flex-col px-6 pb-9 pt-5">
    <div class="flex justify-end">
      <SuwButton v-if="showSkip" class="!px-6 !py-2.5 !text-base" @click="$emit('skip')">Пропустить</SuwButton>
    </div>

    <h1
      class="mt-8 font-sharp text-[2.4rem] font-bold leading-tight text-white [text-shadow:0_2px_16px_rgba(8,29,64,0.25)]"
    >
      {{ title }}
    </h1>
    <p class="mt-3 text-[1.15rem] font-medium leading-snug text-white/90 [text-shadow:0_1px_10px_rgba(8,29,64,0.2)]">
      {{ subtitle }}
    </p>

    <div class="flex flex-1 items-center justify-center px-2">
      <component :is="icon" :size="88" class="text-white/85 [filter:drop-shadow(0_2px_14px_rgba(8,29,64,0.25))]" />
    </div>

    <SuwButton block @click="$emit('next')">{{ nextLabel }}</SuwButton>
  </div>
</template>

<script setup>
import SuwButton from '@/components/onboarding/SuwButton.vue';

// Scaffold for steps with no working desktop equivalent yet.
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  icon: { type: [Object, Function], required: true },
  nextLabel: { type: String, default: 'Далее' },
  showSkip: { type: Boolean, default: false },
});
defineEmits(['next', 'skip']);
</script>
