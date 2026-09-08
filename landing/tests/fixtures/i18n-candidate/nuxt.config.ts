import vuetify from 'vite-plugin-vuetify';

export default defineNuxtConfig({
  compatibilityDate: '2026-09-08',
  devtools: { enabled: false },
  modules: ['@pinia/nuxt', '@nuxtjs/i18n', 'nuxt-icon'],
  app: { baseURL: process.env.UAP_CANDIDATE_BASE || '/' },
  build: { transpile: ['vuetify'] },
  vite: { plugins: [vuetify({ autoImport: true })] },
  i18n: {
    restructureDir: false,
    strategy: 'prefix_except_default',
    defaultLocale: 'en',
    detectBrowserLanguage: false,
    lazy: true,
    langDir: 'locales',
    locales: [
      { code: 'en', language: 'en-US', file: 'en.json' },
      { code: 'ru', language: 'ru-RU', file: 'ru.ts' },
      { code: 'uk', language: 'uk-UA', file: 'uk.ts' },
    ],
  },
});
