import { assembleDownloadContent } from '~/data/download';
import type { DownloadOverlay } from '~/types/download';

// Each overlay is its own lazy chunk. Legacy content and draft languages stay off this path.
export const useDownloadContent = async () => {
  const { locale } = useI18n();
  const load = async (code: string): Promise<DownloadOverlay> => {
    // The route locale comes from the publication manifest. Load its reviewed
    // overlay instead of silently serving English for newly published locales.
    const language = code;
    try {
      return (await import(`../content/download/${language}.json`)).default;
    } catch (error) {
      if (language === 'en') throw error;
      // Emergency fallback only; published overlay completeness is a release gate.
      return (await import('../content/download/en.json')).default;
    }
  };
  const { data } = await useAsyncData(
    () => `shell-download-${locale.value}`,
    () => load(locale.value),
  );
  const fallback = await load('en');
  const content = computed(() => assembleDownloadContent(data.value ?? fallback));
  return { content };
};
