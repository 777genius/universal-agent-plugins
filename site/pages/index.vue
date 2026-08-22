<script setup lang="ts">
import type { ClientID } from '~/types/registry'
import { pluginCommands } from '~/utils/commands'
import { deliveryLabel, expectedDistribution, resolveDistribution } from '~/utils/registry'

const registry = useRegistry()
const { current, expired, published } = useDirectoryStatus()
const { asset, repositoryUrl } = useSite()
const preferredDemoNames = ['cloudflare-docs', 'agent-code-navigator']
const demoPlugin = computed(() => {
  const ranked = [
    ...preferredDemoNames.flatMap(name => registry.plugins.filter(item => item.name === name)),
    ...registry.plugins.filter(item => !preferredDemoNames.includes(item.name)),
  ]
  const plugin = ranked.find(item => item.client_support.clients.some(client => (
    Boolean(expectedDistribution(item, [client]))
  )))
  if (!plugin) {
    throw new Error('The homepage quick start requires one installable Directory product')
  }
  return plugin
})
const heroTargets = computed(() => clients.filter(client => demoPlugin.value.client_support.clients.includes(client.id)))
const heroTargetOptions = computed(() => clients.map(client => ({
  value: client.id,
  label: heroClientLabel(client.id, client.name),
  icon: asset(`client-icons/${client.icon}`),
  disabled: !demoPlugin.value.client_support.clients.includes(client.id),
  description: (() => {
    if (!published.value) return 'Unavailable: review data is not installation authority'
    if (expired.value) return 'Unavailable: signed Directory snapshot expired'
    const target = expectedDistribution(demoPlugin.value, [client.id])?.targets.find(item => item.client === client.id)
    if (target) return deliveryLabel(target.delivery)
    return client.id === 'chatgpt' ? `Unavailable: ${demoPlugin.value.display_name} has no registered app binding in signed policy` : 'Not compatible with an active release'
  })(),
})))
const heroClientLabel = (id: ClientID, name: string) => id === 'copilot' ? 'Copilot' : name
const initialHeroTarget = heroTargets.value.find(client => client.id === 'cursor')?.id ?? heroTargets.value[0]!.id
const heroTargetIDs = ref<ClientID[]>([initialHeroTarget])
const selectedHeroClients = computed(() => heroTargets.value.filter(client => heroTargetIDs.value.includes(client.id)))
const selectedHeroNames = computed(() => selectedHeroClients.value.map(client => heroClientLabel(client.id, client.name)).join(' + '))
const heroResolution = computed(() => resolveDistribution(demoPlugin.value, selectedHeroClients.value.map(client => client.id)))
const heroCommand = computed(() => !current.value || !heroResolution.value.distribution ? '' : pluginCommands(demoPlugin.value, selectedHeroClients.value.map(client => client.id)).add)
const description = 'Install one Agent Plugins 1.0 package into one or several explicitly selected supported clients, then inspect, update, repair, switch, or remove it with the same community CLI.'
const workflowPath = ref<HTMLElement>()
const workflowAnimated = ref(false)
const workflowVisible = ref(false)
let workflowObserver: IntersectionObserver | undefined

function updateHeroTargets(values: string[]) {
  const allowed = new Set(heroTargets.value.map(client => client.id))
  const next = values.filter((value): value is ClientID => allowed.has(value as ClientID))
  if (next.length) heroTargetIDs.value = next
}

onMounted(() => {
  workflowAnimated.value = true
  const element = workflowPath.value
  if (!element) return

  workflowObserver = new IntersectionObserver(([entry]) => {
    if (!entry?.isIntersecting) return
    workflowVisible.value = true
    workflowObserver?.disconnect()
  }, { threshold: 0.22 })

  workflowObserver.observe(element)
})

onBeforeUnmount(() => workflowObserver?.disconnect())

useSeoMeta({
  title: 'A clearer way to use Agent Plugins',
  description,
  ogTitle: 'Universal Agent Plugins',
  ogDescription: description,
  ogType: 'website',
  twitterCard: 'summary',
})
useHead({ link: [{ rel: 'canonical', href: `${useRuntimeConfig().public.siteUrl}/` }] })
</script>

<template>
  <div>
    <section class="hero container">
      <div class="hero__copy">
        <h1>One package.<br /><em>Your selected clients</em></h1>
        <p class="hero__lead">One command installs an Agent Plugins 1.0 package into one or several explicitly selected compatible clients. The same CLI inspects, updates, repairs, switches, and removes it.</p>
        <div class="hero__actions">
          <NuxtLink class="button button--primary" to="/plugins">Explore {{ registry.plugins.length }} plugins <span aria-hidden="true">→</span></NuxtLink>
          <a class="button button--secondary" :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`" target="_blank" rel="noreferrer">Add a plugin</a>
        </div>
        <p class="hero__fine-print">Open source · No tracking · Review before enabling</p>
      </div>
      <div class="hero__demo">
        <div class="hero__window">
          <div class="hero__window-top"><span /><span /><span /><b>Quick start</b></div>
          <div class="hero__window-body">
            <p>Install {{ demoPlugin.display_name }} for {{ selectedHeroNames }}</p>
            <div class="hero-command-row">
              <AppMultiSelect :model-value="heroTargetIDs" label="Choose target clients" :options="heroTargetOptions" @update:model-value="updateHeroTargets" />
              <CommandSnippet v-if="heroCommand" :command="heroCommand" />
              <p v-else class="install-panel__notice" role="status"><strong>Command unavailable{{ expired ? ': stale Directory' : !published ? ': review preview' : '' }}.</strong> {{ expired ? 'Browse historical package information while a fresh signed snapshot is published.' : !published ? 'Production commands require a published signed Directory snapshot.' : heroResolution.unavailable_reason }}</p>
            </div>
            <p v-if="heroResolution.fallback_reason && !expired" class="hero__fine-print">{{ heroResolution.fallback_reason }}</p>
            <p v-else-if="heroResolution.unavailable_reason && !expired" class="hero__fine-print">{{ heroResolution.unavailable_reason }}</p>
            <div v-if="heroCommand" class="hero__success">
              <span>✓</span>
              <div><strong>Command ready for {{ selectedHeroClients.length === 1 ? selectedHeroNames : `${selectedHeroClients.length} agents` }}</strong><small>One command installs each selected target in order.</small></div>
            </div>
          </div>
        </div>
        <div class="hero__float hero__float--schema"><span>✓</span> Schema validated</div>
        <div class="hero__float hero__float--standard">
          <a href="https://agent-plugins.org/specification" target="_blank" rel="noreferrer">Agent Plugins 1.0</a>
          <span>standard</span>
        </div>
      </div>
    </section>

    <section class="client-section container" aria-labelledby="clients-title">
      <p id="clients-title">Supported agents</p>
      <ClientStrip />
      <p class="client-section__note">Compatibility is release-specific. “Prepared” and “manual activation required” remain visible; ChatGPT is selectable only with a registered app binding.</p>
    </section>

    <section id="how-it-works" class="how container" aria-labelledby="how-title">
      <div class="section-heading section-heading--center">
        <p class="eyebrow">A small, explicit workflow</p>
        <h2 id="how-title">From directory to agent in three steps</h2>
      </div>
      <ol ref="workflowPath" class="workflow-path" :class="{ 'workflow-path--animate': workflowAnimated, 'workflow-path--visible': workflowVisible }">
        <li class="workflow-step workflow-step--plugin">
          <div class="workflow-step__head">
            <span class="workflow-step__number">01</span>
            <span class="workflow-step__icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none"><path d="m4.5 8 7.5-4 7.5 4-7.5 4-7.5-4Z" /><path d="m4.5 8v8l7.5 4 7.5-4V8M12 12v8" /></svg>
            </span>
          </div>
          <div><h3>Pick a plugin</h3><p>Review its source, components, permissions, and validation status.</p></div>
          <div class="workflow-step__tags" aria-hidden="true"><span>Source</span><span>Permissions</span></div>
        </li>
        <li class="workflow-step workflow-step--agent">
          <div class="workflow-step__head">
            <span class="workflow-step__number">02</span>
            <span class="workflow-step__icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none"><circle cx="12" cy="6" r="2.5" /><circle cx="6" cy="17" r="2.5" /><circle cx="18" cy="17" r="2.5" /><path d="m10.8 8.2-3.6 6.6m6-6.6 3.6 6.6M8.5 17h7" /></svg>
            </span>
          </div>
          <div><h3>Choose your agents</h3><p>Select one or more of Codex, ChatGPT, Cursor, Copilot, VS Code, or Kiro.</p></div>
          <div class="workflow-step__tags" aria-hidden="true"><span>Multi-target</span><span>One command</span></div>
        </li>
        <li class="workflow-step workflow-step--control">
          <div class="workflow-step__head">
            <span class="workflow-step__number">03</span>
            <span class="workflow-step__icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none"><path d="M19 8a7.5 7.5 0 1 0 .4 7" /><path d="M19 4v4h-4" /><path d="m9 12 2 2 4-4" /></svg>
            </span>
          </div>
          <div><h3>Stay in control</h3><p>Inspect, update, repair, switch source, or remove it. Follow any client activation or OAuth prompt.</p></div>
          <div class="workflow-step__tags" aria-hidden="true"><span>Repair</span><span>Switch</span><span>Remove</span></div>
        </li>
      </ol>
    </section>

    <div class="container catalog-wrap">
      <PluginCatalog
        :plugins="registry.plugins"
        :heading="`Explore ${registry.plugins.length} plugins`"
        :intro="`${registry.plugins.length} reviewed products, each with one Default source and immutable provenance. Alternatives stay on the same product page.`"
      />
    </div>

    <section class="validation-section container" aria-labelledby="validation-title">
      <div>
        <p class="eyebrow">Read status precisely</p>
        <h2 id="validation-title">Validated structure is not a runtime promise.</h2>
      </div>
      <div class="validation-section__cards">
        <article><span class="validation-badge"><span>✓</span> Schema validated</span><p>The manifest and declared components passed the repository’s structural checks.</p></article>
        <article><span class="runtime-badge">◇ Runtime evidence</span><p>Runtime, authentication, and OAuth coverage varies by plugin and agent. Check the linked evidence before relying on a path.</p></article>
      </div>
    </section>

    <section class="submit-cta container">
      <div><p class="eyebrow">Built in the open</p><h2>Have a useful Agent Plugin?</h2><p>Submit a schema-valid package with a reviewable source. External entries stay pinned to an immutable commit.</p></div>
      <a class="button button--primary" :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`" target="_blank" rel="noreferrer">Add a plugin by pull request <span aria-hidden="true">↗</span></a>
    </section>
  </div>
</template>
