import type { ThemeInstance } from 'vuetify';
import { createBrowserThemeAdapter } from '~/composables/useBrowserTheme';
import { useThemeStore } from '~/stores/theme';

export default defineNuxtPlugin({
  name: 'init-theme-locale',
  dependsOn: ['vuetify'],
  setup(nuxtApp) {
    const themeStore = useThemeStore();
    const { initLocale } = useLocation();
    let dispose: (() => void) | undefined;
    let stopped = false;
    const cleanup = () => {
      stopped = true;
      dispose?.();
    };
    nuxtApp.vueApp.onUnmount(cleanup);
    if (import.meta.hot) import.meta.hot.dispose(cleanup);

    // app:mounted can precede async NuxtLayout hydration. onNuxtReady waits
    // for app:suspense:resolve, so existing SSR nodes observe the theme change.
    onNuxtReady(() => {
      if (stopped) return;
      try {
        dispose = createBrowserThemeAdapter(themeStore, nuxtApp.$vuetifyTheme as ThemeInstance, window);
      } finally {
        initLocale();
      }
    });
  },
});
