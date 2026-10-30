import type { MaybeRefOrGetter } from 'vue';
import type { SkillSpectorDisplayReport, SkillSpectorSubject } from '~/types/skillSpector';
import { createSkillSpectorClient, isTransientSkillSpectorError } from '~/utils/skillSpectorCatalog';

const clients = new WeakMap<object, ReturnType<typeof createSkillSpectorClient>>();

/** Shared immutable report cache, with loading/failure state local to the requested repository. */
export function useSkillSpector(subject: MaybeRefOrGetter<SkillSpectorSubject | undefined>) {
  const config = useRuntimeConfig();
  const app = useNuxtApp();
  const report = shallowRef<SkillSpectorDisplayReport>();
  const status = ref<'loading' | 'ready' | 'unavailable'>('loading');
  const recoverable = ref(false);
  const mounted = ref(false);
  let generation = 0;
  const getClient = () => {
    let client = clients.get(app);
    if (!client) {
      client = createSkillSpectorClient(
        String(config.public.skillspectorManifestUrl),
        String(config.public.skillspectorReportsUrl),
        fetch,
        String(config.public.skillspectorV3ManifestUrl),
      );
      clients.set(app, client);
    }
    return client;
  };
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
