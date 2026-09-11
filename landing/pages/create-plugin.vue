<script setup lang="ts">
// Keep this page out of search until standard-first authoring reaches its own
// release gate (see docs/AUTHORING_CAPABILITY_PRESERVATION.md and the release
// holds in the implementation plan) — the page itself stays reachable only by
// direct URL and is not linked from the header/footer yet.
usePageSeo(
  'Use plugins. Build plugins. | Universal Agent Plugins',
  'Install and manage Agent Plugins 1.0 packages today, or prepare your own portable package while standard-first authoring is in preview.',
  {
    translate: false,
    robots: 'noindex, follow',
  },
);
useTrackSections();
const { containerRef } = useParallaxSections();
</script>

<template>
  <div ref="containerRef" class="page">
    <PageBackground />
    <AuthoringHeroSection />
    <SectionDivider />
    <AuthoringJourneysSection />
    <SectionDivider :flip="true" />
    <LazyCustomLogicSection />
  </div>
</template>
