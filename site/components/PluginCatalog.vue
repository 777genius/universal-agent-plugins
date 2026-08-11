<script setup lang="ts">
import { availableFilters, filterPlugins } from '~/utils/filter'
import type { RegistryPlugin } from '~/types/registry'

const props = withDefaults(defineProps<{ plugins: RegistryPlugin[], heading?: string, intro?: string }>(), {
  heading: 'Explore plugins',
  intro: 'Search by capability, component, or source.',
})
const { repositoryUrl } = useSite()
const query = ref('')
const category = ref('all')
const component = ref('all')
const source = ref('all')
const filters = computed(() => availableFilters(props.plugins))
const categoryOptions = computed(() => [
  { value: 'all', label: 'All categories' },
  ...filters.value.categories.map(item => ({ value: item, label: item })),
])
const componentOptions = computed(() => [
  { value: 'all', label: 'All components' },
  ...filters.value.components.map(item => ({ value: item, label: item })),
])
const sourceOptions = [
  { value: 'all', label: 'All sources' },
  { value: 'built-in', label: 'Built-ins' },
  { value: 'external', label: 'External' },
]
const visible = computed(() => filterPlugins(props.plugins, {
  query: query.value,
  category: category.value === 'all' ? '' : category.value,
  component: component.value === 'all' ? undefined : component.value as RegistryPlugin['components'][number],
  source: source.value as 'all' | 'built-in' | 'external',
}))
</script>

<template>
  <section class="catalog" aria-labelledby="catalog-title">
    <div class="section-heading">
      <p class="eyebrow">Plugin directory</p>
      <h2 id="catalog-title">{{ heading }}</h2>
      <p>{{ intro }}</p>
    </div>
    <div class="catalog-controls" role="search" aria-label="Filter plugins">
      <label class="search-field">
        <span class="sr-only">Search plugins</span>
        <span aria-hidden="true">⌕</span>
        <input v-model="query" type="search" placeholder="Search by name, author, or capability…" />
      </label>
      <AppSelect v-model="category" label="Filter by category" :options="categoryOptions" />
      <AppSelect v-model="component" label="Filter by component" :options="componentOptions" />
      <AppSelect v-model="source" label="Filter by source" :options="sourceOptions" />
    </div>
    <div class="catalog-meta">
      <div class="catalog-count" aria-live="polite">Showing {{ visible.length }} of {{ plugins.length }} plugins</div>
      <a class="button button--secondary catalog-submit" :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`" target="_blank" rel="noreferrer">
        <span aria-hidden="true">＋</span> Add your plugin
      </a>
    </div>
    <div v-if="visible.length" class="plugin-grid">
      <PluginCard v-for="plugin in visible" :key="plugin.name" :plugin="plugin" />
    </div>
    <div v-else class="empty-state">
      <h3>No matching plugins</h3>
      <p>Try a broader search or clear one of the filters.</p>
      <button class="button button--secondary" type="button" @click="query = ''; category = 'all'; component = 'all'; source = 'all'">Clear filters</button>
    </div>
  </section>
</template>
