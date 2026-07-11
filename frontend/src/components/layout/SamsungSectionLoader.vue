<template>
  <div class="loading-layout">
    <span class="loading-dots">
      <span class="samsung-loader" aria-hidden="true">
        <span class="samsung-loader-dot samsung-loader-dot-top"></span>
        <span class="samsung-loader-dot samsung-loader-dot-right"></span>
        <span class="samsung-loader-dot samsung-loader-dot-bottom"></span>
        <span class="samsung-loader-dot samsung-loader-dot-left"></span>
      </span>
    </span>
    <p v-if="label" class="admin-muted mt-3">{{ label }}</p>
  </div>
</template>

<script setup>
defineProps({
  /** Optional helper text shown beneath the loader. */
  label: { type: String, default: '' },
});
</script>

<style scoped>
.loading-layout {
  display: flex;
  min-height: 220px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

/* Flex-centre the loader box so it spins around its own centre. top/left:50% offset it by
   its size, and translate cannot pull it back because the rotate animation owns transform. */
.loading-dots {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
}

.loading-dots :deep(.samsung-loader) {
  position: relative;
}
</style>
