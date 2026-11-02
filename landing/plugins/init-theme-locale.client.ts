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

    // app:mounted can precede async NuxtLayout hydration. Apply preferences
    // when it completes; waiting for browser idle can leave the saved theme unapplied.
    const initialize = () => {
      if (stopped) return;
      try {
        dispose = createBrowserThemeAdapter(themeStore, nuxtApp.$vuetifyTheme as ThemeInstance, window);
      } finally {
        initLocale();
      }
    };
    if (nuxtApp.isHydrating) {
      nuxtApp.hooks.hookOnce('app:suspense:resolve', initialize);
    } else {
      initialize();
    }
  },
});
