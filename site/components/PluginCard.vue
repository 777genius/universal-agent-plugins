<!--
  Card composition adapted from plugin-kit-ai landing/components/plugins/PluginCard.vue (MIT).
  Content model and implementation are new for Universal Agent Plugins.
-->
<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry'
import { authenticationLabel, deliveryLabel, expectedDistribution, githubSourceUrl, resolveDistribution, validationLabel } from '~/utils/registry'
import { pluginCommands } from '~/utils/commands'

const props = defineProps<{ plugin: RegistryPlugin }>()
const { asset, pluginIcon } = useSite()
const { current, expired, published } = useDirectoryStatus()
const availableClients = computed(() => clients.filter(client => props.plugin.client_support.clients.includes(client.id)))
const initialTarget = availableClients.value.find(client => client.id === 'cursor')?.id ?? availableClients.value[0]?.id
const targets = ref<(typeof clients)[number]['id'][]>(initialTarget ? [initialTarget] : [])
const resolution = computed(() => resolveDistribution(props.plugin, targets.value))
const selectedDistribution = computed(() => current.value ? resolution.value.distribution : undefined)
const command = computed(() => selectedDistribution.value ? pluginCommands(props.plugin, targets.value).add : '')
const targetOptions = computed(() => clients.map(client => ({
  value: client.id,
  label: client.name,
  icon: asset(`client-icons/${client.icon}`),
  disabled: !props.plugin.client_support.clients.includes(client.id),
  description: (() => {
    if (!published.value) return 'Unavailable: review data is not installation authority'
    if (expired.value) return 'Unavailable: signed Directory snapshot expired'
    const source = expectedDistribution(props.plugin, [client.id])
    const target = source?.targets.find(item => item.client === client.id)
    return target ? deliveryLabel(target.delivery) : client.id === 'chatgpt' ? 'No signed app binding' : 'Not installable from an active release'
  })(),
})))
const authLabel = computed(() => authenticationLabel(resolution.value.distribution, targets.value, props.plugin.authentication))

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
      <span class="source-pill">{{ selectedDistribution ? `Install candidate · ${selectedDistribution.label}` : expired ? 'Stale Directory · history only' : !published ? 'Review preview · history only' : 'No install candidate' }}</span>
    </div>
    <h3><NuxtLink class="plugin-card__title-link" :to="`/plugins/${plugin.name}`">{{ plugin.display_name }}</NuxtLink></h3>
    <p v-if="selectedDistribution" class="plugin-card__author">Install candidate v{{ selectedDistribution.version }} · release {{ selectedDistribution.release_sequence }}</p>
    <p class="plugin-card__description">{{ plugin.description }}</p>
    <p v-if="selectedDistribution" class="plugin-card__author">Install source by {{ selectedDistribution.publisher }}<template v-if="selectedDistribution.id !== plugin.declared_default_distribution"> (signed fallback for selected clients)</template> · <a :href="githubSourceUrl(plugin, selectedDistribution)" target="_blank" rel="noreferrer">exact provenance <span class="sr-only">for {{ plugin.name }}</span></a><template v-if="plugin.distributions.length > 1"> · {{ plugin.distributions.length - 1 }} historical {{ plugin.distributions.length === 2 ? 'alternative' : 'alternatives' }}</template></p>
    <p v-if="resolution.fallback_reason && current" class="plugin-card__author">{{ resolution.fallback_reason }}</p>
    <p class="plugin-card__auth">{{ authLabel }}</p>
    <div class="plugin-card__bottom">
      <ul v-if="selectedDistribution" class="badge-list" aria-label="Install candidate components">
        <li v-for="component in selectedDistribution.components" :key="component">{{ component }}</li>
      </ul>
      <span v-if="selectedDistribution" class="validation-badge">
        <span aria-hidden="true">✓</span> {{ validationLabel(selectedDistribution) }}
      </span>
      <div class="plugin-card__install">
        <AppMultiSelect v-if="targets.length" :model-value="targets" :label="`Choose clients for ${plugin.display_name}`" :options="targetOptions" @update:model-value="updateTargets" />
        <CommandSnippet v-if="selectedDistribution" label="Add" kind="add" :command="command" />
        <span v-else class="plugin-card__author">{{ expired ? 'Commands disabled because the signed Directory snapshot is stale.' : !published ? 'Commands disabled because review data is not installation authority.' : resolution.unavailable_reason }}</span>
      </div>
    </div>
  </article>
</template>
