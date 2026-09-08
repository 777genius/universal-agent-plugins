import reference from './en.json';

// Real i18n 9 lazy-loader rejection, controlled by Playwright; no translated copy.
export default defineI18nLocale(async locale => {
  if (import.meta.client) {
    const base = useRuntimeConfig().app.baseURL;
    await $fetch(`${base}fixture-messages/${locale}`, { retry: 0 });
  }
  return reference;
});
