<template>
  <span class="dex-spin" aria-hidden="true">
    <span class="samsung-loader">
      <span class="samsung-loader-dot samsung-loader-dot-top"></span>
      <span class="samsung-loader-dot samsung-loader-dot-right"></span>
      <span class="samsung-loader-dot samsung-loader-dot-bottom"></span>
      <span class="samsung-loader-dot samsung-loader-dot-left"></span>
    </span>
  </span>
</template>

<script setup>
// Small inline variant of the standard colored Samsung loader (SamsungLoader is the
// white on-button variant; this one keeps the brand dot colors for use on cards).
</script>

<style scoped>
/* Shrink with `zoom`, not `transform: scale`: a static transform makes WebKitGTK
   composite this into a layer where the loader's own rotate animation freezes. `zoom`
   scales layout (and footprint) without that. */
.dex-spin {
  position: relative;
  display: inline-block;
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  zoom: 0.62;
}
.dex-spin :deep(.samsung-loader) {
  position: absolute;
  top: 50%;
  left: 50%;
}
</style>
