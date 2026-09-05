<script setup lang="ts">
import {
  availableFilters,
  catalogVisiblePlugins,
  filterPlugins,
  groupCatalogPlugins,
  catalogQuery,
  restoreCatalogQuery,
} from '~/utils/filter';
import type { LocationQueryRaw } from 'vue-router';
import type { RegistryPlugin } from '~/types/registry';
import { canonicalPath } from '~/utils/seo';

const props = withDefaults(
  defineProps<{ plugins: RegistryPlugin[]; heading?: string; intro?: string }>(),
  {
    heading: 'Explore plugins',
    intro: 'Search by capability, component, or source.',
  },
);
const { asset, repositoryUrl } = useSite();
const query = ref('');
const category = ref('all');
const component = ref('all');
const source = ref('all');
const trust = ref('all');
const client = ref('all');
const authentication = ref('all');
const owner = ref('all');
const route = useRoute();
const router = useRouter();
const filterRefs = [query, category, component, source, trust, client, authentication, owner];
function restoreFilters() {
  restoreCatalogQuery(route.query).forEach((value, index) => {
    filterRefs[index]!.value = value;
  });
}
restoreFilters();
watch(() => route.query, restoreFilters);
const mobileFiltersOpen = ref(false);
const pageSize = 48;
const displayLimit = ref(pageSize);
const discovery = useDiscoveryStatus();
const catalogPlugins = computed(() => catalogVisiblePlugins(props.plugins));
const catalogTotal = computed(() => groupCatalogPlugins(catalogPlugins.value).length);
const filters = computed(() => availableFilters(catalogPlugins.value));
type FilterOption = { value: string; label: string };
let previousCategoryOptions: FilterOption[] = [];
let previousComponentOptions: FilterOption[] = [];
function stableOptions(next: FilterOption[], previous: FilterOption[]) {
  return next.length === previous.length &&
    next.every(
      (item, index) =>
        item.value === previous[index]?.value && item.label === previous[index]?.label,
    )
    ? previous
    : next;
}
const stableCategoryOptions = computed(() => {
  const next = [
    { value: 'all', label: 'All categories' },
    ...filters.value.categories.map((item) => ({ value: item, label: item })),
  ];
  previousCategoryOptions = stableOptions(next, previousCategoryOptions);
  return previousCategoryOptions;
});
const stableComponentOptions = computed(() => {
  const next = [
    { value: 'all', label: 'All components' },
    ...filters.value.components.map((item) => ({ value: item, label: item })),
  ];
  previousComponentOptions = stableOptions(next, previousComponentOptions);
  return previousComponentOptions;
});
const sourceOptions = [
  { value: 'all', label: 'All sources' },
  { value: 'upstream', label: 'Upstream packages' },
  { value: 'community_bridge', label: 'Community bridges' },
  { value: 'community', label: 'Community packages' },
  { value: 'direct', label: 'Direct sources' },
];
const trustOptions = [
  { value: 'all', label: 'All trust levels' },
  { value: 'reviewed', label: 'Reviewed listings' },
  { value: 'conformant_unreviewed', label: 'Community discovery' },
];
const clientOptions = [
  { value: 'all', label: 'All agents' },
  ...clients.map((item) => ({
    value: item.id,
    label: item.name,
    icon: asset(`client-icons/${item.icon}`),
  })),
];
const authenticationOptions = [
  { value: 'all', label: 'All authentication' },
  { value: 'none', label: 'Works without sign-in' },
  { value: 'required_or_unknown', label: 'May require sign-in' },
];
const ownerOptions = computed(() => [
  { value: 'all', label: 'All owners' },
  ...filters.value.owners.map((item) => ({ value: item, label: item })),
]);
const visible = computed(() =>
  groupCatalogPlugins(
    filterPlugins(catalogPlugins.value, {
      query: query.value,
      category: category.value === 'all' ? '' : category.value,
      component:
        component.value === 'all'
          ? undefined
          : (component.value as RegistryPlugin['components'][number]),
      source: source.value as 'all' | 'upstream' | 'community_bridge' | 'community' | 'direct',
      trust: trust.value as 'all' | 'reviewed' | 'conformant_unreviewed',
      client: client.value as 'all' | RegistryPlugin['client_support']['clients'][number],
      authentication: authentication.value as 'all' | 'none' | 'required_or_unknown',
      owner: owner.value === 'all' ? '' : owner.value,
    }),
  ),
);
const displayed = computed(() => visible.value.slice(0, displayLimit.value));
const remaining = computed(() => Math.max(0, visible.value.length - displayed.value.length));
const activeFilterCount = computed(
  () =>
    [category, component, source, trust, client, authentication, owner].filter(
      (filter) => filter.value !== 'all',
    ).length,
);
const activeChips = computed(() => {
  const labels = [
    'Search',
    'Category',
    'Component',
    'Source',
    'Trust',
    'Agent',
    'Authentication',
    'Owner',
  ];
  const options = [
    [],
    stableCategoryOptions.value,
    stableComponentOptions.value,
    sourceOptions,
    trustOptions,
    clientOptions,
    authenticationOptions,
    ownerOptions.value,
  ];
  return filterRefs.flatMap((filter, index) =>
    filter.value !== (index === 0 ? '' : 'all')
      ? [
          {
            index,
            label: `${labels[index]}: ${options[index]?.find((option) => option.value === filter.value)?.label ?? filter.value}`,
          },
        ]
      : [],
  );
});
const catalogSummary = computed(() => {
  const total = catalogTotal.value;
  if (!visible.value.length) return `No matches · ${total} total`;
  if (visible.value.length === total) return `${displayed.value.length} shown · ${total} plugins`;
  if (displayed.value.length < visible.value.length)
    return `${displayed.value.length} shown · ${visible.value.length} matching · ${total} total`;
  return `${visible.value.length} matching · ${total} total`;
});

function clearFilters() {
  query.value = '';
  category.value = 'all';
  component.value = 'all';
  source.value = 'all';
  trust.value = 'all';
  client.value = 'all';
  authentication.value = 'all';
  owner.value = 'all';
  mobileFiltersOpen.value = false;
}

watch([query, category, component, source, trust, client, authentication, owner], () => {
  displayLimit.value = pageSize;
  const values = filterRefs.map((filter) => filter.value);
  if (JSON.stringify(values) !== JSON.stringify(restoreCatalogQuery(route.query))) {
    void router.replace({
      path: canonicalPath(route.path),
      query: catalogQuery(values, route.query) as LocationQueryRaw,
      hash: route.hash,
    });
  }
});
</script>

<template>
  <section class="catalog" aria-labelledby="catalog-title" :data-discovery-state="discovery.state">
    <div class="section-heading">
      <p class="eyebrow">Plugin directory</p>
      <div class="catalog-heading-row">
        <h2 id="catalog-title">{{ heading }}</h2>
        <a
          class="catalog-add-button"
          :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`"
          target="_blank"
          rel="noreferrer"
          aria-label="Add a plugin"
        >
          <span aria-hidden="true">＋</span><span>Add plugin</span>
        </a>
      </div>
      <p>{{ intro }}</p>
    </div>
    <div class="catalog-controls" role="search" aria-label="Filter plugins">
      <label class="search-field">
        <span class="sr-only">Search plugins</span>
        <svg class="search-field__icon" aria-hidden="true" viewBox="0 0 24 24" fill="none">
          <circle cx="11" cy="11" r="6.5" />
          <path d="m16 16 4 4" />
        </svg>
        <input
          v-model="query"
          type="search"
          aria-label="Search plugins"
          placeholder="Search by name, author, or capability…"
        />
        <button
          v-if="query"
          class="search-field__clear"
          type="button"
          aria-label="Clear plugin search"
          @click="query = ''"
        >
          <span aria-hidden="true">×</span>
        </button>
      </label>
      <button
        class="catalog-filter-toggle"
        type="button"
        :aria-expanded="mobileFiltersOpen"
        aria-controls="catalog-advanced-filters"
        @click="mobileFiltersOpen = !mobileFiltersOpen"
      >
        <FilterIcon name="category" />
        <span>{{ mobileFiltersOpen ? 'Hide filters' : 'More filters' }}</span>
        <span v-if="activeFilterCount" class="catalog-filter-toggle__count"
          >{{ activeFilterCount }} active</span
        >
        <span class="catalog-filter-toggle__chevron" aria-hidden="true">⌄</span>
      </button>
      <div
        id="catalog-advanced-filters"
        class="catalog-advanced-filters"
        :class="{ 'catalog-advanced-filters--open': mobileFiltersOpen }"
      >
        <AppCombobox
          v-model="category"
          leading-icon="category"
          label="Filter by category"
          search-placeholder="Search categories…"
          :options="stableCategoryOptions"
        />
        <AppSelect
          v-model="component"
          leading-icon="component"
          label="Filter by component"
          :options="stableComponentOptions"
        />
        <AppSelect
          v-model="source"
          leading-icon="source"
          label="Filter by source"
          :options="sourceOptions"
        />
        <AppSelect
          v-model="trust"
          leading-icon="trust"
          label="Filter by trust level"
          :options="trustOptions"
        />
        <AppSelect
          v-model="client"
          leading-icon="agent"
          label="Filter by agent"
          :options="clientOptions"
        />
        <AppSelect
          v-model="authentication"
          leading-icon="authentication"
          label="Filter by authentication"
          :options="authenticationOptions"
        />
        <AppCombobox
          v-model="owner"
          leading-icon="owner"
          label="Filter by owner"
          search-placeholder="Search owners…"
          :options="ownerOptions"
        />
      </div>
    </div>
    <div class="catalog-active-filters" aria-label="Active filters">
      <button
        v-for="chip in activeChips"
        :key="chip.index"
        type="button"
        class="catalog-filter-chip"
        :aria-label="`Remove ${chip.label}`"
        @click="filterRefs[chip.index]!.value = chip.index === 0 ? '' : 'all'"
      >
        {{ chip.label }} <span aria-hidden="true">×</span>
      </button>
      <button
        type="button"
        class="catalog-filter-reset"
        :disabled="!activeChips.length"
        @click="clearFilters"
      >
        Reset filters
      </button>
    </div>
    <div class="catalog-meta">
      <div>
        <div class="catalog-count" aria-live="polite">{{ catalogSummary }}</div>
        <p
          v-if="['loading', 'stale', 'unavailable'].includes(discovery.state)"
          class="discovery-status"
          :class="`discovery-status--${discovery.state}`"
        >
          <template v-if="discovery.state === 'loading'"
            >Finding more community plugins on GitHub…</template
          >
          <template v-else-if="discovery.state === 'stale'"
            >Community results are refreshing. Reviewed listings remain available.</template
          >
          <template v-else-if="discovery.state === 'unavailable'"
            >Community results are temporarily unavailable. Reviewed listings remain
            available.</template
          >
        </p>
      </div>
    </div>
    <div v-if="visible.length" class="plugin-grid">
      <div
        v-for="group in displayed"
        :key="group.primary.install_source"
        class="plugin-source-group"
      >
        <RegistryPluginCard :plugin="group.primary" />
        <details v-if="group.alternatives.length" class="plugin-other-sources">
          <summary>
            Other sources ({{ group.alternatives.length }})<span class="sr-only">
              for {{ group.primary.display_name }}</span
            >
          </summary>
          <ul class="plugin-other-sources__list">
            <li v-for="alternative in group.alternatives" :key="alternative.install_source">
              <NuxtLink
                :to="
                  alternative.trust_state === 'conformant_unreviewed'
                    ? { path: '/plugins/community/', query: { source: alternative.install_source } }
                    : `/plugins/${alternative.name}/`
                "
              >
                {{ alternative.source.repository
                }}{{ alternative.source.path ? `/${alternative.source.path}` : '' }}
              </NuxtLink>
              <span
                >{{
                  alternative.trust_state === 'conformant_unreviewed'
                    ? 'Community listing'
                    : 'Reviewed listing'
                }}
                ·
                {{
                  alternative.distributions.find(
                    (item) => item.id === alternative.default_distribution,
                  )?.kind === 'upstream'
                    ? 'Upstream'
                    : 'Community / direct'
                }}
                · {{ alternative.installable ? 'Installable' : 'Unavailable' }}</span
              >
            </li>
          </ul>
        </details>
      </div>
    </div>
    <div v-else class="empty-state">
      <h3>No matching plugins</h3>
      <p>Try a broader search or clear one of the filters.</p>
      <button class="button button--secondary" type="button" @click="clearFilters">
        Clear filters
      </button>
    </div>
    <div v-if="remaining" class="catalog-more">
      <button class="button button--secondary" type="button" @click="displayLimit += pageSize">
        Show {{ Math.min(pageSize, remaining) }} more <span aria-hidden="true">↓</span>
      </button>
      <span>{{ remaining }} matching plugins remaining</span>
    </div>
    <div class="catalog-end-submit">
      <a
        class="button button--secondary"
        :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`"
        target="_blank"
        rel="noreferrer"
        >Add a plugin by pull request <span aria-hidden="true">↗</span></a
      >
    </div>
  </section>
</template>
