<script setup lang="ts">
import { clients } from '~/data/clients';
import { projectComponentSupported } from '~/utils/projectCatalogSource';
import type { ClientID, DistributionView, InstallScope } from '~/types/registry';

const props = defineProps<{
  distribution: DistributionView;
  targets: readonly ClientID[];
  scope?: InstallScope;
}>();
const { t } = useI18n();
const selected = computed(() =>
  props.targets.map((id) => ({
    id,
    name: clients.find((client) => client.id === id)?.name ?? id,
    policy: props.distribution.targets.find((target) => target.client === id),
  })),
);
</script>

<template>
  <div class="install-outcome">
    <p v-if="scope === 'project'">{{ t('registryUi.install.outcome.projectPreview') }}</p>
    <p v-else>
      {{ t('registryUi.install.outcome.includes') }}
      <span v-for="(component, index) in distribution.components" :key="component"
        >{{ index ? ', ' : '' }}{{ t(`registryUi.install.outcome.components.${component}`) }}</span
      >
    </p>
    <ul :aria-label="t('registryUi.install.outcome.label')">
      <li v-for="target in selected" :key="target.id">
        <strong>{{ target.name }}:</strong>
        <template v-if="scope === 'project'">
          <span
            v-for="(component, index) in distribution.components"
            :key="component"
            :data-component="component"
            :data-support="projectComponentSupported(target.id, component) ? 'supported' : 'unsupported'"
          >
            {{ index ? ' · ' : ' ' }}{{ t(`registryUi.install.outcome.components.${component}`) }} —
            {{ t(`registryUi.install.outcome.${projectComponentSupported(target.id, component) ? 'projectSupported' : 'projectUnsupported'}`) }}
          </span>
        </template>
        <template v-else>{{ t(`registryUi.install.outcome.${target.policy?.delivery ?? 'unknown'}`) }}</template>
        <span v-if="target.policy?.authentication !== 'not_required'">{{
          t(`registryUi.install.outcome.auth.${target.policy?.authentication ?? 'unknown'}`)
        }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.install-outcome {
  color: var(--subtle);
  font-size: 0.76rem;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.install-outcome p {
  margin: 0 0 5px;
}
.install-outcome ul {
  margin: 0;
  padding-inline-start: 18px;
}
.install-outcome strong {
  color: var(--ink);
}
</style>
