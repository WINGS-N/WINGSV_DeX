<template>
  <button
    ref="btn"
    type="button"
    aria-label="Наверх"
    class="fixed bottom-20 left-1/2 z-40 flex h-11 w-11 -translate-x-1/2 items-center justify-center rounded-full bg-[#3a3a3c] text-white shadow-lg transition-opacity duration-200"
    :class="visible ? 'opacity-90 hover:opacity-100' : 'pointer-events-none opacity-0'"
    @click="toTop"
  >
    <ChevronUp :size="24" />
  </button>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ChevronUp } from 'lucide-vue-next';

// Floating "scroll to top" button, reused across the profiles, apps and strategy lists. It
// finds its own nearest scrollable ancestor on mount, so it works both inside a tab (the
// shared <main> scroller) and inside a full-screen overlay (its own scroll area).
const btn = ref(null);
const visible = ref(false);
let scroller = null;

function findScroller(el) {
  for (let node = el?.parentElement; node; node = node.parentElement) {
    const oy = getComputedStyle(node).overflowY;
    if ((oy === 'auto' || oy === 'scroll') && node.scrollHeight > node.clientHeight) {
      return node;
    }
  }
  return null;
}

function onScroll() {
  visible.value = !!scroller && scroller.scrollTop > 300;
}

function toTop() {
  scroller?.scrollTo({ top: 0, behavior: 'smooth' });
}

onMounted(() => {
  scroller = findScroller(btn.value);
  if (scroller) {
    scroller.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
  }
});
onBeforeUnmount(() => {
  scroller?.removeEventListener('scroll', onScroll);
});
</script>
