<script setup lang="ts">
const registry = useRegistry()
const { expired } = useDirectoryStatus(true)

function conciseDate(value: string): string {
  return new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(value))
}
</script>

<template>
  <div class="app-shell">
    <a class="skip-link" href="#main-content">Skip to content</a>
    <PageBackground />
    <AppHeader />
    <div v-if="registry.data_source === 'review_preview'" class="preview-banner" role="status">Pull request preview — unresolved review data is shown for review only. Production commands come from a published signed Directory snapshot.</div>
    <div v-else-if="registry.data_source === 'published_snapshot'" class="directory-meta" :class="{ 'directory-meta--stale': expired }" :role="expired ? 'alert' : 'status'">
      <template v-if="expired"><strong>Stale Directory snapshot.</strong> Published data expired <time :datetime="registry.expires_at">{{ conciseDate(registry.expires_at!) }}</time>. Historical browsing remains available; install commands and candidate claims are disabled.</template>
      <template v-else>Signed Directory snapshot {{ registry.snapshot_sequence }} · generated <time :datetime="registry.generated_at">{{ conciseDate(registry.generated_at!) }}</time> · expires <time :datetime="registry.expires_at">{{ conciseDate(registry.expires_at!) }}</time></template>
    </div>
    <main id="main-content">
      <slot />
    </main>
    <AppFooter />
  </div>
</template>
