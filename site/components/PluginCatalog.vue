<script setup lang="ts">
import { availableFilters, filterPlugins } from '~/utils/filter'
import type { RegistryPlugin } from '~/types/registry'

const props = withDefaults(defineProps<{ plugins: RegistryPlugin[], heading?: string, intro?: string }>(), {
  heading: 'Explore plugins',
  intro: 'Search by capability, component, or source.',
})
const { repositoryUrl } = useSite()
const query = ref('')
const category = ref('')
const component = ref<RegistryPlugin['components'][number] | ''>('')
const source = ref<'all' | 'built-in' | 'external'>('all')
const filters = computed(() => availableFilters(props.plugins))
const visible = computed(() => filterPlugins(props.plugins, {
  query: query.value,
  category: category.value,
  component: component.value || undefined,
  source: source.value,
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
      <label>
        <span class="sr-only">Filter by category</span>
        <select v-model="category">
          <option value="">All categories</option>
          <option v-for="item in filters.categories" :key="item" :value="item">{{ item }}</option>
        </select>
      </label>
      <label>
        <span class="sr-only">Filter by component</span>
        <select v-model="component">
          <option value="">All components</option>
          <option v-for="item in filters.components" :key="item" :value="item">{{ item }}</option>
        </select>
      </label>
      <label>
        <span class="sr-only">Filter by source</span>
        <select v-model="source">
          <option value="all">All sources</option>
          <option value="built-in">Built-ins</option>
          <option value="external">External</option>
        </select>
      </label>
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
      <button class="button button--secondary" type="button" @click="query = ''; category = ''; component = ''; source = 'all'">Clear filters</button>
    </div>
  </section>
</template>
