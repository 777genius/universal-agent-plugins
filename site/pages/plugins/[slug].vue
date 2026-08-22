<script setup lang="ts">
import type { ClientID } from '~/types/registry'
import { authenticationLabel, deliveryLabel, evidenceLabel, githubSourceUrl, resolveDistribution, targetAuthenticationLabel, validationLabel } from '~/utils/registry'
const route = useRoute()
const registry = useRegistry()
const { current, expired, published } = useDirectoryStatus()
const { pluginIcon, repositoryUrl } = useSite()
const plugin = registry.plugins.find(item => item.name === route.params.slug)

if (!plugin) {
  throw createError({ statusCode: 404, statusMessage: 'Plugin not found' })
}
const availableClients = clients.filter(client => plugin.client_support.clients.includes(client.id))
const initialTarget = availableClients.find(client => client.id === 'cursor')?.id ?? availableClients[0]?.id
const targets = ref<ClientID[]>(initialTarget ? [initialTarget] : [])
const resolution = computed(() => resolveDistribution(plugin, targets.value))
const installCandidate = computed(() => current.value ? resolution.value.distribution : undefined)
const authLabel = computed(() => authenticationLabel(installCandidate.value, targets.value, plugin.authentication))

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
          <div><div class="plugin-profile__meta"><span class="source-pill">{{ installCandidate ? 'Install candidate' : expired ? 'Stale Directory · history only' : !published ? 'Review preview · history only' : 'Unavailable provenance' }}<template v-if="installCandidate"> · {{ installCandidate.label }}</template></span><span v-if="installCandidate">v{{ installCandidate.version }} · release {{ installCandidate.release_sequence }}</span></div><h1>{{ plugin.display_name }}</h1></div>
        </div>
        <p class="plugin-profile__description">{{ plugin.description }}</p>
        <dl v-if="installCandidate" class="plugin-facts">
          <div><dt>Install source</dt><dd>{{ installCandidate.id }} ({{ installCandidate.publisher }})</dd></div>
          <div><dt>License</dt><dd>{{ plugin.license || 'Not specified' }}</dd></div>
          <div><dt>Authentication</dt><dd>{{ authLabel }}</dd></div>
          <div><dt>Immutable revision</dt><dd><code>{{ installCandidate.source.revision }}</code></dd></div>
          <div><dt>Provenance</dt><dd><a :href="githubSourceUrl(plugin, installCandidate)" target="_blank" rel="noreferrer">View exact install package source <span aria-hidden="true">↗</span></a></dd></div>
        </dl>
        <p v-if="resolution.fallback_reason && current"><strong>Fallback reason:</strong> {{ resolution.fallback_reason }}</p>
        <p v-if="expired">This signed Directory snapshot is stale. Install-candidate claims and commands are disabled; the entries below remain available as product history.</p>
        <p v-else-if="!published">This is unresolved review data, not installation authority. Install-candidate claims and commands are disabled; production uses a published signed Directory snapshot.</p>
        <p v-else-if="!installCandidate">{{ resolution.unavailable_reason }} The entries below are product history, not install candidates.</p>
        <div v-if="installCandidate" class="plugin-profile__section"><h2>Install candidate components</h2><ul class="badge-list"><li v-for="component in installCandidate.components" :key="component">{{ component }}</li></ul></div>
        <div v-if="plugin.categories.length" class="plugin-profile__section"><h2>Categories</h2><ul class="tag-list"><li v-for="category in plugin.categories" :key="category">{{ category }}</li></ul></div>
        <div class="plugin-profile__section">
          <h2>Product release history</h2>
          <p>Historical records below are not the selected install candidate.</p>
          <ul class="distribution-list">
            <li v-for="item in plugin.distributions" :key="item.id"><strong>{{ item.id }}</strong> — {{ item.label }}<span v-if="item.id === plugin.declared_default_distribution"> (Declared default source)</span><ul><li v-for="release in item.releases" :key="release.release_sequence"><strong>Historical release {{ release.release_sequence }} · v{{ release.version }}</strong> — {{ item.status }} / {{ release.release_status }}<br /><small>{{ release.source.repository }}@{{ release.source.revision }}//{{ release.source.path }}</small><ul><li v-for="target in release.targets" :key="target.client">{{ target.client }} — {{ deliveryLabel(target.delivery) }}; {{ targetAuthenticationLabel(target.authentication) }}; scopes: {{ target.scopes.join(', ') }}<template v-if="target.app_binding">; app key <code>{{ target.app_binding.app_key }}</code>, app ID <code>{{ target.app_binding.id }}</code>, MCP server <code>{{ target.app_binding.mcp_server }}</code></template></li></ul></li></ul></li>
          </ul>
        </div>
        <div v-if="installCandidate" class="status-card">
          <span class="validation-badge"><span>✓</span> {{ validationLabel(installCandidate) }}</span>
          <h3>Package evidence</h3>
          <ul v-if="installCandidate.package_evidence.length" class="evidence-list">
            <li v-for="item in installCandidate.package_evidence" :key="item.id">
              <strong>{{ evidenceLabel(item) }}</strong><span v-if="item.tested_at"> — {{ item.tested_at }}</span><br />
              <small>Evidence ID <code>{{ item.id }}</code><br />Package <code>{{ item.package_tree_digest }}</code><br />Artifact <code>{{ item.artifact.repository }}@{{ item.artifact.revision }}//{{ item.artifact.path }}</code><br />Artifact digest <code>{{ item.artifact.digest }}</code></small>
              <a :href="item.artifact.url" target="_blank" rel="noreferrer">Exact evidence ↗</a>
            </li>
          </ul>
          <p v-else>No package-level schema evidence is selected for this exact release.</p>
          <h3>Client evidence</h3>
          <ul v-if="installCandidate.evidence.length" class="evidence-list">
            <li v-for="item in installCandidate.evidence" :key="item.id"><strong>{{ item.client }}: {{ evidenceLabel(item) }}</strong><span v-if="item.client_version || item.os || item.architecture || item.tested_at"> — {{ [item.client_version, item.os, item.architecture, item.installer_version && `installer ${item.installer_version}`, item.dependency_identity, item.tested_at].filter(Boolean).join(' · ') }}</span><span v-else> — legacy evidence record; open the report for the exact applicable environment</span><template v-if="item.package_tree_digest && item.artifact"><br /><small>Evidence ID <code>{{ item.id }}</code><br />Package <code>{{ item.package_tree_digest }}</code><br />Artifact <code>{{ item.artifact.repository }}@{{ item.artifact.revision }}//{{ item.artifact.path }}</code><br />Artifact digest <code>{{ item.artifact.digest }}</code></small> <a :href="item.artifact.url" target="_blank" rel="noreferrer">Exact evidence ↗</a></template></li>
          </ul>
          <p v-else>No client materialization, discovery, runtime, or OAuth evidence is selected for this exact release.</p>
          <a :href="`${repositoryUrl}/blob/main/docs/VERIFICATION.md`" target="_blank" rel="noreferrer">Read verification evidence →</a>
        </div>
      </article>
      <InstallPanel v-model:targets="targets" :plugin="plugin" />
    </div>
  </div>
</template>
