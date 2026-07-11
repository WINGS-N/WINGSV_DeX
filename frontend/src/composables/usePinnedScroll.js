import { onBeforeUnmount, onMounted, ref } from 'vue';

// Focusing a control (toggling a switch, tabbing into an input) makes WebKitGTK scroll an
// ancestor to bring it into view; in the overlays' fixed-height flex layout that displaces
// the whole page off-screen and never comes back (only the inner body is meant to scroll).
// Bind the returned ref to the overlay root: it pins every ancestor - and the document -
// back to 0 on any scroll, undoing stray focus scrolls while the inner body keeps scrolling.
export function usePinnedScroll() {
  const rootEl = ref(null);
  function pinAncestors() {
    let el = rootEl.value;
    while (el) {
      if (el.scrollTop) el.scrollTop = 0;
      if (el.scrollLeft) el.scrollLeft = 0;
      el = el.parentElement;
    }
    const se = document.scrollingElement;
    if (se && se.scrollTop) se.scrollTop = 0;
  }
  onMounted(() => window.addEventListener('scroll', pinAncestors, true));
  onBeforeUnmount(() => window.removeEventListener('scroll', pinAncestors, true));
  return rootEl;
}
