import type { MaybeRefOrGetter } from 'vue';
import type { SkillSpectorDisplayReport, SkillSpectorSubject } from '~/types/skillSpector';
import {
  createSkillSpectorClient,
  isTransientSkillSpectorError,
} from '~/utils/skillSpectorCatalog';

const clients = new WeakMap<object, ReturnType<typeof createSkillSpectorClient>>();

/** One content-digest-validated client per Nuxt app. */
export function sharedSkillSpectorClient(
  app: object,
  urls: { v1: string; v2: string; v3: string },
) {
  let client = clients.get(app);
  if (!client) {
    client = createSkillSpectorClient(urls.v2, urls.v1, fetch, urls.v3);
    clients.set(app, client);
  }
  return client;
}

/** Shared immutable report cache, with loading/failure state local to the requested repository. */
export function useSkillSpector(subject: MaybeRefOrGetter<SkillSpectorSubject | undefined>) {
  const config = useRuntimeConfig();
  const app = useNuxtApp();
  const report = shallowRef<SkillSpectorDisplayReport>();
  const status = ref<'loading' | 'ready' | 'unavailable'>('loading');
  const recoverable = ref(false);
  const mounted = ref(false);
  let generation = 0;
  const getClient = () => sharedSkillSpectorClient(app, {
    v1: String(config.public.skillspectorReportsUrl),
    v2: String(config.public.skillspectorManifestUrl),
    v3: String(config.public.skillspectorV3ManifestUrl),
  });
  const load = async (repository: string, current: number, retryFailed = false) => {
    report.value = undefined;
    recoverable.value = false;
    status.value = 'loading';
    try {
      const client = getClient();
      const result = await (retryFailed
        ? client.retryReportFor(repository)
        : client.reportFor(repository));
      if (current === generation) {
        report.value = result;
        status.value = 'ready';
      }
    } catch (error) {
      if (current === generation) {
        recoverable.value = isTransientSkillSpectorError(error);
        status.value = 'unavailable';
      }
    }
  };
  const retry = () => {
    const repository = toValue(subject)?.repository.toLowerCase();
    if (!mounted.value || !repository || status.value !== 'unavailable' || !recoverable.value)
      return;
    return load(repository, ++generation, true);
  };
  onMounted(() => {
    mounted.value = true;
  });
  watch(
    [mounted, () => toValue(subject)?.repository.toLowerCase()],
    ([ready, repository]: [boolean, string | undefined]) => {
      if (!ready) return;
      const current = ++generation;
      if (!repository) {
        report.value = undefined;
        recoverable.value = false;
        status.value = 'ready';
        return;
      }
      void load(repository, current);
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    generation++;
  });
  return { report, status, recoverable, retry };
}
