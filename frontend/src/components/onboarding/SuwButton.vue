<template>
  <button
    type="button"
    class="suw-pill"
    :class="[`suw-pill-${variant}`, block ? 'w-full' : '']"
    @click="$emit('click', $event)"
  >
    <span v-if="$slots.icon" class="suw-pill-icon"><slot name="icon" /></span>
    <span class="suw-pill-label"><slot /></span>
  </button>
</template>

<script setup>
// Fill is low-alpha white so the sky gradient shows through - warm near the bottom, cool
// near the top.
defineProps({
  variant: { type: String, default: 'glass' }, // glass | badge
  block: { type: Boolean, default: false },
});
defineEmits(['click']);
</script>

<style scoped>
.suw-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  border-radius: 28px;
  padding: 0.95rem 1.4rem;
  font-weight: 700;
  font-size: 1.05rem;
  line-height: 1.15;
  text-align: center;
  color: #173a5a;
  -webkit-backdrop-filter: blur(10px);
  backdrop-filter: blur(10px);
  transition:
    background-color 0.18s ease,
    transform 0.12s ease;
}
.suw-pill:active {
  transform: scale(0.97);
}
.suw-pill-glass {
  background: rgba(255, 255, 255, 0.4);
}
.suw-pill-glass:hover {
  background: rgba(255, 255, 255, 0.6);
}
.suw-pill-badge {
  padding: 0.55rem 1.2rem;
  font-size: 0.95rem;
  background: rgba(255, 255, 255, 0.91);
}
.suw-pill-icon {
  display: inline-flex;
  flex-shrink: 0;
}
.suw-pill-label {
  min-width: 0;
}
</style>
