<script setup lang="ts">
import { skillManualSourceCommand } from '~/utils/skillsCatalog';

defineProps<{ reason: string }>();
const { t } = useI18n({ useScope: 'parent' });
const expanded = ref(false);
const source = ref('');
const exactName = ref('');
const command = computed(() => skillManualSourceCommand(source.value, exactName.value));
const panelId = `skill-source-${useId()}`;
const urlId = `${panelId}-url`;
const nameId = `${panelId}-name`;
const helpId = `${panelId}-help`;
</script>

<template>
  <p class="plugin-card__unavailable">{{ reason }}</p>
  <button
    type="button"
    class="plugin-card__vendor-install"
    :aria-expanded="expanded"
    :aria-controls="panelId"
    @click="expanded = !expanded"
  >
    {{ t('registryUi.skills.manualSourceAction') }}
  </button>
  <div v-if="expanded" :id="panelId" class="plugin-card__install-panel skill-card__manual-panel">
    <p :id="helpId" class="plugin-card__unavailable">
      {{ t('registryUi.skills.manualSourceHelp') }}
    </p>
    <label :for="urlId">{{ t('registryUi.skills.manualSourceURL') }}</label>
    <input
      :id="urlId"
      v-model.trim="source"
      type="url"
      autocomplete="off"
      spellcheck="false"
      :aria-describedby="helpId"
      placeholder="https://github.com/owner/repo"
    />
    <label :for="nameId">{{ t('registryUi.skills.manualExactName') }}</label>
    <input
      :id="nameId"
      v-model.trim="exactName"
      type="text"
      autocomplete="off"
      spellcheck="false"
      :aria-describedby="helpId"
      placeholder="my-skill"
    />
    <CommandSnippet
      v-if="command"
      :command="command"
      :label="t('registryUi.card.add')"
      kind="add"
      compact
    />
    <p v-else-if="source || exactName" class="plugin-card__unavailable" role="status">
      {{ t('registryUi.skills.manualSourceInvalid') }}
    </p>
  </div>
</template>
