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
  urls: { v1: string; v2: string; v3: string; cohort: string },
) {
  let client = clients.get(app);
  if (!client) {
    client = createSkillSpectorClient(urls.v2, urls.v1, fetch, urls.v3, urls.cohort);
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
  let cohortExpiryTimer: ReturnType<typeof setTimeout> | undefined;
  const clearCohortExpiry = () => {
    if (cohortExpiryTimer !== undefined) clearTimeout(cohortExpiryTimer);
    cohortExpiryTimer = undefined;
  };
  const getClient = () => sharedSkillSpectorClient(app, {
    v1: String(config.public.skillspectorReportsUrl),
    v2: String(config.public.skillspectorManifestUrl),
    v3: String(config.public.skillspectorV3ManifestUrl),
    cohort: String(config.public.skillspectorCohortLatestUrl),
  });
  const load = async (repository: string, current: number, retryFailed = false) => {
    clearCohortExpiry();
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
        // A signed Complete must stop being displayed when its 24-hour
        // authenticated snapshot expires, even if this tab stays open.
        const expiries = result.records
          .filter((record) => record.signed_cohort_verified === true)
          .map((record) => record.signed_cohort_expires_at)
          .filter((value): value is number => typeof value === 'number' && Number.isFinite(value));
        if (expiries.length) {
          cohortExpiryTimer = setTimeout(() => {
            if (mounted.value && current === generation) void load(repository, ++generation);
          }, Math.max(0, Math.min(...expiries) - Date.now() + 100));
        }
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
      clearCohortExpiry();
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
    mounted.value = false;
    generation++;
    clearCohortExpiry();
  });
  return { report, status, recoverable, retry };
}
