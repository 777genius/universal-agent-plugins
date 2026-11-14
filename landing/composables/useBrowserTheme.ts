import { computed, watch } from 'vue';
import { useThemeStore } from '~/stores/theme';

// Installed once by the client plugin, after Nuxt's entire hydration completes.
// Components only read Pinia and dispatch actions; they own no browser effects.
export const createBrowserThemeAdapter = (
  themeStore: ReturnType<typeof useThemeStore>,
  vuetifyTheme: { change: (name: string) => void },
  browser: Pick<Window, 'localStorage' | 'matchMedia'>,
) => {
  try {
    themeStore.restoreTheme(browser.localStorage.getItem('theme'));
  } catch {
    // Both obtaining localStorage and reading it can be denied.
  }

  const stop = watch(
    () => [themeStore.current, themeStore.userSelected] as const,
    ([name, userSelected]) => {
      vuetifyTheme.change(name);
      if (userSelected) {
        try {
          browser.localStorage.setItem('theme', name);
        } catch {
          // Pinia and Vuetify still update when persistence is unavailable.
        }
      }
    },
    { immediate: true, flush: 'sync' },
  );

  // Preserve dark on initial system-light; only later system changes apply.
  let media: MediaQueryList | undefined;
  const onChange = (event: MediaQueryListEvent) => {
    if (!themeStore.userSelected) {
      themeStore.setTheme(event.matches ? 'dark' : 'light', false);
    }
  };
  if (!themeStore.userSelected) {
    media = browser.matchMedia('(prefers-color-scheme: dark)');
    media.addEventListener('change', onChange);
  }

  return () => {
    stop();
    media?.removeEventListener('change', onChange);
  };
};

export const useBrowserTheme = () => {
  const themeStore = useThemeStore();
  return {
    currentTheme: computed(() => themeStore.current),
    isDark: computed(() => themeStore.current === 'dark'),
    toggleTheme: () => themeStore.toggleTheme(),
  };
};
