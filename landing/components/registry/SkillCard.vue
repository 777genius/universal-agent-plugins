<script setup lang="ts">
import type { SkillListing } from '~/types/skillsCatalog';
import { skillClients } from '~/data/skillClients';
import { skillDetailRoute, skillInstallCommand, skillNeedsManualSelection, skillPublisherUrl, skillRepositoryUrl } from '~/utils/skillsCatalog';
import { useInstallPreferencesStore } from '~/stores/installPreferences';

const props = defineProps<{ skill: SkillListing }>();
const localePath = useLocalePath();
const selectedSkill = useState<SkillListing | null>('skills-catalog:selected-skill', () => null);
function rememberSkill() { selectedSkill.value = props.skill; }
const { t, n } = useI18n({ useScope: 'parent' });
const { asset } = useSite();
const preferences = useInstallPreferencesStore();
const identity = `skill:${props.skill.id}`;
const allowed = skillClients.map((client) => client.id);
const saved = preferences.readPackage(identity, allowed);
const expanded = ref(saved.expanded);
const chooseInTerminal = ref(saved.autoDetect);
const targets = ref(saved.targetIds.length ? saved.targetIds : ['codex']);
const repositoryUrl = computed(() => skillRepositoryUrl(props.skill));
const publisherUrl = computed(() => skillPublisherUrl(props.skill));
const needsManualSelection = computed(() => skillNeedsManualSelection(props.skill));
const command = computed(() =>
  skillInstallCommand(props.skill, chooseInTerminal.value ? undefined : targets.value),
);
const panelId = `skill-install-${useId()}`;
const options = computed(() =>
  skillClients.map((client) => ({
    value: client.id,
    label: client.name,
    icon: asset(`client-icons/${client.icon}`),
    description: t('registryUi.skills.checkedBeforeInstall'),
  })),
);
const terminalOption = computed(() => ({
  label: t('registryUi.skills.chooseInTerminal'),
  summary: t('registryUi.skills.chooseInTerminal'),
  description: t('registryUi.skills.chooseInTerminalHelp'),
}));
function saveChoice() {
  preferences.selectPackage(
    identity,
    { targetIds: targets.value, autoDetect: chooseInTerminal.value, expanded: expanded.value },
    allowed,
  );
}
function toggleInstall() {
  expanded.value = !expanded.value;
  saveChoice();
}
function updateTargets(values: string[]) {
  const next = values.filter((id) => allowed.includes(id as (typeof allowed)[number]));
  if (next.length) {
    targets.value = next;
    saveChoice();
  }
}
function updateTerminal(value: boolean) {
  chooseInTerminal.value = value;
  saveChoice();
}
</script>

<template>
  <CatalogCard :expanded="expanded" class="skill-card" :data-skill-id="skill.id">
    <div class="plugin-card__identity">
      <div class="plugin-card__identity-copy">
        <h3 :title="skill.title"><NuxtLink :to="localePath(skillDetailRoute(skill))" @click="rememberSkill">{{ skill.title }}</NuxtLink></h3>
        <p class="plugin-card__source-label">
          <a v-if="repositoryUrl" :href="repositoryUrl" target="_blank" rel="noreferrer">{{
            skill.source.repository
          }}</a>
          <a v-else-if="publisherUrl" :href="publisherUrl" target="_blank" rel="noreferrer noopener">{{ publisherUrl.replace('https://', '') }}</a>
          <span v-else>{{ t('registryUi.skills.externalSource') }}</span>
        </p>
      </div>
    </div>
    <SkillSpectorAssessment
      :subject="
        skill.source.kind === 'github' && skill.source.repository
          ? { kind: 'skill', repository: skill.source.repository, name: skill.skillNameHint }
          : undefined
      "
      :applicable="skill.source.kind === 'github' && Boolean(skill.source.repository)"
      :name="skill.title"
      scope="skill"
    />
    <p class="skill-card__metric">
      {{
        t(`registryUi.skills.metric.${skill.metrics.window}`, { count: n(skill.metrics.installs) })
      }}
    </p>
    <div v-if="repositoryUrl || publisherUrl" class="skill-card__links">
      <a :href="repositoryUrl || publisherUrl" target="_blank" rel="noreferrer noopener"
        >{{ t(repositoryUrl ? 'registryUi.skills.githubSource' : 'registryUi.skills.publisherSource') }}<span aria-hidden="true"> ↗</span></a
      >
    </div>
    <div class="plugin-card__bottom">
      <button
        v-if="command"
        type="button"
        class="plugin-card__install-toggle"
        :aria-expanded="expanded"
        :aria-controls="panelId"
        :aria-label="t('registryUi.card.installName', { name: skill.title })"
        @click="toggleInstall"
      >
        {{ t('registryUi.card.install') }}
      </button>
      <p v-else class="plugin-card__unavailable">
        {{
          t(
            repositoryUrl
              ? 'registryUi.skills.unsupportedName'
              : publisherUrl
                ? 'registryUi.skills.invalidPublisherName'
                : 'registryUi.skills.unsupportedSource',
          )
        }}
      </p>
      <Transition name="plugin-install">
        <div v-if="expanded && command" :id="panelId" class="plugin-card__install-panel">
          <div class="plugin-card__install">
            <AppMultiSelect
              :model-value="targets"
              :auto-selected="chooseInTerminal"
              :auto-option="terminalOption"
              :label="t('registryUi.skills.chooseAgents')"
              :options="options"
              :automatic-help="t('registryUi.skills.chooseInTerminalHelp')"
              @update:model-value="updateTargets"
              @update:auto-selected="updateTerminal"
            />
            <CommandSnippet
              :command="command"
              :label="t('registryUi.card.add')"
              kind="add"
              compact
            />
          </div>
          <p v-if="needsManualSelection" class="plugin-card__unavailable">{{ t('registryUi.skills.manualSkillSelection') }}</p>
          <p class="plugin-card__unavailable">{{ t('registryUi.skills.checkedBeforeInstall') }}</p>
        </div>
      </Transition>
    </div>
  </CatalogCard>
</template>
