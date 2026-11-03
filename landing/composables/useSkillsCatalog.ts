import type { SkillsCatalogPage, SkillsView, SkillListing } from '~/types/skillsCatalog';
import { skillInstallCommand } from '~/utils/skillsCatalog';
import { hasCurrentPublishedScan, publishedScanSubject } from '~/utils/skillSpectorCurrent';
import { sharedSkillSpectorClient } from '~/composables/useSkillSpector';
import type { SkillSpectorCoverageSummary } from '~/utils/skillSpectorCatalog';

export type SkillsScanView = 'scanned' | 'all';

/** Request generations isolate search/view/filter changes from late API or scan responses. */
export function useSkillsCatalog(
  query: Ref<string>,
  view: Ref<SkillsView>,
  scanView?: Ref<SkillsScanView>,
) {
  const config = useRuntimeConfig();
  const app = useNuxtApp();
  const items = shallowRef<SkillListing[]>([]);
  const pending = shallowRef<SkillListing[]>([]);
  const metadata = shallowRef<SkillsCatalogPage>();
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
  const scannedOnly = computed(() => scanView?.value === 'scanned' && !query.value.trim());
  const displayed = computed(() => items.value.slice(0, displayLimit.value));
  const remaining = computed(() => Math.max(0, items.value.length - displayLimit.value));
  const hasMore = computed(() =>
    remaining.value > 0 ||
    (!query.value.trim() && (pending.value.length > 0 || Boolean(metadata.value?.hasMore))),
  );

  // Keep bounded acquisition separate from the scan queue. The published report
  // and its SHA-256 verified shards are shared with the card's detail viewer.
  const maxPagesPerLoad = 5;
  const maxCandidatesPerLoad = 64;
  /** Load one bounded window, retaining candidates not yet verified across pagination. */
  async function load(append = false) {
    if (searchTooShort.value) return;
    controller?.abort();
    controller = new AbortController();
    const signal = controller.signal;
    const request = ++generation;
    if (append) loadingMore.value = true;
    else loading.value = true;
    failed.value = false;
    if (!append) scanReportUnavailable.value = false;
    let reportUnavailable = false;

    const requireCurrentScan = scannedOnly.value;
    const nextLimit = append ? displayed.value.length + 48 : 48;
    const collected: SkillListing[] = append ? [...items.value] : [];
    const queue: SkillListing[] = append && requireCurrentScan ? [...pending.value] : [];
    const seen = new Set([...collected, ...queue].map((item) => item.id));
    const search = query.value.trim();
    let latest = append ? metadata.value : undefined;
    let pageNumber = append ? (metadata.value?.page ?? -1) + 1 : 0;
    let moreUpstream = !append || Boolean(metadata.value?.hasMore);
    let fetched = 0;
    let evaluated = 0;

    const client = requireCurrentScan
      ? sharedSkillSpectorClient(app, {
          v1: String(config.public.skillspectorReportsUrl),
          v2: String(config.public.skillspectorManifestUrl),
          v3: String(config.public.skillspectorV3ManifestUrl),
        })
      : undefined;
    /** Match a candidate to inspected, current publication evidence; fail closed on errors. */
    const summaryPromise = client?.coverageSummary().catch(() => {
      reportUnavailable = true;
      return undefined;
    });
    async function eligible(skill: SkillListing) {
      const subject = publishedScanSubject(skill);
      if (!subject || !client) return false;
      try {
        const report = await client.reportFor(subject.repository);
        return hasCurrentPublishedScan(report, subject);
      } catch {
        // Missing, malformed and unavailable evidence all fail closed.
        // The user may still explicitly choose the unfiltered catalog.
        reportUnavailable = true;
        return false;
      }
    }

    try {
      while (collected.length < nextLimit && !signal.aborted) {
        if (requireCurrentScan && queue.length && evaluated < maxCandidatesPerLoad) {
          const batch = queue.splice(0, Math.min(8, maxCandidatesPerLoad - evaluated));
          const decisions = await Promise.all(batch.map(eligible));
          if (request !== generation) return;
          evaluated += batch.length;
          batch.forEach((item, index) => {
            if (decisions[index]) collected.push(item);
          });
          continue;
        }
        if ((requireCurrentScan && (queue.length || evaluated >= maxCandidatesPerLoad)) ||
            !moreUpstream || fetched >= maxPagesPerLoad) break;

        const params = new URLSearchParams({
          view: search ? 'popular' : view.value,
          page: String(pageNumber),
        });
        if (search) params.set('q', search);
        const endpoint = String(config.public.skillsCatalogEndpoint);
        const response = await fetch(
          endpoint + (endpoint.includes('?') ? '&' : '?') + params,
          { signal, headers: { Accept: 'application/json' } },
        );
        if (!response.ok) throw new Error('Catalog unavailable');
        const page = (await response.json()) as SkillsCatalogPage;
        if (!Array.isArray(page.items) || typeof page.hasMore !== 'boolean' ||
            page.page !== pageNumber || !page.cache) throw new Error('Invalid catalog');
        if (request !== generation) return;
        fetched++;
        pageNumber++;
        moreUpstream = !search && page.hasMore && page.items.length > 0;
        for (const item of page.items) {
          if (!item || typeof item !== 'object' || typeof item.id !== 'string') continue;
          if (seen.has(item.id) || !skillInstallCommand(item)) continue;
          seen.add(item.id);
          if (requireCurrentScan) queue.push(item);
          else collected.push(item);
        }
        latest = page;
      }
      const overview = await summaryPromise;
      if (request !== generation) return;
      items.value = collected;
      scanOverview.value = requireCurrentScan ? overview : undefined;
      if (requireCurrentScan) scanReportUnavailable.value ||= reportUnavailable;
      pending.value = requireCurrentScan ? queue : [];
      displayLimit.value = nextLimit;
      metadata.value = latest;
    } catch {
      if (request === generation && !signal.aborted) failed.value = true;
    } finally {
      if (request === generation) {
        loading.value = false;
        loadingMore.value = false;
      }
    }
  }
  /** Invalidate stale async work before debouncing a new query, sort or scan mode. */
  function schedule() {
    ++generation;
    controller?.abort();
    if (timer) clearTimeout(timer);
    items.value = [];
    pending.value = [];
    metadata.value = undefined;
    failed.value = false;
    scanReportUnavailable.value = false;
    scanOverview.value = undefined;
    loadingMore.value = false;
    loading.value = !searchTooShort.value;
    displayLimit.value = 48;
    if (mounted && !searchTooShort.value)
      timer = setTimeout(() => { void load(); }, 300);
  }
  watch([query, view, () => scanView?.value], schedule);
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
  /** Reveal buffered results before requesting another bounded acquisition window. */
  function showMore() {
    if (remaining.value) displayLimit.value += 48;
    else if ((pending.value.length || metadata.value?.hasMore) && !loadingMore.value)
      void load(true);
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
    scannedOnly,
    scanReportUnavailable,
    scanOverview,
    searchTooShort,
    showMore,
    retry: () => load(!query.value.trim() && Boolean(items.value.length)),
  };
}
