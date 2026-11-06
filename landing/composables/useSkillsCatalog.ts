import type { SkillsCatalogPage, SkillsView, SkillListing } from '~/types/skillsCatalog';
import type { SkillsFilters } from '~/utils/skillsCatalog';
import {
  hasCurrentPublishedScan,
  hasNoPublishedRemarks,
  publishedScanSubject,
} from '~/utils/skillSpectorCurrent';
import { sharedSkillSpectorClient } from '~/composables/useSkillSpector';
import type { SkillSpectorCoverageSummary } from '~/utils/skillSpectorCatalog';

/** Request generations isolate search/filter changes from late API or report responses. */
export function useSkillsCatalog(
  query: Ref<string>,
  view: Ref<SkillsView>,
  filters?: Ref<SkillsFilters>,
) {
  const config = useRuntimeConfig();
  const app = useNuxtApp();
  const items = shallowRef<SkillListing[]>([]);
  const pending = shallowRef<SkillListing[]>([]);
  const unavailable = shallowRef<SkillListing[]>([]);
  const metadata = shallowRef<SkillsCatalogPage>();
  const upstreamMore = ref(false);
  const loading = ref(true);
  const loadingMore = ref(false);
  const failed = ref(false);
  const scanReportUnavailable = ref(false);
  const scanOverview = shallowRef<SkillSpectorCoverageSummary>();
  const displayLimit = ref(48);
  let controller: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let generation = 0;
  let mounted = false;

  const searchTooShort = computed(() => {
    const length = [...query.value.trim()].length;
    return length > 0 && length < 2;
  });
  const requiresReport = computed(() =>
    Boolean(filters?.value.scannedOnly || filters?.value.noRemarksOnly),
  );
  const displayed = computed(() => items.value.slice(0, displayLimit.value));
  const remaining = computed(() => Math.max(0, items.value.length - displayLimit.value));
  const hasMore = computed(
    () => remaining.value > 0 || pending.value.length > 0 || upstreamMore.value,
  );
  const maxPagesPerLoad = 5;
  const maxCandidatesPerLoad = 64;

  /** A bounded acquisition window retains every queued candidate for continuation or retry. */
  async function load(append = false, retryEvidence = false) {
    if (searchTooShort.value) return;
    controller?.abort();
    controller = new AbortController();
    const signal = controller.signal;
    const request = ++generation;
    if (append) loadingMore.value = true;
    else loading.value = true;
    failed.value = false;
    const requireCurrentScan = requiresReport.value;
    const requireNoRemarks = Boolean(filters?.value.noRemarksOnly);
    const nextLimit = append ? displayed.value.length + 48 : 48;
    const collected = append ? [...items.value] : [];
    const queue = append ? [...pending.value] : [];
    const rejected = append && !retryEvidence ? [...unavailable.value] : [];
    if (append && retryEvidence) queue.unshift(...unavailable.value);
    const seen = new Set([...collected, ...queue, ...rejected].map((item) => item.id));
    const search = query.value.trim();
    let latest = append ? metadata.value : undefined;
    let pageNumber = append ? (metadata.value?.page ?? -1) + 1 : 0;
    let moreUpstream = !append || upstreamMore.value;
    let fetched = 0;
    let evaluated = 0;
    let reportUnavailable = false;
    const client = requireCurrentScan
      ? sharedSkillSpectorClient(app, {
          v1: String(config.public.skillspectorReportsUrl),
          v2: String(config.public.skillspectorManifestUrl),
          v3: String(config.public.skillspectorV3ManifestUrl),
          cohort: String(config.public.skillspectorCohortLatestUrl),
        })
      : undefined;
    if (retryEvidence) client?.retryFailedRequests();
    const summaryPromise = client?.coverageSummary().catch(() => {
      reportUnavailable = true;
      return undefined;
    });
    async function eligible(skill: SkillListing): Promise<'yes' | 'no' | 'unavailable'> {
      const subject = publishedScanSubject(skill);
      if (!subject || !client) return 'no';
      try {
        const report = await client.reportFor(subject.repository);
        return (
          requireNoRemarks
            ? hasNoPublishedRemarks(report, subject)
            : hasCurrentPublishedScan(report, subject)
        )
          ? 'yes'
          : 'no';
      } catch {
        return 'unavailable';
      }
    }
    function commit(overview?: SkillSpectorCoverageSummary) {
      items.value = collected;
      pending.value = queue;
      unavailable.value = rejected;
      metadata.value = latest;
      upstreamMore.value = moreUpstream;
      displayLimit.value = nextLimit;
      scanOverview.value = requireCurrentScan ? overview : undefined;
      scanReportUnavailable.value =
        requireCurrentScan && (reportUnavailable || !overview || rejected.length > 0);
    }
    try {
      // Catalog visibility is independent of scan evidence and install authority.
      const currentSourceIds = client
        ? await client.currentSourceStateIds().catch(() => {
            reportUnavailable = true;
            return undefined;
          })
        : undefined;
      if (request !== generation) return;
      while (collected.length < nextLimit && !signal.aborted) {
        if (requireCurrentScan && queue.length && evaluated < maxCandidatesPerLoad) {
          const batch = queue.splice(0, Math.min(8, maxCandidatesPerLoad - evaluated));
          const decisions = await Promise.all(batch.map(eligible));
          if (request !== generation) return;
          evaluated += batch.length;
          batch.forEach((item, index) => {
            if (decisions[index] === 'yes') collected.push(item);
            else if (decisions[index] === 'unavailable') rejected.push(item);
          });
          continue;
        }
        if (
          (requireCurrentScan && (queue.length || evaluated >= maxCandidatesPerLoad)) ||
          !moreUpstream ||
          fetched >= maxPagesPerLoad
        )
          break;
        const params = new URLSearchParams({
          view: search ? 'popular' : view.value,
          page: String(pageNumber),
        });
        if (search) params.set('q', search);
        const endpoint = String(config.public.skillsCatalogEndpoint);
        const response = await fetch(endpoint + (endpoint.includes('?') ? '&' : '?') + params, {
          signal,
          headers: { Accept: 'application/json' },
        });
        if (!response.ok) throw new Error('Catalog unavailable');
        const page = (await response.json()) as SkillsCatalogPage;
        if (
          !Array.isArray(page.items) ||
          typeof page.hasMore !== 'boolean' ||
          page.page !== pageNumber ||
          !page.cache
        )
          throw new Error('Invalid catalog');
        if (request !== generation) return;
        fetched++;
        pageNumber++;
        // Provider search supports page zero only; its candidate queue still paginates locally.
        moreUpstream = !search && page.hasMore && page.items.length > 0;
        for (const item of page.items) {
          if (
            !item ||
            typeof item !== 'object' ||
            typeof item.id !== 'string' || !item.id ||
            typeof item.title !== 'string' || !item.title ||
            typeof item.skillNameHint !== 'string' ||
            !item.source || !['github', 'external'].includes(item.source.kind) ||
            !item.metrics || !Number.isFinite(item.metrics.installs) ||
            !['all-time', '24h', '1h'].includes(item.metrics.window) ||
            seen.has(item.id)
          )
            continue;
          seen.add(item.id);
          if (requireCurrentScan) {
            const subject = publishedScanSubject(item);
            if (subject && (!currentSourceIds || currentSourceIds.has(subject.sourceStateId)))
              queue.push(item);
          } else collected.push(item);
        }
        latest = page;
      }
      const overview = await summaryPromise;
      if (request !== generation) return;
      commit(overview);
    } catch {
      const overview = await summaryPromise;
      if (request === generation && !signal.aborted) {
        commit(overview);
        failed.value = true;
      }
    } finally {
      if (request === generation) {
        loading.value = false;
        loadingMore.value = false;
      }
    }
  }
  function schedule() {
    ++generation;
    controller?.abort();
    if (timer) clearTimeout(timer);
    items.value = [];
    pending.value = [];
    unavailable.value = [];
    metadata.value = undefined;
    upstreamMore.value = false;
    failed.value = false;
    scanReportUnavailable.value = false;
    scanOverview.value = undefined;
    loadingMore.value = false;
    loading.value = !searchTooShort.value;
    displayLimit.value = 48;
    if (mounted && !searchTooShort.value)
      timer = setTimeout(() => {
        void load();
      }, 300);
  }
  watch(
    [query, view, () => filters?.value.scannedOnly, () => filters?.value.noRemarksOnly],
    schedule,
  );
  onMounted(() => {
    mounted = true;
    loading.value = !searchTooShort.value;
    void load();
  });
  onScopeDispose(() => {
    ++generation;
    controller?.abort();
    if (timer) clearTimeout(timer);
  });
  function showMore() {
    if (remaining.value) displayLimit.value += 48;
    else if ((pending.value.length || upstreamMore.value) && !loadingMore.value) void load(true);
  }
  return {
    displayed,
    items,
    metadata,
    loading,
    loadingMore,
    failed,
    remaining,
    hasMore,
    requiresReport,
    scanReportUnavailable,
    scanOverview,
    searchTooShort,
    showMore,
    retry: () => load(
      Boolean(metadata.value || items.value.length) &&
        (!requiresReport.value || Boolean(scanOverview.value)),
      true,
    ),
  };
}
