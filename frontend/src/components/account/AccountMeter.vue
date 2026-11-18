<template>
  <div
    role="meter"
    aria-valuemin="0"
    aria-valuemax="100"
    :aria-valuenow="value"
    class="relative overflow-hidden rounded-full bg-wings-surface py-3 text-center text-[15px] font-bold text-white"
  >
    <div aria-hidden="true" class="absolute inset-y-0 left-0" :class="tint" :style="{ width: value + '%' }" />
    <span class="relative"><slot /></span>
  </div>
</template>

<script setup>
defineProps({
  value: { type: Number, required: true },
  tint: { type: String, required: true },
});
</script>
