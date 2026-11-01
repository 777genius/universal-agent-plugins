export interface SkillDocument {
  status: 'resolved' | 'selection_required' | 'not_found' | 'incomplete' | 'unavailable';
  source: string;
  markdown?: string;
  sha256?: string;
  commit?: string;
  hasResources?: boolean;
}

// Document resolution is independent of the catalog's bounded search result
// window, so a shared detail URL can load without a previous card click.
export function useSkillDocument(id: Ref<string>, name: Ref<string>) {
  const config = useRuntimeConfig();
  const document = shallowRef<SkillDocument>();
  const loading = ref(false);
  const failed = ref(false);
  let current: AbortController | undefined;
  let generation = 0;

  async function load() {
    const request = ++generation;
    current?.abort();
    document.value = undefined;
    failed.value = false;
    if (!id.value || !name.value) {
      loading.value = false;
      return;
    }
    const controller = new AbortController();
    current = controller;
    loading.value = true;
    try {
      const catalog = String(config.public.skillsCatalogEndpoint);
      const endpoint = catalog.replace(/\/v1\/skills\/?$/, '/v1/skills/document');
      if (endpoint === catalog) throw new Error('Invalid Skills endpoint');
      const params = new URLSearchParams({ id: id.value, name: name.value });
      const response = await fetch(endpoint + '?' + params, {
        signal: controller.signal,
        headers: { Accept: 'application/json' },
      });
      if (!response.ok) throw new Error('Skill document unavailable');
      const result = (await response.json()) as SkillDocument;
      if (
        !['resolved', 'selection_required', 'not_found', 'incomplete', 'unavailable'].includes(result.status) ||
        result.status === 'resolved' &&
          (typeof result.markdown !== 'string' || result.markdown.length > 65536)
      ) throw new Error('Invalid Skill document');
      if (request === generation) document.value = result;
    } catch {
      if (request === generation && !controller.signal.aborted) failed.value = true;
    } finally {
      if (request === generation) loading.value = false;
    }
  }

  onMounted(() => { void load(); });
  watch([id, name], () => { void load(); });
  onScopeDispose(() => {
    generation++;
    current?.abort();
  });
  return { document, loading, failed, retry: load };
}
