import type { SkillSpectorReport } from '~/types/skillSpector';
import { loadSkillSpectorReport } from '~/utils/skillSpector';

/** One bounded static-report request shared by both catalogs, independent of installation. */
export function useSkillSpector() {
  const config = useRuntimeConfig();
  const report = useState<SkillSpectorReport | undefined>('skillspector-report');
  const requested = useState('skillspector-requested', () => false);
  onMounted(() => {
    if (requested.value) return;
    requested.value = true;
    void (async () => {
      try {
        report.value = await loadSkillSpectorReport(String(config.public.skillspectorReportsUrl));
      } catch {
        // Unavailable and invalid assessments remain "not checked", never clean.
      }
    })();
  });
  return report;
}
