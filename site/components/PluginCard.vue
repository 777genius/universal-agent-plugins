<!--
  Card composition adapted from plugin-kit-ai landing/components/plugins/PluginCard.vue (MIT).
  Content model and implementation are new for Universal Agent Plugins.
-->
<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry'
import { defaultDistribution, validationLabel } from '~/utils/registry'
import { pluginCommands } from '~/utils/commands'

const props = defineProps<{ plugin: RegistryPlugin }>()
const { asset, pluginIcon, sourceUrl } = useSite()
const distribution = computed(() => defaultDistribution(props.plugin))
const availableClients = computed(() => clients.filter(client => props.plugin.client_support.clients.includes(client.id)))
const initialTarget = availableClients.value.find(client => client.id === 'cursor')?.id ?? availableClients.value[0]!.id
const targets = ref<(typeof clients)[number]['id'][]>([initialTarget])
const command = computed(() => pluginCommands(props.plugin, targets.value).add)
const targetOptions = computed(() => clients.map(client => ({
  value: client.id,
  label: client.name,
  icon: asset(`client-icons/${client.icon}`),
  disabled: !props.plugin.client_support.clients.includes(client.id) || (client.id === 'chatgpt' && !props.plugin.client_support.chatgpt_binding),
  description: client.id === 'chatgpt' && !props.plugin.client_support.chatgpt_binding ? 'No registered app binding' : undefined,
})))
const authLabel = computed(() => props.plugin.authentication === 'none' ? 'No account required' : props.plugin.authentication === 'oauth' ? 'OAuth required' : props.plugin.authentication === 'client_managed' ? 'Client-managed authentication' : 'Authentication varies')

function updateTargets(values: string[]) {
  const allowed = new Set(availableClients.value.map(client => client.id))
  const next = values.filter((value): value is (typeof clients)[number]['id'] => allowed.has(value as (typeof clients)[number]['id']))
  if (next.length) targets.value = next
}
</script>

<template>
  <article class="plugin-card">
    <div class="plugin-card__top">
      <span class="plugin-card__icon"><img :src="pluginIcon(plugin)" alt="" width="32" height="32" loading="lazy" /></span>
      <span class="source-pill">Default source · {{ distribution.label }}</span>
    </div>
    <h3><NuxtLink class="plugin-card__title-link" :to="`/plugins/${plugin.name}`">{{ plugin.display_name }}</NuxtLink></h3>
    <p class="plugin-card__description">{{ plugin.description }}</p>
    <p class="plugin-card__author">Default source by {{ distribution.publisher }} · <a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">provenance <span class="sr-only">for {{ plugin.name }}</span></a><template v-if="plugin.distributions.length > 1"> · {{ plugin.distributions.length - 1 }} {{ plugin.distributions.length === 2 ? 'alternative' : 'alternatives' }}</template></p>
    <p class="plugin-card__auth">{{ authLabel }}</p>
    <div class="plugin-card__bottom">
      <ul class="badge-list" aria-label="Plugin components">
        <li v-for="component in plugin.components" :key="component">{{ component }}</li>
      </ul>
      <span class="validation-badge">
        <span aria-hidden="true">✓</span> {{ validationLabel(plugin) }}
      </span>
      <div class="plugin-card__install">
        <AppMultiSelect :model-value="targets" :label="`Choose clients for ${plugin.display_name}`" :options="targetOptions" @update:model-value="updateTargets" />
        <CommandSnippet label="Add" kind="add" :command="command" />
      </div>
    </div>
  </article>
</template>
