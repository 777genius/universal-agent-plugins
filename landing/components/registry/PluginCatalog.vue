<script setup lang="ts">
import { useCatalogUiStore } from '~/stores/catalogUi';
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
const { t, n } = useI18n();

const props = withDefaults(
  defineProps<{ plugins: RegistryPlugin[]; heading?: string; intro?: string }>(),
  {
    heading: undefined,
    intro: undefined,
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
const catalogUi = useCatalogUiStore();
const family = computed(() =>
  /\/plugins\/?$/.test(route.path) ? ('catalog' as const) : ('home' as const),
);
const fingerprint = computed(() => JSON.stringify(filterRefs.map((filter) => filter.value)));
catalogUi.reconcile(family.value, fingerprint.value);
const displayLimit = computed(() =>
  catalogUi.displayLimit(family.value, fingerprint.value, pageSize),
);
function showMore() {
  catalogUi.showMore(family.value, fingerprint.value, displayLimit.value + pageSize);
}
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
    { value: 'all', label: t('registryUi.catalog.allCategories') },
    ...filters.value.categories.map((item) => ({ value: item, label: item })),
  ];
  previousCategoryOptions = stableOptions(next, previousCategoryOptions);
  return previousCategoryOptions;
});
const stableComponentOptions = computed(() => {
  const next = [
    { value: 'all', label: t('registryUi.catalog.allComponents') },
    ...filters.value.components.map((item) => ({
      value: item,
      label: t(`registryUi.components.${item}`),
    })),
  ];
  previousComponentOptions = stableOptions(next, previousComponentOptions);
  return previousComponentOptions;
});
const sourceOptions = computed(() => [
  { value: 'all', label: t('registryUi.catalog.allSources') },
  { value: 'upstream', label: t('registryUi.catalog.upstreamPackages') },
  { value: 'community_bridge', label: t('registryUi.catalog.communityBridges') },
  { value: 'community', label: t('registryUi.catalog.communityPackages') },
  { value: 'direct', label: t('registryUi.catalog.directSources') },
]);
const trustOptions = computed(() => [
  { value: 'all', label: t('registryUi.catalog.allTrustLevels') },
  { value: 'reviewed', label: t('registryUi.catalog.reviewedListings') },
  { value: 'conformant_unreviewed', label: t('registryUi.catalog.communityDiscovery') },
]);
const clientOptions = computed(() => [
  { value: 'all', label: t('registryUi.catalog.allAgents') },
  ...clients.map((item) => ({
    value: item.id,
    label: item.name,
    icon: asset(`client-icons/${item.icon}`),
  })),
]);
const authenticationOptions = computed(() => [
  { value: 'all', label: t('registryUi.catalog.allAuthentication') },
  { value: 'none', label: t('registryUi.catalog.worksWithoutSignIn') },
  { value: 'required_or_unknown', label: t('registryUi.catalog.mayRequireSignIn') },
]);
const ownerOptions = computed(() => [
  { value: 'all', label: t('registryUi.catalog.allOwners') },
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
    t('registryUi.catalog.search'),
    t('registryUi.catalog.category'),
    t('registryUi.catalog.component'),
    t('registryUi.catalog.source'),
    t('registryUi.catalog.trust'),
    t('registryUi.catalog.agent'),
    t('registryUi.catalog.authentication'),
    t('registryUi.catalog.owner'),
  ];
  const options = [
    [],
    stableCategoryOptions.value,
    stableComponentOptions.value,
    sourceOptions.value,
    trustOptions.value,
    clientOptions.value,
    authenticationOptions.value,
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
  const params = {
    total: n(catalogTotal.value),
    shown: n(displayed.value.length),
    matching: n(visible.value.length),
  };
  if (!visible.value.length) return t('registryUi.catalog.noMatches', params);
  if (visible.value.length === catalogTotal.value)
    return t('registryUi.catalog.shown', params, catalogTotal.value);
  if (displayed.value.length < visible.value.length)
    return t('registryUi.catalog.shownMatching', params);
  return t('registryUi.catalog.matching', params);
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
  catalogUi.reconcile(family.value, fingerprint.value);
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
      <p class="eyebrow">{{ t('registryUi.catalog.pluginDirectory') }}</p>
      <div class="catalog-heading-row">
        <h2 id="catalog-title">{{ heading ?? t('registryUi.catalog.explorePlugins') }}</h2>
        <a
          class="catalog-add-button"
          :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`"
          target="_blank"
          rel="noreferrer"
          :aria-label="t('registryUi.catalog.addAPlugin')"
        >
          <span aria-hidden="true">＋</span><span>{{ t('registryUi.catalog.addPlugin') }}</span>
        </a>
      </div>
      <p>{{ intro ?? t('registryUi.catalog.searchByCapabilityComponentOrSource') }}</p>
    </div>
    <div class="catalog-controls" role="search" :aria-label="t('registryUi.catalog.filterPlugins')">
      <label class="search-field">
        <span class="sr-only">{{ t('registryUi.catalog.searchPlugins') }}</span>
        <svg class="search-field__icon" aria-hidden="true" viewBox="0 0 24 24" fill="none">
          <circle cx="11" cy="11" r="6.5" />
          <path d="m16 16 4 4" />
        </svg>
        <input
          v-model="query"
          type="search"
          :aria-label="t('registryUi.catalog.searchPlugins')"
          :placeholder="t('registryUi.catalog.searchByNameAuthorOrCapability')"
        />
        <button
          v-if="query"
          class="search-field__clear"
          type="button"
          :aria-label="t('registryUi.catalog.clearPluginSearch')"
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
        <span>{{
          mobileFiltersOpen
            ? t('registryUi.catalog.hideFilters')
            : t('registryUi.catalog.moreFilters')
        }}</span>
        <span v-if="activeFilterCount" class="catalog-filter-toggle__count">{{
          t('registryUi.catalog.activeCount', { count: n(activeFilterCount) })
        }}</span>
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
          :label="t('registryUi.catalog.filterByCategory')"
          :search-placeholder="t('registryUi.catalog.searchCategories')"
          :options="stableCategoryOptions"
        />
        <AppSelect
          v-model="component"
          leading-icon="component"
          :label="t('registryUi.catalog.filterByComponent')"
          :options="stableComponentOptions"
        />
        <AppSelect
          v-model="source"
          leading-icon="source"
          :label="t('registryUi.catalog.filterBySource')"
          :options="sourceOptions"
        />
        <AppSelect
          v-model="trust"
          leading-icon="trust"
          :label="t('registryUi.catalog.filterByTrustLevel')"
          :options="trustOptions"
        />
        <AppSelect
          v-model="client"
          leading-icon="agent"
          :label="t('registryUi.catalog.filterByAgent')"
          :options="clientOptions"
        />
        <AppSelect
          v-model="authentication"
          leading-icon="authentication"
          :label="t('registryUi.catalog.filterByAuthentication')"
          :options="authenticationOptions"
        />
        <AppCombobox
          v-model="owner"
          leading-icon="owner"
          :label="t('registryUi.catalog.filterByOwner')"
          :search-placeholder="t('registryUi.catalog.searchOwners')"
          :options="ownerOptions"
        />
      </div>
    </div>
    <div class="catalog-active-filters" :aria-label="t('registryUi.catalog.activeFilters')">
      <button
        v-for="chip in activeChips"
        :key="chip.index"
        type="button"
        class="catalog-filter-chip"
        :aria-label="t('registryUi.catalog.removeFilter', { label: chip.label })"
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
        {{ t('registryUi.catalog.resetFilters') }}
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
          <template v-if="discovery.state === 'loading'">{{
            t('registryUi.catalog.findingMoreCommunityPluginsOnGithub')
          }}</template>
          <template v-else-if="discovery.state === 'stale'">{{
            t('registryUi.catalog.communityResultsAreRefreshingReviewedListingsRemainAvailable')
          }}</template>
          <template v-else-if="discovery.state === 'unavailable'">{{
            t(
              'registryUi.catalog.communityResultsAreTemporarilyUnavailableReviewedListingsRemainAvailable',
            )
          }}</template>
        </p>
      </div>
    </div>
    <div v-if="visible.length" class="plugin-grid">
      <RegistryPluginCard
        v-for="group in displayed"
        :key="group.primary.install_source"
        :plugin="group.primary"
        :alternatives="group.alternatives"
      />
    </div>
    <div v-else class="empty-state">
      <h3>{{ t('registryUi.catalog.noMatchingPlugins') }}</h3>
      <p>{{ t('registryUi.catalog.tryABroaderSearchOrClearOneOfTheFilters') }}</p>
      <button class="button button--secondary" type="button" @click="clearFilters">
        {{ t('registryUi.catalog.clearFilters') }}
      </button>
    </div>
    <div v-if="remaining" class="catalog-more">
      <button class="button button--secondary" type="button" @click="showMore">
        {{ t('registryUi.catalog.showMore', { count: n(Math.min(pageSize, remaining)) }) }}
        <span aria-hidden="true">↓</span>
      </button>
      <span>{{ t('registryUi.catalog.remaining', { count: n(remaining) }, remaining) }}</span>
    </div>
    <div class="catalog-end-submit">
      <a
        class="button button--secondary"
        :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`"
        target="_blank"
        rel="noreferrer"
        >{{ t('registryUi.catalog.addAPluginByPullRequest') }}<span aria-hidden="true">↗</span></a
      >
    </div>
  </section>
</template>
