<script setup lang="ts">
const registry = useRegistry()
</script>

<template>
  <div class="app-shell">
    <a class="skip-link" href="#main-content">Skip to content</a>
    <PageBackground />
    <AppHeader />
    <div v-if="registry.data_source === 'review_preview'" class="preview-banner" role="status">Pull request preview — unresolved review data is shown for review only. Production commands come from a published signed Directory snapshot.</div>
    <main id="main-content">
      <slot />
    </main>
    <AppFooter />
  </div>
</template>
