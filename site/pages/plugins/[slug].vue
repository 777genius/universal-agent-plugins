<script setup lang="ts">
import { defaultDistribution, deliveryLabel, evidenceLabel, validationLabel } from '~/utils/registry'
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
          <div><div class="plugin-profile__meta"><span class="source-pill">{{ plugin.installable ? 'Default source' : 'Unavailable provenance' }} · {{ distribution.label }}</span><span>v{{ plugin.version }}</span></div><h1>{{ plugin.display_name }}</h1></div>
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
            <li v-for="item in plugin.distributions" :key="item.id"><strong>{{ item.id }}</strong> — {{ item.label }}<span v-if="item.id === plugin.default_distribution"> (Selected default)</span><span v-else-if="item.id === plugin.declared_default_distribution"> (Declared default)</span> · {{ item.status }} / {{ item.release_status }}<br /><small>{{ item.source.repository }}@{{ item.source.revision }}//{{ item.source.path }}</small><ul><li v-for="target in item.targets" :key="target.client">{{ target.client }} — {{ deliveryLabel(target.delivery) }}; scopes: {{ target.scopes.join(', ') }}<template v-if="target.app_binding">; app key <code>{{ target.app_binding.app_key }}</code>, app ID <code>{{ target.app_binding.id }}</code>, MCP server <code>{{ target.app_binding.mcp_server }}</code></template></li></ul></li>
          </ul>
        </div>
        <div class="status-card">
          <span class="validation-badge"><span>✓</span> {{ validationLabel(plugin) }}</span>
          <h3>Package evidence</h3>
          <ul v-if="plugin.package_evidence.length" class="evidence-list">
            <li v-for="item in plugin.package_evidence" :key="item.id">
              <strong>{{ evidenceLabel(item) }}</strong><span v-if="item.tested_at"> — {{ item.tested_at }}</span><br />
              <small>Evidence ID <code>{{ item.id }}</code><br />Package <code>{{ item.package_tree_digest }}</code><br />Artifact <code>{{ item.artifact.repository }}@{{ item.artifact.revision }}//{{ item.artifact.path }}</code><br />Artifact digest <code>{{ item.artifact.digest }}</code></small>
              <a :href="item.artifact.url" target="_blank" rel="noreferrer">Exact evidence ↗</a>
            </li>
          </ul>
          <p v-else>No package-level schema evidence is selected for this exact release.</p>
          <h3>Client evidence</h3>
          <ul v-if="plugin.evidence.length" class="evidence-list">
            <li v-for="item in plugin.evidence" :key="item.id"><strong>{{ item.client }}: {{ evidenceLabel(item) }}</strong><span v-if="item.client_version || item.os || item.architecture || item.tested_at"> — {{ [item.client_version, item.os, item.architecture, item.installer_version && `installer ${item.installer_version}`, item.dependency_identity, item.tested_at].filter(Boolean).join(' · ') }}</span><span v-else> — legacy evidence record; open the report for the exact applicable environment</span><template v-if="item.package_tree_digest && item.artifact"><br /><small>Evidence ID <code>{{ item.id }}</code><br />Package <code>{{ item.package_tree_digest }}</code><br />Artifact <code>{{ item.artifact.repository }}@{{ item.artifact.revision }}//{{ item.artifact.path }}</code><br />Artifact digest <code>{{ item.artifact.digest }}</code></small> <a :href="item.artifact.url" target="_blank" rel="noreferrer">Exact evidence ↗</a></template></li>
          </ul>
          <p v-else>No client materialization, discovery, runtime, or OAuth evidence is selected for this exact release.</p>
          <a :href="`${repositoryUrl}/blob/main/docs/VERIFICATION.md`" target="_blank" rel="noreferrer">Read verification evidence →</a>
        </div>
      </article>
      <InstallPanel :plugin="plugin" />
    </div>
  </div>
</template>
