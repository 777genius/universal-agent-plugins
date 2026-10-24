<script setup lang="ts">
import type { SkillsView } from '~/types/skillsCatalog';
import { restoreSkillsQuery, skillsCatalogQuery } from '~/utils/skillsCatalog';
import { canonicalPath } from '~/utils/seo';

defineProps<{ hideHeading?: boolean }>();

const { t, n, d } = useI18n({ useScope: 'parent' });
const route = useRoute();
const router = useRouter();
const initial = restoreSkillsQuery(route.query);
const query = ref(initial.query);
const view = ref<SkillsView>(initial.view);
let syncing = false;
let disposed = false;
let pending: string | undefined;
const fingerprint = () => JSON.stringify({ query: query.value.trim(), view: view.value });
watch(
  () => route.query,
  () => {
    const restored = restoreSkillsQuery(route.query);
    if (JSON.stringify(restored) === pending) return;
    query.value = restored.query;
    view.value = restored.view;
  },
);
async function syncQuery() {
  if (syncing) return;
  syncing = true;
  try {
    while (!disposed) {
      if (fingerprint() === JSON.stringify(restoreSkillsQuery(route.query))) break;
      pending = fingerprint();
      const failure = await router.replace({
        path: canonicalPath(route.path),
        query: skillsCatalogQuery(query.value, view.value, route.query),
        hash: route.hash,
      });
      if (failure) break;
    }
  } finally {
    pending = undefined;
    syncing = false;
  }
}
watch([query, view], () => {
  void syncQuery();
});
onScopeDispose(() => {
  disposed = true;
});
const {
  displayed,
  metadata,
  loading,
  loadingMore,
  failed,
  hasMore,
  searchTooShort,
  showMore,
  retry,
} = useSkillsCatalog(query, view);
const viewOptions = computed(() =>
  (['popular', 'trending', 'hot'] as const).map((value) => ({
    value,
    label: t(`registryUi.skills.views.${value}`),
  })),
);
const collectionTotal = computed(() => {
  const total = metadata.value?.total;
  return typeof total === 'number' &&
    Number.isSafeInteger(total) &&
    total > 0 &&
    total >= displayed.value.length
    ? total
    : undefined;
});
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
    aria-labelledby="skills-catalog-title"
    :aria-busy="loading"
  >
    <div class="section-heading" :class="{ 'sr-only': hideHeading }">
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
    </div>
    <div class="catalog-meta">
      <div aria-live="polite">
        <div class="skills-catalog__summary">
          <p v-if="query.trim() && !searchTooShort" class="catalog-count">
            {{ t('registryUi.skills.searchResults', { count: n(displayed.length) }) }}
          </p>
          <p v-else-if="!loading && !searchTooShort && displayed.length" class="catalog-count">
            {{
              collectionTotal
                ? t('registryUi.skills.collectionShown', {
                    count: n(displayed.length),
                    total: n(collectionTotal),
                  })
                : t('registryUi.skills.loaded', { count: n(displayed.length) })
            }}
          </p>
          <button
            v-if="!loading && !searchTooShort && (hasMore || (failed && displayed.length))"
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
            }}
            <span v-if="!loadingMore" aria-hidden="true">↓</span>
          </button>
        </div>
        <p class="skills-catalog__disclosure">{{ t('registryUi.skills.listingNote') }}</p>
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
        <h3>{{ t('registryUi.skills.noMatches') }}</h3>
        <p>{{ t('registryUi.skills.noMatchesHelp') }}</p>
        <button type="button" class="button button--secondary" @click="query = ''">
          {{ t('registryUi.skills.clearSearch') }}
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
