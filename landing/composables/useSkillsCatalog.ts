import type { SkillsCatalogPage, SkillsView, SkillListing } from '~/types/skillsCatalog';

/** A request belongs to exactly one search/view; late responses cannot replace it. */
export function useSkillsCatalog(query: Ref<string>, view: Ref<SkillsView>) {
  const config = useRuntimeConfig();
  const items = shallowRef<SkillListing[]>([]);
  const metadata = shallowRef<SkillsCatalogPage>();
  const loading = ref(true);
  const loadingMore = ref(false);
  const failed = ref(false);
  const displayLimit = ref(48);
  let controller: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let generation = 0;
  let mounted = false;
  const searchTooShort = computed(() => {
    const length = [...query.value.trim()].length;
    return length > 0 && length < 2;
  });
  const displayed = computed(() => items.value.slice(0, displayLimit.value));
  const remaining = computed(() => Math.max(0, items.value.length - displayLimit.value));
  const hasMore = computed(() => remaining.value > 0 || Boolean(metadata.value?.hasMore));

  async function load(append = false) {
    if (searchTooShort.value) return;
    controller?.abort();
    controller = new AbortController();
    const request = ++generation;
    if (append) loadingMore.value = true;
    else loading.value = true;
    failed.value = false;
    const params = new URLSearchParams({
      view: view.value,
      page: String(append ? (metadata.value?.page ?? 0) + 1 : 0),
    });
    const search = query.value.trim();
    if (search) {
      params.set('q', search);
      params.set('view', 'popular');
    }
    try {
      const endpoint = String(config.public.skillsCatalogEndpoint);
      const response = await fetch(`${endpoint}${endpoint.includes('?') ? '&' : '?'}${params}`, {
        signal: controller.signal,
        headers: { Accept: 'application/json' },
      });
      if (!response.ok) throw new Error('Catalog unavailable');
      const page = (await response.json()) as SkillsCatalogPage;
      if (
        !Array.isArray(page.items) ||
        typeof page.hasMore !== 'boolean' ||
        !Number.isInteger(page.page) ||
        !page.cache
      )
        throw new Error('Invalid catalog');
      if (request !== generation) return;
      if (append) {
        const nextLimit = displayed.value.length + 48;
        const seen = new Set(items.value.map((item) => item.id));
        items.value = [...items.value, ...page.items.filter((item) => !seen.has(item.id))];
        displayLimit.value = nextLimit;
      } else {
        items.value = page.items;
        displayLimit.value = 48;
      }
      metadata.value = page;
    } catch {
      if (request === generation && !controller.signal.aborted) failed.value = true;
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
    metadata.value = undefined;
    failed.value = false;
    loadingMore.value = false;
    loading.value = !searchTooShort.value;
    displayLimit.value = 48;
    if (mounted && !searchTooShort.value)
      timer = setTimeout(() => {
        void load();
      }, 300);
  }
  watch([query, view], schedule);
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
    else if (metadata.value?.hasMore && !loadingMore.value) void load(true);
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
    searchTooShort,
    showMore,
    retry: () => load(Boolean(items.value.length)),
  };
}
