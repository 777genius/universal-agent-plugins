import { supportedLocales } from "~/data/i18n";
import { useLocaleStore } from "~/stores/locale";

export const useLocation = () => {
  const nuxtApp = useNuxtApp();
  const i18n = nuxtApp.$i18n as { locale?: { value: string }; setLocale?: (code: string) => void } | undefined;
  const localeStore = useLocaleStore();
  const cookie = useCookie("i18n_redirected", { default: () => "" });

  const getBrowserLocale = () => {
    if (!import.meta.client) return "en";
    const browserLocale = navigator.language || "en";
    const normalized = browserLocale.split("-")[0].toLowerCase();
    const supported: readonly string[] = supportedLocales.map((item) => item.code);
    return supported.includes(normalized) ? normalized : "en";
  };

  const initLocale = () => {
    // Localized routes are not published yet. Keep the locale resolved from the
    // current route instead of sending first-time visitors to a browser-derived
    // path that GitHub Pages cannot serve.
    const currentLocale = i18n?.locale?.value || "en";
    localeStore.setLocale(currentLocale, false);
    cookie.value = currentLocale;
  };

  return { initLocale, getBrowserLocale };
};
