export interface SkillDocument {
  status: 'resolved' | 'selection_required' | 'not_found' | 'incomplete' | 'unavailable';
  source: string;
  markdown?: string;
  sha256?: string;
  commit?: string;
  hasResources?: boolean;
}

const defaultRetrySeconds = 20;
const maxRetrySeconds = 30;

async function retryDelay(response: Response): Promise<number | undefined> {
  let retryAfter: unknown;
  try {
    retryAfter = ((await response.json()) as { retryAfter?: unknown }).retryAfter;
  } catch {
    // The header is not necessarily exposed by CORS, so use the API default.
  }
  if (retryAfter === undefined) return defaultRetrySeconds * 1000;
  if (typeof retryAfter !== 'number' || !Number.isInteger(retryAfter) || retryAfter < 1 || retryAfter > maxRetrySeconds) return undefined;
  return retryAfter * 1000;
}

function waitForRetry(signal: AbortSignal, delay: number): Promise<boolean> {
  return new Promise((resolve) => {
    if (signal.aborted) return resolve(false);
    const cancel = () => { clearTimeout(timer); resolve(false); };
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', cancel);
      resolve(true);
    }, delay);
    signal.addEventListener('abort', cancel, { once: true });
  });
}

// Document resolution is independent of the catalog's bounded search result
// window, so a shared detail URL can load without a previous card click.
export function useSkillDocument(id: Ref<string>, name: Ref<string>) {
  const config = useRuntimeConfig();
  const document = shallowRef<SkillDocument>();
  const loading = ref(false);
  const failed = ref(false);
  const waitingForRetry = ref(false);
  let current: AbortController | undefined;
  let generation = 0;

  async function load() {
    const request = ++generation;
    current?.abort();
    document.value = undefined;
    failed.value = false;
    waitingForRetry.value = false;
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
      const options = {
        signal: controller.signal,
        headers: { Accept: 'application/json' },
      };
      let response = await fetch(endpoint + '?' + params, options);
      if (response.status === 429) {
        const delay = await retryDelay(response);
        if (delay !== undefined && request === generation && !controller.signal.aborted) {
          waitingForRetry.value = true;
          const ready = await waitForRetry(controller.signal, delay);
          if (!ready || request !== generation) return;
          waitingForRetry.value = false;
          response = await fetch(endpoint + '?' + params, options);
        }
      }
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
      if (request === generation) {
        loading.value = false;
        waitingForRetry.value = false;
      }
    }
  }

  onMounted(() => { void load(); });
  watch([id, name], () => { void load(); });
  onScopeDispose(() => {
    generation++;
    current?.abort();
  });
  return { document, loading, failed, waitingForRetry, retry: load };
}
