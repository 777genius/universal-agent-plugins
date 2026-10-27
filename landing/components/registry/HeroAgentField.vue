<script setup lang="ts">
const anchor = ref<HTMLElement>();
const field = ref<HTMLElement>();
const position = ref<{ left: string; top: string; width: string; height: string }>();
let resizeObserver: ResizeObserver | undefined;
let scrollFrame: number | undefined;

// Follow the viewport, with a gentle upward/leftward drift in viewport pixels.
const updateScroll = () => {
  scrollFrame = undefined;
  const distance = Math.max(0, window.scrollY);
  field.value?.style.setProperty('--scroll-x', `${-distance * 0.025}px`);
  field.value?.style.setProperty('--scroll-y', `${-distance * 0.12}px`);
};
const scheduleScroll = () => {
  if (scrollFrame === undefined) scrollFrame = requestAnimationFrame(updateScroll);
};
const measureAnchor = () => {
  if (!anchor.value) return;
  const bounds = anchor.value.getBoundingClientRect();
  const anchorLeft = bounds.left + window.scrollX;
  const anchorTop = bounds.top + window.scrollY;
  position.value = {
    left: `${anchorLeft}px`,
    top: `${anchorTop}px`,
    width: `${bounds.width}px`,
    height: `${bounds.height}px`,
  };
  scheduleScroll();
};
const inView = ref(false);
const pageVisible = ref(true);
let observer: IntersectionObserver | undefined;
const updatePageVisibility = () => {
  pageVisible.value = !document.hidden;
};

onMounted(async () => {
  measureAnchor();
  window.addEventListener('scroll', scheduleScroll, { passive: true });
  window.addEventListener('resize', measureAnchor);
  if (anchor.value && 'ResizeObserver' in window) {
    resizeObserver = new ResizeObserver(measureAnchor);
    resizeObserver.observe(anchor.value);
  }
  await nextTick();
  updateScroll();
  updatePageVisibility();
  document.addEventListener('visibilitychange', updatePageVisibility);
  if (field.value && 'IntersectionObserver' in window) {
    observer = new IntersectionObserver(([entry]) => {
      inView.value = Boolean(entry?.isIntersecting);
    });
    observer.observe(field.value);
  } else {
    inView.value = true;
  }
});

onBeforeUnmount(() => {
  observer?.disconnect();
  resizeObserver?.disconnect();
  window.removeEventListener('scroll', scheduleScroll);
  window.removeEventListener('resize', measureAnchor);
  if (scrollFrame !== undefined) cancelAnimationFrame(scrollFrame);
  document.removeEventListener('visibilitychange', updatePageVisibility);
});
</script>

<template>
  <span ref="anchor" class="hero-agent-field-anchor" aria-hidden="true" />
  <Teleport v-if="position" to=".unified-home">
    <div
      ref="field"
      class="hero-agent-field hero-agent-field--fixed"
      :class="{ 'hero-agent-field--active': inView && pageVisible }"
      :style="position"
      aria-hidden="true"
    >
      <HeroAgentOrbit />
    </div>
  </Teleport>
  <div v-else ref="field" class="hero-agent-field" aria-hidden="true">
    <HeroAgentOrbit />
  </div>
</template>

<style src="~/assets/styles/hero-agent-field.scss" lang="scss" />
