export default defineNuxtPlugin({
  name: "init-theme-locale",
  dependsOn: ["vuetify"],
  setup(nuxtApp) {
    const { initTheme } = useBrowserTheme();
    const { initLocale } = useLocation();

    // Run after hydration to avoid SSR/CSR mismatches.
    nuxtApp.hook("app:mounted", () => {
      try {
        initTheme();
      } catch {
        // Storage denial must not prevent locale preference initialization.
      } finally {
        initLocale();
      }
    });
  }
});
