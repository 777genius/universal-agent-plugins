<script setup lang="ts">
import type { SkillsView } from '~/types/skillsCatalog';
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
} from 'reka-ui';
import {
  restoreSkillsQuery,
  restoreSkillsFilters,
  skillsCatalogQuery,
  skillsFiltersQuery,
} from '~/utils/skillsCatalog';
import { canonicalPath } from '~/utils/seo';

defineProps<{ hideHeading?: boolean }>();

const { t, n, d } = useI18n({ useScope: 'parent' });
const route = useRoute();
const router = useRouter();
const initial = restoreSkillsQuery(route.query);
const query = ref(initial.query);
const view = ref<SkillsView>(initial.view);
const filters = ref(restoreSkillsFilters(route.query));
const filtersOpen = ref(false);
const activeFilterCount = computed(
  () => Number(filters.value.scannedOnly) + Number(filters.value.noRemarksOnly),
);
function clearFilters() {
  filters.value = { scannedOnly: false, noRemarksOnly: false };
}
/** Normalize the URL state, including the explicitly requested unfiltered mode. */
const routeState = () => ({
  ...restoreSkillsQuery(route.query),
  ...restoreSkillsFilters(route.query),
});
let syncing = false;
let disposed = false;
let pending: string | undefined;
/** Compare route and local state without overwriting a newer user selection. */
const fingerprint = () =>
  JSON.stringify({
    query: query.value.trim(),
    view: view.value,
    ...filters.value,
  });
watch(
  () => route.query,
  () => {
    const restored = routeState();
    if (JSON.stringify(restored) === pending) return;
    query.value = restored.query;
    view.value = restored.view;
    filters.value = { scannedOnly: restored.scannedOnly, noRemarksOnly: restored.noRemarksOnly };
  },
);
/** Serialize changes to avoid navigation races while keeping the shareable filter URL. */
async function syncQuery() {
  if (syncing) return;
  syncing = true;
  try {
    while (!disposed) {
      if (fingerprint() === JSON.stringify(routeState())) break;
      pending = fingerprint();
      const params = skillsFiltersQuery(
        skillsCatalogQuery(query.value, view.value, route.query),
        filters.value,
      );
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
watch([query, view, () => filters.value.scannedOnly, () => filters.value.noRemarksOnly], () => {
  void syncQuery();
});
onScopeDispose(() => {
  disposed = true;
});
const {
  displayed,
  metadata,
  requiresReport,
  scanReportUnavailable,
  loading,
  loadingMore,
  failed,
  hasMore,
  searchTooShort,
  showMore,
  retry,
} = useSkillsCatalog(query, view, filters);
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
      <DialogRoot v-model:open="filtersOpen">
        <DialogTrigger as-child>
          <button type="button" class="button button--secondary skills-filters__trigger">
            <FilterIcon name="filters" />
            <span>{{ t('registryUi.skills.filters.button') }}</span>
            <span
              v-if="activeFilterCount"
              class="skills-filters__count"
              :aria-label="t('registryUi.skills.filters.active', { count: activeFilterCount })"
            >
              {{ n(activeFilterCount) }}
            </span>
          </button>
        </DialogTrigger>
        <DialogPortal>
          <DialogOverlay class="skills-filters__overlay" />
          <DialogContent class="skills-filters__dialog">
            <div class="skills-filters__heading">
              <DialogTitle>{{ t('registryUi.skills.filters.title') }}</DialogTitle>
              <DialogClose as-child>
                <button
                  type="button"
                  class="skills-filters__close"
                  :aria-label="t('registryUi.skills.filters.close')"
                >
                  ×
                </button>
              </DialogClose>
            </div>
            <DialogDescription class="skills-filters__description">
              {{ t('registryUi.skills.filters.description') }}
            </DialogDescription>
            <label class="skills-filters__option">
              <input
                v-model="filters.scannedOnly"
                type="checkbox"
                aria-labelledby="skills-filter-scanned-label"
                aria-describedby="skills-filter-scanned-help"
              />
              <span>
                <strong id="skills-filter-scanned-label">{{ t('registryUi.skills.filters.scanned') }}</strong>
                <span id="skills-filter-scanned-help">{{
                  t('registryUi.skills.filters.scannedHelp')
                }}</span>
              </span>
            </label>
            <label class="skills-filters__option">
              <input
                v-model="filters.noRemarksOnly"
                type="checkbox"
                aria-labelledby="skills-filter-remarks-label"
                aria-describedby="skills-filter-remarks-help"
              />
              <span>
                <strong id="skills-filter-remarks-label">{{ t('registryUi.skills.filters.noRemarks') }}</strong>
                <span id="skills-filter-remarks-help">{{
                  t('registryUi.skills.filters.noRemarksHelp')
                }}</span>
              </span>
            </label>
            <div class="skills-filters__actions">
              <button
                type="button"
                class="button button--secondary"
                :disabled="!activeFilterCount"
                @click="clearFilters"
              >
                {{ t('registryUi.skills.filters.reset') }}
              </button>
              <DialogClose as-child>
                <button type="button" class="button button--primary">
                  {{ t('registryUi.skills.filters.close') }}
                </button>
              </DialogClose>
            </div>
          </DialogContent>
        </DialogPortal>
      </DialogRoot>
    </div>
    <div class="catalog-meta">
      <div aria-live="polite">
        <p v-if="query.trim() && !searchTooShort && !loading && !failed" class="catalog-count">
          {{ t('registryUi.skills.searchResults', { count: n(displayed.length) }) }}
        </p>
        <p v-else-if="!loading && !searchTooShort && displayed.length" class="catalog-count">
          {{ t('registryUi.skills.loaded', { count: n(displayed.length) }) }}
        </p>
        <p v-if="requiresReport && scanReportUnavailable" role="status">
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
        <h3>
          {{
            t(
              requiresReport
                ? hasMore
                  ? 'registryUi.skills.filters.moreCandidates'
                  : 'registryUi.skills.filters.noMatches'
                : 'registryUi.skills.noMatches',
            )
          }}
        </h3>
        <p>
          {{
            t(
              requiresReport
                ? hasMore
                  ? 'registryUi.skills.filters.moreCandidatesHelp'
                  : 'registryUi.skills.filters.noMatchesHelp'
                : 'registryUi.skills.noMatchesHelp',
            )
          }}
        </p>
        <button
          type="button"
          class="button button--secondary"
          @click="requiresReport ? clearFilters() : (query = '')"
        >
          {{
            t(requiresReport ? 'registryUi.skills.filters.reset' : 'registryUi.skills.clearSearch')
          }}
        </button>
      </div>
      <div v-if="scanReportUnavailable" class="catalog-more">
        <button
          type="button"
          class="button button--secondary"
          :disabled="loadingMore"
          @click="retry"
        >
          {{ t(loadingMore ? 'registryUi.skills.loading' : 'registryUi.skills.retry') }}
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
