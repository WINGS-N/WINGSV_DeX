<template>
  <span v-if="state === 'applying'" class="patchdot" aria-hidden="true">
    <span class="samsung-loader">
      <span class="samsung-loader-dot samsung-loader-dot-top"></span>
      <span class="samsung-loader-dot samsung-loader-dot-right"></span>
      <span class="samsung-loader-dot samsung-loader-dot-bottom"></span>
      <span class="samsung-loader-dot samsung-loader-dot-left"></span>
    </span>
  </span>
  <span v-else-if="state === 'failed' || state === 'reverted_needs_restart'" class="shrink-0 text-[15px] leading-none">
    ⚠️
  </span>
</template>

<script setup>
// Per-row live-patch status: the classic Samsung four-dot loader while the relay
// applies the field, a warning glyph when it could only take effect on the next
// restart. Nothing when idle or applied.
defineProps({
  state: { type: String, default: '' },
});
</script>

<style scoped>
/* Shrink with `zoom`, not `transform: scale` (which freezes the loader's rotate
   animation under WebKitGTK compositing). */
/* Flex-centre so the loader spins around its own centre (top/left:50% offset it by its
   size, which cannot be corrected with translate while the rotate animation owns it). */
.patchdot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  zoom: 0.5;
}
.patchdot :deep(.samsung-loader) {
  position: relative;
}
</style>
