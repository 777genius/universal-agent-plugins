<script setup lang="ts">
import { defaultDistribution, evidenceLabel, validationLabel } from '~/utils/registry'
const route = useRoute()
const registry = useRegistry()
const { pluginIcon, sourceUrl, repositoryUrl } = useSite()
const plugin = registry.plugins.find(item => item.name === route.params.slug)

if (!plugin) {
  throw createError({ statusCode: 404, statusMessage: 'Plugin not found' })
}
const distribution = defaultDistribution(plugin)
const authLabel = plugin.authentication === 'none' ? 'No account required' : plugin.authentication === 'oauth' ? 'OAuth required' : plugin.authentication === 'client_managed' ? 'Client-managed authentication' : 'Check package requirements'

const canonical = `${useRuntimeConfig().public.siteUrl}/plugins/${plugin.name}`
useSeoMeta({
  title: plugin.name,
  description: plugin.description,
  ogTitle: `${plugin.name} · Universal Agent Plugins`,
  ogDescription: plugin.description,
  ogType: 'website',
})
useHead({ link: [{ rel: 'canonical', href: canonical }] })
</script>

<template>
  <div class="plugin-page container">
    <nav class="breadcrumbs" aria-label="Breadcrumb"><NuxtLink to="/plugins">Directory</NuxtLink><span aria-hidden="true">/</span><span aria-current="page">{{ plugin.name }}</span></nav>
    <div class="plugin-page__grid">
      <article class="plugin-profile">
        <div class="plugin-profile__heading">
          <span class="plugin-profile__icon"><img :src="pluginIcon(plugin)" alt="" width="54" height="54" /></span>
          <div><div class="plugin-profile__meta"><span class="source-pill">Default source · {{ distribution.label }}</span><span>v{{ plugin.version }}</span></div><h1>{{ plugin.display_name }}</h1></div>
        </div>
        <p class="plugin-profile__description">{{ plugin.description }}</p>
        <dl class="plugin-facts">
          <div><dt>Default source</dt><dd>{{ distribution.id }} ({{ distribution.publisher }})</dd></div>
          <div><dt>License</dt><dd>{{ plugin.license || 'Not specified' }}</dd></div>
          <div><dt>Authentication</dt><dd>{{ authLabel }}</dd></div>
          <div><dt>Immutable revision</dt><dd><code>{{ distribution.source.revision }}</code></dd></div>
          <div><dt>Provenance</dt><dd><a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">View package source <span aria-hidden="true">↗</span></a></dd></div>
        </dl>
        <div class="plugin-profile__section"><h2>Components</h2><ul class="badge-list"><li v-for="component in plugin.components" :key="component">{{ component }}</li></ul></div>
        <div v-if="plugin.categories.length" class="plugin-profile__section"><h2>Categories</h2><ul class="tag-list"><li v-for="category in plugin.categories" :key="category">{{ category }}</li></ul></div>
        <div class="plugin-profile__section">
          <h2>Sources and alternatives</h2>
          <ul class="distribution-list">
            <li v-for="item in plugin.distributions" :key="item.id"><strong>{{ item.id }}</strong> — {{ item.label }}<span v-if="item.id === plugin.default_distribution"> (Default)</span><br /><small>{{ item.source.repository }}@{{ item.source.revision }}//{{ item.source.path }}</small></li>
          </ul>
        </div>
        <div class="status-card">
          <span class="validation-badge"><span>✓</span> {{ validationLabel(plugin) }}</span>
          <ul v-if="plugin.evidence.length" class="evidence-list">
            <li v-for="item in plugin.evidence" :key="`${item.client}-${item.level}`"><strong>{{ item.client }}: {{ evidenceLabel(item) }}</strong><span v-if="item.client_version || item.os || item.architecture || item.tested_at"> — {{ [item.client_version, item.os, item.architecture, item.tested_at].filter(Boolean).join(' · ') }}</span><span v-else> — legacy evidence record; open the report for the exact applicable environment</span> <a v-if="item.evidence_url?.startsWith('https://')" :href="item.evidence_url" target="_blank" rel="noreferrer">Evidence ↗</a></li>
          </ul>
          <p v-else>No client runtime or OAuth evidence is recorded for this exact release. Schema validation covers package structure only.</p>
          <a :href="`${repositoryUrl}/blob/main/docs/VERIFICATION.md`" target="_blank" rel="noreferrer">Read verification evidence →</a>
        </div>
      </article>
      <InstallPanel :plugin="plugin" />
    </div>
  </div>
</template>
