import { resolve } from 'node:path'
import { loadRegistryIndex } from './build/load-registry'

const signedSnapshotPath = process.env.UAP_SIGNED_SNAPSHOT_PATH
const previewPath = process.env.UAP_DIRECTORY_PREVIEW_PATH
const defaultRegistryPath = resolve(process.cwd(), '../registry/directory.json')
const implicitPreview = !signedSnapshotPath && !previewPath && !process.env.UAP_REGISTRY_PATH
const registryPath = signedSnapshotPath
  ? resolve(process.cwd(), signedSnapshotPath)
  : previewPath
    ? resolve(process.cwd(), previewPath)
    : process.env.UAP_REGISTRY_PATH
      ? resolve(process.cwd(), process.env.UAP_REGISTRY_PATH)
      : defaultRegistryPath
const registryIndex = loadRegistryIndex(registryPath, signedSnapshotPath ? 'published_snapshot' : (previewPath || implicitPreview) ? 'review_preview' : undefined)
if (!signedSnapshotPath && process.env.CI && process.env.GITHUB_EVENT_NAME !== 'pull_request') {
  throw new Error('Production builds require UAP_SIGNED_SNAPSHOT_PATH; unsigned data is allowed only for pull-request previews')
}

const siteUrl = (process.env.NUXT_PUBLIC_SITE_URL
  ?? 'https://777genius.github.io/universal-agent-plugins').replace(/\/$/, '')
const baseURL = process.env.NUXT_APP_BASE_URL ?? '/'
const repositoryUrl = 'https://github.com/777genius/universal-agent-plugins'
// Production HTML is finalized after prerendering so script-src contains the
// hashes of the exact Nuxt-generated inline scripts and style elements for
// each route. The finalizer also adds byte-exact hashes for Reka UI's two
// reviewed viewport rules. Runtime popover positioning needs style attributes;
// the scoped exception does not allow inline scripts or other style elements.
const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'none'; img-src 'self' data:; object-src 'none'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; upgrade-insecure-requests"
const productionMeta = process.env.NODE_ENV === 'production'
  ? [{ 'http-equiv': 'Content-Security-Policy', content: contentSecurityPolicy }]
  : []

export default defineNuxtConfig({
  compatibilityDate: '2026-08-10',
  ssr: true,
  devtools: { enabled: false },
  modules: ['@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  app: {
    baseURL,
    head: {
      htmlAttrs: { lang: 'en' },
      titleTemplate: '%s · Universal Agent Plugins',
      meta: [
        ...productionMeta,
        { name: 'referrer', content: 'strict-origin-when-cross-origin' },
      ],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: `${baseURL}logo.svg` },
      ],
      script: [{
        innerHTML: "try{document.documentElement.dataset.theme=localStorage.getItem('uap-theme')||((matchMedia('(prefers-color-scheme:light)').matches)?'light':'dark')}catch(e){}",
      }],
    },
  },
  runtimeConfig: {
    public: {
      registryIndex,
      siteUrl,
      baseURL,
      repositoryUrl,
    },
  },
  nitro: {
    compressPublicAssets: true,
    prerender: {
      crawlLinks: false,
      routes: [
        '/',
        '/plugins',
        '/robots.txt',
        '/sitemap.xml',
        ...registryIndex.plugins.map(plugin => `/plugins/${plugin.name}`),
      ],
    },
  },
  routeRules: {
    '/**': {
      prerender: true,
      headers: {
        'referrer-policy': 'strict-origin-when-cross-origin',
        'x-content-type-options': 'nosniff',
      },
    },
    '/_nuxt/**': { headers: { 'cache-control': 'public, max-age=31536000, immutable' } },
  },
  typescript: {
    strict: true,
    typeCheck: true,
  },
})
