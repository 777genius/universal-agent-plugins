import { ref, unref, type Ref } from 'vue';
import { isPublishedLocale, type PublishedLocale } from '../data/i18n.ts';
import { useLocaleStore } from '../stores/locale.ts';
import { normalizeAppBase } from '../utils/localizedRoutes.ts';

/** A stored manual choice is metadata, never a redirect instruction. */
export function readManualLocaleCookie(read: () => string): PublishedLocale | null {
  try {
    const raw = read().split(';').map(item => item.trim()).find(item => item.startsWith('uap_locale='))?.slice('uap_locale='.length);
    const value = raw ? decodeURIComponent(raw) : null;
    return isPublishedLocale(value) ? value : null;
  } catch {
    return null;
  }
}

/** One transaction, shared by all switcher instances in the current Nuxt app. */
export async function switchLocaleTransaction(code: unknown, adapter: {
  valid: (code: unknown) => code is string;
  active: () => string;
  pending: { value: boolean };
  error: { value: boolean };
  destination: (code: string) => string | undefined;
  navigate: (path: string) => Promise<unknown>;
  remember: (code: string) => void;
  persist: (code: string) => void;
  track: (previous: string, next: string) => void;
}): Promise<boolean> {
  if (adapter.pending.value || !adapter.valid(code) || code === adapter.active()) return false;
  adapter.pending.value = true;
  adapter.error.value = false;
  const previous = adapter.active();
  try {
    const path = adapter.destination(code);
    if (!path) throw new Error('Missing localized route');
    const result = await adapter.navigate(path);
    if (result || adapter.active() !== code) throw new Error('Locale navigation did not complete');
    adapter.remember(code);
    try { adapter.persist(code); } catch { /* Cookie denial cannot undo navigation. */ }
    try { adapter.track(previous, code); } catch { /* Analytics is optional. */ }
    return true;
  } catch {
    adapter.error.value = true;
    return false;
  } finally {
    adapter.pending.value = false;
  }
}

export const useLocation = () => {
  const i18n = useNuxtApp().$i18n as { locale: string | Ref<string> };
  const route = useRoute();
  const router = useRouter();
  const switchLocalePath = useSwitchLocalePath();
  const store = useLocaleStore();
  const base = normalizeAppBase(useRuntimeConfig().app.baseURL);
  const pending = useState('locale:pending', () => false);
  const error = ref(false);
  const initialized = useState('locale:initialized', () => false);
  const { trackLanguageSwitch } = useAnalytics();
  const initLocale = () => {
    if (initialized.value || !import.meta.client) return;
    initialized.value = true;
    const value = readManualLocaleCookie(() => document.cookie);
    if (value) store.rememberChoice(value);
  };
  const switchLocale = (code: unknown) => switchLocaleTransaction(code, {
    valid: isPublishedLocale,
    active: () => unref(i18n.locale),
    pending,
    error,
    destination: target => {
      const path = switchLocalePath(target as PublishedLocale);
      if (!path) return undefined;
      // Nuxt resolves existing params; explicitly retain the complete query and hash.
      return router.resolve({ path: path.split(/[?#]/, 1)[0], query: { ...route.query }, hash: route.hash }).fullPath;
    },
    navigate: async path => await navigateTo(path),
    remember: target => store.rememberChoice(target),
    persist: target => {
      document.cookie = `uap_locale=${encodeURIComponent(target)}; Path=${base}; Max-Age=31536000; SameSite=Lax${location.protocol === 'https:' ? '; Secure' : ''}`;
    },
    track: trackLanguageSwitch,
  });
  return { initLocale, switchLocale, pending, error };
};
