import type { SkillSpectorReport } from '~/types/skillSpector';
import { loadSkillSpectorReport } from '~/utils/skillSpector';

/** One bounded static-report request shared by both catalogs, independent of installation. */
export function useSkillSpector() {
  const config = useRuntimeConfig();
  const report = useState<SkillSpectorReport | undefined>('skillspector-report');
  const status = useState<'loading' | 'ready' | 'unavailable'>(
    'skillspector-load-status',
    () => 'loading',
  );
  const requested = useState('skillspector-requested', () => false);
  onMounted(() => {
    if (requested.value) return;
    requested.value = true;
    void (async () => {
      try {
        report.value = await loadSkillSpectorReport(String(config.public.skillspectorReportsUrl));
        status.value = 'ready';
      } catch {
        status.value = 'unavailable';
      }
    })();
  });
  return { report, status };
}
