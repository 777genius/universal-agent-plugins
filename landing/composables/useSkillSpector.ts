import type { MaybeRefOrGetter } from 'vue';
import type { SkillSpectorDisplayReport, SkillSpectorSubject } from '~/types/skillSpector';
import { createSkillSpectorClient } from '~/utils/skillSpectorCatalog';

const clients = new WeakMap<object, ReturnType<typeof createSkillSpectorClient>>();

/** Shared immutable report cache, with loading/failure state local to the requested repository. */
export function useSkillSpector(subject: MaybeRefOrGetter<SkillSpectorSubject | undefined>) {
  const config = useRuntimeConfig();
  const app = useNuxtApp();
  const report = shallowRef<SkillSpectorDisplayReport>();
  const status = ref<'loading' | 'ready' | 'unavailable'>('loading');
  const mounted = ref(false);
  let generation = 0;
  onMounted(() => {
    mounted.value = true;
  });
  watch(
    [mounted, () => toValue(subject)?.repository.toLowerCase()],
    async ([ready, repository]: [boolean, string | undefined]) => {
      if (!ready) return;
      const current = ++generation;
      report.value = undefined;
      if (!repository) {
        status.value = 'ready';
        return;
      }
      status.value = 'loading';
      let client = clients.get(app);
      if (!client) {
        client = createSkillSpectorClient(
          String(config.public.skillspectorManifestUrl),
          String(config.public.skillspectorReportsUrl),
        );
        clients.set(app, client);
      }
      try {
        const result = await client.reportFor(repository);
        if (current === generation) {
          report.value = result;
          status.value = 'ready';
        }
      } catch {
        if (current === generation) status.value = 'unavailable';
      }
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    generation++;
  });
  return { report, status };
}
