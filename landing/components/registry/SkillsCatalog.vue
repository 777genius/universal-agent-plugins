<script setup lang="ts">
import type { SkillsView } from '~/types/skillsCatalog';
import type { SkillsScanView } from '~/composables/useSkillsCatalog';
import { restoreSkillsQuery, skillsCatalogQuery } from '~/utils/skillsCatalog';
import { canonicalPath } from '~/utils/seo';

defineProps<{ hideHeading?: boolean }>();

const { t, n, d } = useI18n({ useScope: 'parent' });
const route = useRoute();
const router = useRouter();
const initial = restoreSkillsQuery(route.query);
const query = ref(initial.query);
const view = ref<SkillsView>(initial.view);
const scanMode = ref<SkillsScanView>(route.query.skillScan === 'all' ? 'all' : 'scanned');
const routeState = () => ({
  ...restoreSkillsQuery(route.query),
  scanMode: route.query.skillScan === 'all' ? 'all' : 'scanned',
});
let syncing = false;
let disposed = false;
let pending: string | undefined;
const fingerprint = () => JSON.stringify({
  query: query.value.trim(),
  view: view.value,
  scanMode: scanMode.value,
});
watch(
  () => route.query,
  () => {
    const restored = routeState();
    if (JSON.stringify(restored) === pending) return;
    query.value = restored.query;
    view.value = restored.view;
    scanMode.value = restored.scanMode as SkillsScanView;
  },
);
async function syncQuery() {
  if (syncing) return;
  syncing = true;
  try {
    while (!disposed) {
      if (fingerprint() === JSON.stringify(routeState())) break;
      pending = fingerprint();
      const params = skillsCatalogQuery(query.value, view.value, route.query);
      if (scanMode.value === 'all') params.skillScan = 'all';
      else delete params.skillScan;
      const failure = await router.replace({
        path: canonicalPath(route.path),
        query: params,
        hash: '',
      });
      if (failure) break;
    }
  } finally {
    pending = undefined;
    syncing = false;
  }
}
watch([query, view, scanMode], () => {
  void syncQuery();
});
onScopeDispose(() => {
  disposed = true;
});
const {
  displayed,
  metadata,
  scannedOnly,
  scanReportUnavailable,
  loading,
  loadingMore,
  failed,
  hasMore,
  searchTooShort,
  showMore,
  retry,
} = useSkillsCatalog(query, view, scanMode);
const scanOptions = computed(() =>
  (['scanned', 'all'] as const).map((value) => ({
    value,
    label: t('registryUi.skills.scanModes.' + value),
  })),
);
const viewOptions = computed(() =>
  (['popular', 'trending', 'hot'] as const).map((value) => ({
    value,
    label: t(`registryUi.skills.views.${value}`),
  })),
);
const staleDate = computed(() => {
  const date = metadata.value?.cache.fetchedAt;
  return date && Number.isFinite(Date.parse(date))
    ? d(new Date(date), { dateStyle: 'medium', timeStyle: 'short' })
    : '';
});
</script>

<template>
  <section
    class="catalog skills-catalog"
    :aria-labelledby="hideHeading ? 'unified-catalog-title' : 'skills-catalog-title'"
    :aria-busy="loading"
  >
    <div v-if="!hideHeading" class="section-heading">
      <div class="catalog-heading-row">
        <h2 id="skills-catalog-title">{{ t('registryUi.skills.heading') }}</h2>
      </div>
      <p>{{ t('registryUi.skills.intro') }}</p>
    </div>
    <div
      class="catalog-controls skills-catalog__controls"
      role="search"
      :aria-label="t('registryUi.skills.search')"
    >
      <CatalogSearchField
        v-model="query"
        :label="t('registryUi.skills.search')"
        :placeholder="t('registryUi.skills.searchPlaceholder')"
        :clear-label="t('registryUi.skills.clearSearch')"
      />
      <AppSelect
        v-if="!query.trim()"
        v-model="view"
        leading-icon="category"
        :label="t('registryUi.skills.sortLabel')"
        :options="viewOptions"
      />
      <AppSelect
        v-if="!query.trim()"
        v-model="scanMode"
        leading-icon="shield-check"
        :label="t('registryUi.skills.scanFilterLabel')"
        :options="scanOptions"
      />
    </div>
    <p v-if="scannedOnly" class="skills-catalog__scan-notice">
      {{ t('registryUi.skills.scanNotice') }}
    </p>
    <div class="catalog-meta">
      <div aria-live="polite">
        <p v-if="query.trim() && !searchTooShort && !loading && !failed" class="catalog-count">
          {{ t('registryUi.skills.searchResults', { count: n(displayed.length) }) }}
        </p>
        <p v-else-if="!loading && !searchTooShort && displayed.length" class="catalog-count">
          {{ t('registryUi.skills.loaded', { count: n(displayed.length) }) }}
        </p>
        <p v-if="scannedOnly && scanReportUnavailable" role="status">
          {{ t('registryUi.skills.scanReportUnavailable') }}
        </p>
        <p
          v-if="metadata?.cache.status === 'stale'"
          class="discovery-status discovery-status--stale"
        >
          {{ t('registryUi.skills.stale', { date: staleDate }) }}
        </p>
      </div>
    </div>
    <div v-if="searchTooShort" class="empty-state">
      <h3>{{ t('registryUi.skills.minSearch') }}</h3>
      <p>{{ t('registryUi.skills.minSearchHelp') }}</p>
    </div>
    <div
      v-else-if="loading"
      class="plugin-grid"
      :aria-label="t('registryUi.skills.loading')"
      role="status"
    >
      <CatalogCard v-for="index in 6" :key="index" class="skill-card--skeleton" aria-hidden="true"
        ><div />
        <div />
        <div
      /></CatalogCard>
      <span class="sr-only">{{ t('registryUi.skills.loading') }}</span>
    </div>
    <div v-else-if="failed && !displayed.length" class="empty-state" role="status">
      <h3>{{ t('registryUi.skills.unavailable') }}</h3>
      <p>{{ t('registryUi.skills.unavailableHelp') }}</p>
      <button type="button" class="button button--secondary" @click="retry">
        {{ t('registryUi.skills.retry') }}
      </button>
    </div>
    <template v-else>
      <div v-if="displayed.length" class="plugin-grid">
        <SkillCard v-for="skill in displayed" :key="skill.id" :skill="skill" />
      </div>
      <div v-else class="empty-state">
        <h3>{{ t(scannedOnly ? 'registryUi.skills.scanNone' : 'registryUi.skills.noMatches') }}</h3>
        <p>{{ t(scannedOnly ? 'registryUi.skills.scanHelp' : 'registryUi.skills.noMatchesHelp') }}</p>
        <button type="button" class="button button--secondary"
          @click="scannedOnly ? scanMode = 'all' : query = ''">
          {{ t(scannedOnly ? 'registryUi.skills.scanClear' : 'registryUi.skills.clearSearch') }}
        </button>
      </div>
      <div v-if="hasMore || failed" class="catalog-more">
        <p v-if="failed" role="status">{{ t('registryUi.skills.moreUnavailable') }}</p>
        <button
          type="button"
          class="button button--secondary"
          :disabled="loadingMore"
          @click="failed ? retry() : showMore()"
        >
          {{
            loadingMore
              ? t('registryUi.skills.loading')
              : failed
                ? t('registryUi.skills.retry')
                : t('registryUi.skills.showMore')
          }}<span v-if="!loadingMore" aria-hidden="true">↓</span>
        </button>
      </div>
    </template>
  </section>
</template>
