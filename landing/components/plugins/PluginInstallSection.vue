<script setup lang="ts">
import { computed } from 'vue';
import { mdiApps, mdiArrowRight, mdiChevronUp, mdiOpenInNew } from '@mdi/js';
import CommandSnippetCard from '~/components/shared/CommandSnippetCard.vue';
import { resolveAgentBadge } from '~/data/agentBadges';
import { buildInstallCommandForSelection } from '~/data/pluginInstall';
import type { InstallChannel, PluginCard, PluginResolvedInstallSpec } from '~/types/content';
import { applyCliInvocation, getCliInvocation } from '~/utils/cliInvocation';

const props = withDefaults(
  defineProps<{
    plugin: PluginCard;
    installSpec: PluginResolvedInstallSpec;
    accent?: string;
    expanded?: boolean;
  }>(),
  {
    accent: '#00f0ff',
    expanded: false,
  },
);
const emit = defineEmits<{
  'update:expanded': [value: boolean];
}>();

const { content } = useLandingContent();
const { t, te } = useI18n();
const localePath = useLocalePath();

const selectedTargetIds = ref<
  Array<(typeof props.installSpec.supportedTargets)[number]['targetId']>
>([]);
const isExpanded = computed({
  get: () => props.expanded,
  set: (value: boolean) => emit('update:expanded', value),
});

const downloadPagePath = computed(() => localePath('/download'));

const targetLanes = computed(() => props.installSpec.supportedTargets);
const allSelected = computed(() => selectedTargetIds.value.length === 0);
const selectedTargets = computed(() =>
  allSelected.value
    ? supportLanes.value
    : supportLanes.value.filter((lane) => selectedTargetIds.value.includes(lane.targetId)),
);

const installChannels = computed<InstallChannel[]>(() =>
  content.value.installChannels.filter(
    (channel) => channel.command && ['brew', 'script', 'powershell', 'npm'].includes(channel.id),
  ),
);

const {
  detectedInstallPlatform,
  recommendedChannelId,
  selectedInstallChannelId,
  selectInstallChannel,
} = useInstallChannelSelection(installChannels);

const selectedInstallChannel = computed(
  () =>
    installChannels.value.find((channel) => channel.id === selectedInstallChannelId.value) ??
    installChannels.value[0],
);

const skipCliStep = computed(() => selectedInstallChannel.value?.id === 'npm');
const showCliStep = computed(() => Boolean(selectedInstallChannel.value) && !skipCliStep.value);
const installStepIndex = computed(() => (skipCliStep.value ? '02' : '03'));
const manualStepIndex = computed(() => (skipCliStep.value ? '03' : '04'));
const manageStepIndex = computed(() => (skipCliStep.value ? '04' : '05'));

const selectedCliInvocation = computed(() =>
  getCliInvocation(selectedInstallChannel.value?.invocation),
);

const selectedTargetArgument = computed(() =>
  selectedTargets.value.map((lane) => lane.targetId).join(','),
);

const supportLanes = computed(() =>
  props.installSpec.recommendedTargetOrder
    .map((targetId) => targetLanes.value.find((lane) => lane.targetId === targetId))
    .filter((lane): lane is NonNullable<typeof lane> => Boolean(lane))
    .map((lane) => ({
      ...lane,
      iconSrc: resolveAgentBadge(lane.badgeLabel),
    })),
);

const selectedTargetNames = computed(() =>
  selectedTargets.value.map((lane) => lane.badgeLabel).join(', '),
);

const installCommand = computed(() =>
  buildInstallCommandForSelection(props.installSpec, selectedTargetIds.value),
);

const renderedInstallCommand = computed(() =>
  applyCliInvocation(installCommand.value, selectedCliInvocation.value),
);

const renderedManageCommands = computed(() => ({
  update: `${applyCliInvocation(props.installSpec.manageCommands.update, selectedCliInvocation.value)} --target ${selectedTargetArgument.value}`,
  repair: `${applyCliInvocation(props.installSpec.manageCommands.repair, selectedCliInvocation.value)} --target ${selectedTargetArgument.value}`,
  remove: `${applyCliInvocation(props.installSpec.manageCommands.remove, selectedCliInvocation.value)} --target ${selectedTargetArgument.value}`,
}));

const selectionScope = computed(() => {
  const uniqueScopes = [...new Set(selectedTargets.value.map((lane) => lane.scope))];
  if (uniqueScopes.length === 1) {
    return uniqueScopes[0] === 'project'
      ? t('plugins.install.projectScope')
      : t('plugins.install.userScope');
  }

  return t('plugins.install.mixedScope');
});

const installPaths = computed(() => [
  ...new Set(selectedTargets.value.map((lane) => lane.installPath)),
]);

const nextStepSummary = computed(() => {
  const followUps = [...new Set(selectedTargets.value.map((lane) => lane.followUp))];
  if (followUps.length === 1) {
    return t(`plugins.install.followUps.${followUps[0]}`);
  }

  return t('plugins.install.followUps.depends');
});

const projectRootHint = computed(() => {
  if (!selectedTargets.value.some((lane) => lane.projectRootRequired)) {
    return '';
  }

  if (allSelected.value || selectedTargets.value.some((lane) => lane.scope === 'user')) {
    return t('plugins.install.projectRootHintMixed');
  }

  return t('plugins.install.projectRootHint');
});

const targetBoundaryNotes = computed(() => {
  const notes = selectedTargets.value
    .map((lane) => {
      const key = `plugins.install.targets.${lane.targetId}.boundaryNote`;
      if (!te(key)) {
        return null;
      }

      return {
        id: lane.targetId,
        label: lane.badgeLabel,
        iconSrc: resolveAgentBadge(lane.badgeLabel),
        note: t(key),
        subtle: false,
      };
    })
    .filter((note): note is NonNullable<typeof note> => Boolean(note));

  if ((allSelected.value || selectedTargets.value.length > 1) && projectRootHint.value) {
    notes.unshift({
      id: 'project-root',
      label: t('plugins.install.scopeLabel'),
      iconSrc: '',
      note: projectRootHint.value,
      subtle: true,
    });
  }

  return notes;
});

const selectedDocsTargets = computed(() =>
  selectedTargets.value
    .filter((lane) => Boolean(lane.vendorDocsHref))
    .map((lane) => ({
      ...lane,
      iconSrc: resolveAgentBadge(lane.badgeLabel),
    })),
);

function toggleAllTargets() {
  selectedTargetIds.value = [];
}

function toggleTarget(targetId: (typeof props.installSpec.supportedTargets)[number]['targetId']) {
  if (allSelected.value) {
    selectedTargetIds.value = [targetId];
    return;
  }

  if (selectedTargetIds.value.includes(targetId)) {
    selectedTargetIds.value = selectedTargetIds.value.filter((id) => id !== targetId);
  } else {
    selectedTargetIds.value = [...selectedTargetIds.value, targetId];
  }

  if (selectedTargetIds.value.length === 0) {
    selectedTargetIds.value = [];
  }
}

function toggleExpanded() {
  isExpanded.value = !isExpanded.value;
}
</script>

<template>
  <section
    v-if="isExpanded"
    id="plugin-install"
    class="plugin-install section"
    :style="{ '--install-accent': accent }"
  >
    <v-container>
      <div class="plugin-install__shell">
        <div class="plugin-install__toolbar">
          <p class="plugin-install__eyebrow">{{ t('plugins.install.eyebrow') }}</p>
          <v-btn
            variant="text"
            class="plugin-install__expand-cta"
            :aria-expanded="isExpanded"
            :append-icon="mdiChevronUp"
            @click="toggleExpanded"
          >
            {{ t('plugins.install.hideInstallCta') }}
          </v-btn>
        </div>

        <div class="plugin-install__details">
          <article class="plugin-install__card plugin-install__card--targets">
            <div class="plugin-install__card-head">
              <span class="plugin-install__step-index">01</span>
              <div>
                <h3 class="plugin-install__card-title">
                  {{ t('plugins.install.chooseAgentTitle') }}
                </h3>
                <p class="plugin-install__card-copy">{{ t('plugins.install.chooseAgentBody') }}</p>
              </div>
            </div>

            <div
              class="plugin-install__target-tabs"
              role="group"
              :aria-label="t('plugins.install.chooseAgentTitle')"
            >
              <button
                type="button"
                class="plugin-install__target-tab plugin-install__target-tab--all"
                :class="{ 'plugin-install__target-tab--active': allSelected }"
                :aria-pressed="allSelected"
                @click="toggleAllTargets"
              >
                <span class="plugin-install__target-tab-content">
                  <v-icon :icon="mdiApps" size="18" class="plugin-install__target-tab-icon" />
                  <span>{{ t('plugins.install.allAgents') }}</span>
                </span>
              </button>

              <span class="plugin-install__target-divider" aria-hidden="true" />

              <button
                v-for="lane in supportLanes"
                :key="lane.targetId"
                type="button"
                class="plugin-install__target-tab"
                :class="{
                  'plugin-install__target-tab--active':
                    !allSelected && selectedTargetIds.includes(lane.targetId),
                }"
                :aria-pressed="!allSelected && selectedTargetIds.includes(lane.targetId)"
                @click="toggleTarget(lane.targetId)"
              >
                <span class="plugin-install__target-tab-content">
                  <img
                    v-if="lane.iconSrc"
                    :src="lane.iconSrc"
                    :alt="`${lane.badgeLabel} icon`"
                    class="plugin-install__target-tab-image"
                    loading="lazy"
                    decoding="async"
                  >
                  <span>{{ lane.badgeLabel }}</span>
                </span>
              </button>
            </div>

            <hr class="plugin-install__section-divider">

            <div
              class="plugin-install__channel-tabs"
              role="group"
              :aria-label="t('plugins.install.getCliTitle')"
            >
              <button
                v-for="channel in installChannels"
                :key="channel.id"
                type="button"
                class="plugin-install__channel-tab"
                :class="{
                  'plugin-install__channel-tab--active': channel.id === selectedInstallChannelId,
                }"
                :aria-pressed="channel.id === selectedInstallChannelId"
                @click="selectInstallChannel(channel.id)"
              >
                <span>{{ channel.title }}</span>
                <span
                  v-if="channel.id === recommendedChannelId"
                  class="plugin-install__channel-tab-badge"
                >
                  {{ t('plugins.install.recommended') }}
                </span>
              </button>
            </div>
            <p v-if="detectedInstallPlatform === 'mobile'" class="plugin-install__platform-note">
              {{ t('download.mobileUnsupported') }}
            </p>
          </article>

          <div class="plugin-install__stack">
            <article v-if="showCliStep" class="plugin-install__card plugin-install__card--onboard">
              <div class="plugin-install__card-head">
                <span class="plugin-install__step-index">02</span>
                <div>
                  <h3 class="plugin-install__card-title">{{ t('plugins.install.getCliTitle') }}</h3>
                  <p class="plugin-install__card-copy">{{ t('plugins.install.getCliBody') }}</p>
                </div>
              </div>

              <p
                v-if="selectedInstallChannel && selectedInstallChannel.id !== 'npm'"
                class="plugin-install__channel-description"
              >
                {{ selectedInstallChannel.description }}
              </p>

              <CommandSnippetCard
                v-if="selectedInstallChannel"
                :label="t('plugins.install.cliCommandLabel')"
                :command="selectedInstallChannel.command || ''"
                :copy-label="t('plugins.install.copy')"
                :copied-label="t('plugins.install.copied')"
                :accent="accent"
              />

              <p v-if="selectedInstallChannel?.note" class="plugin-install__muted-note">
                {{ selectedInstallChannel.note }}
              </p>

              <div class="plugin-install__cta-row">
                <v-btn
                  :to="downloadPagePath"
                  variant="outlined"
                  class="plugin-install__secondary-cta"
                >
                  {{ t('plugins.install.fullOnboardingCta') }}
                  <v-icon :icon="mdiArrowRight" end size="18" />
                </v-btn>
                <span class="plugin-install__microcopy">{{
                  t('plugins.install.alreadyInstalled')
                }}</span>
              </div>
            </article>

            <article class="plugin-install__card plugin-install__card--install">
              <div class="plugin-install__card-head">
                <span class="plugin-install__step-index">{{ installStepIndex }}</span>
                <div>
                  <h3 class="plugin-install__card-title">
                    {{ t('plugins.install.installTitle') }}
                  </h3>
                </div>
              </div>

              <CommandSnippetCard
                :label="t('plugins.install.installCommandLabel')"
                :command="renderedInstallCommand"
                :copy-label="t('plugins.install.copy')"
                :copied-label="t('plugins.install.copied')"
                :accent="accent"
              />

              <div class="plugin-install__facts">
                <div class="plugin-install__fact">
                  <span class="plugin-install__fact-label">{{
                    t('plugins.install.targetsLabel')
                  }}</span>
                  <strong class="plugin-install__fact-value">{{ selectedTargetNames }}</strong>
                </div>
                <div class="plugin-install__fact">
                  <span class="plugin-install__fact-label">{{
                    t('plugins.install.scopeLabel')
                  }}</span>
                  <strong class="plugin-install__fact-value">{{ selectionScope }}</strong>
                </div>
                <div class="plugin-install__fact">
                  <span class="plugin-install__fact-label">{{
                    t('plugins.install.writesToLabel')
                  }}</span>
                  <strong class="plugin-install__fact-value plugin-install__fact-value--stacked">
                    <span
                      v-for="installPath in installPaths"
                      :key="installPath"
                      class="plugin-install__fact-pill"
                    >
                      {{ installPath }}
                    </span>
                  </strong>
                </div>
                <div class="plugin-install__fact">
                  <span class="plugin-install__fact-label">{{
                    t('plugins.install.nextStepLabel')
                  }}</span>
                  <strong class="plugin-install__fact-value">{{ nextStepSummary }}</strong>
                </div>
              </div>

              <div v-if="targetBoundaryNotes.length > 0" class="plugin-install__boundary-group">
                <p
                  v-for="boundaryNote in targetBoundaryNotes"
                  :key="boundaryNote.id"
                  class="plugin-install__boundary"
                  :class="{
                    'plugin-install__boundary--subtle': boundaryNote.subtle,
                  }"
                >
                  <span class="plugin-install__boundary-head">
                    <AgentBadge
                      v-if="boundaryNote.iconSrc"
                      :label="boundaryNote.label"
                      tone="card"
                    />
                    <span
                      v-else
                      class="plugin-install__boundary-chip plugin-install__boundary-chip--neutral"
                    >
                      {{ boundaryNote.label }}
                    </span>
                    <span class="plugin-install__boundary-label">
                      {{ t('plugins.install.boundaryLabel') }}
                    </span>
                  </span>
                  <span class="plugin-install__boundary-text">{{ boundaryNote.note }}</span>
                </p>
              </div>
            </article>
          </div>

          <div class="plugin-install__grid plugin-install__grid--secondary">
            <article class="plugin-install__card">
              <div class="plugin-install__card-head">
                <span class="plugin-install__step-index">{{ manualStepIndex }}</span>
                <div>
                  <h3 class="plugin-install__card-title">{{ t('plugins.install.manualTitle') }}</h3>
                  <p class="plugin-install__card-copy">{{ t('plugins.install.manualBody') }}</p>
                </div>
              </div>

              <div class="plugin-install__manual-links">
                <a
                  :href="plugin.href"
                  target="_blank"
                  rel="noreferrer noopener"
                  class="plugin-install__manual-link plugin-install__manual-link--repo"
                >
                  <span class="plugin-install__manual-link-main">
                    <span class="plugin-install__manual-link-label">{{
                      t('plugins.install.repositoryCta')
                    }}</span>
                  </span>
                  <v-icon :icon="mdiOpenInNew" size="18" />
                </a>
                <a
                  v-for="lane in selectedDocsTargets"
                  :key="lane.targetId"
                  :href="lane.vendorDocsHref"
                  target="_blank"
                  rel="noreferrer noopener"
                  class="plugin-install__manual-link"
                >
                  <span class="plugin-install__manual-link-main">
                    <img
                      v-if="lane.iconSrc"
                      :src="lane.iconSrc"
                      :alt="`${lane.badgeLabel} icon`"
                      class="plugin-install__manual-link-icon"
                      loading="lazy"
                      decoding="async"
                    >
                    <span class="plugin-install__manual-link-label">{{
                      t('plugins.install.agentDocsCta', { agent: lane.badgeLabel })
                    }}</span>
                  </span>
                  <v-icon :icon="mdiOpenInNew" size="18" />
                </a>
              </div>
            </article>

            <article class="plugin-install__card">
              <div class="plugin-install__card-head">
                <span class="plugin-install__step-index">{{ manageStepIndex }}</span>
                <div>
                  <h3 class="plugin-install__card-title">{{ t('plugins.install.manageTitle') }}</h3>
                  <p class="plugin-install__card-copy">{{ t('plugins.install.manageBody') }}</p>
                </div>
              </div>

              <div class="plugin-install__manage-grid">
                <CommandSnippetCard
                  :label="t('plugins.install.updateCommandLabel')"
                  :command="renderedManageCommands.update"
                  :copy-label="t('plugins.install.copy')"
                  :copied-label="t('plugins.install.copied')"
                  :accent="accent"
                />
                <CommandSnippetCard
                  :label="t('plugins.install.repairCommandLabel')"
                  :command="renderedManageCommands.repair"
                  :copy-label="t('plugins.install.copy')"
                  :copied-label="t('plugins.install.copied')"
                  :accent="accent"
                />
                <CommandSnippetCard
                  :label="t('plugins.install.removeCommandLabel')"
                  :command="renderedManageCommands.remove"
                  :copy-label="t('plugins.install.copy')"
                  :copied-label="t('plugins.install.copied')"
                  :accent="accent"
                />
              </div>
            </article>
          </div>
        </div>
      </div>
    </v-container>
  </section>
</template>

<style scoped>
.plugin-install {
  position: relative;
  padding-top: 8px;
}

.plugin-install__shell {
  display: grid;
  gap: 24px;
  max-width: 1100px;
  margin: 0 auto;
}

.plugin-install__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.plugin-install__eyebrow,
.plugin-install__fact-label {
  margin: 0;
  font-size: 0.72rem;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.plugin-install__eyebrow {
  color: var(--install-accent);
}

.plugin-install__subtitle,
.plugin-install__card-copy,
.plugin-install__channel-description,
.plugin-install__muted-note,
.plugin-install__microcopy {
  margin: 0;
  color: #95a3c4;
  line-height: 1.65;
}

.plugin-install__expand-cta {
  justify-self: start;
  color: #dffaff !important;
  font-weight: 700 !important;
  letter-spacing: 0.03em !important;
  padding-inline: 0 !important;
  min-width: 0 !important;
}

.plugin-install__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}

.plugin-install__details,
.plugin-install__stack {
  display: grid;
  gap: 24px;
}

.plugin-install__grid--secondary {
  align-items: start;
}

.plugin-install__card {
  border-radius: 24px;
  border: 1px solid color-mix(in srgb, var(--install-accent) 16%, rgba(255, 255, 255, 0.08));
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--install-accent) 10%, transparent) 0%,
      transparent 38%
    ),
    rgba(10, 10, 15, 0.82);
  box-shadow: 0 22px 60px rgba(0, 0, 0, 0.24);
  backdrop-filter: blur(14px);
  padding: 24px;
}

.plugin-install__card--targets {
  display: grid;
  gap: 18px;
}

.plugin-install__section-divider {
  width: 100%;
  height: 1px;
  margin: 0;
  border: 0;
  background: rgba(255, 255, 255, 0.08);
}

.plugin-install__card-head {
  display: grid;
  grid-template-columns: 54px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
  margin-bottom: 16px;
}

.plugin-install__step-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--install-accent) 24%, rgba(255, 255, 255, 0.08));
  background: color-mix(in srgb, var(--install-accent) 8%, rgba(255, 255, 255, 0.02));
  color: var(--install-accent);
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  font-family: 'JetBrains Mono', monospace;
}

.plugin-install__card-title {
  margin: 0 0 8px;
  font-size: 1.06rem;
  color: #eff6ff;
}

.plugin-install__target-tabs,
.plugin-install__channel-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.plugin-install__platform-note {
  margin: 12px 0 0;
  color: #ffb703;
  font-size: 0.8rem;
  line-height: 1.5;
  font-family: 'JetBrains Mono', monospace;
}

.plugin-install__target-divider {
  width: 1px;
  align-self: stretch;
  min-height: 34px;
  background: rgba(255, 255, 255, 0.1);
  margin-inline: 4px 2px;
}

.plugin-install__target-tab,
.plugin-install__channel-tab {
  appearance: none;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
  color: #dbeafe;
  border-radius: 999px;
  padding: 10px 14px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 0.82rem;
  font-weight: 700;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.plugin-install__target-tab-content {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.plugin-install__target-tab-icon {
  flex-shrink: 0;
}

.plugin-install__target-tab-image {
  width: 16px;
  height: 16px;
  object-fit: contain;
  flex-shrink: 0;
}

.plugin-install__target-tab:hover,
.plugin-install__channel-tab:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--install-accent) 24%, rgba(255, 255, 255, 0.08));
}

.plugin-install__target-tab--active,
.plugin-install__channel-tab--active {
  color: #07141a;
  border-color: color-mix(in srgb, var(--install-accent) 28%, transparent);
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--install-accent) 88%, #ffffff),
    #d9fbff
  );
  box-shadow: 0 10px 26px color-mix(in srgb, var(--install-accent) 16%, transparent);
}

.plugin-install__target-tab--all {
  min-width: 82px;
  justify-content: center;
}

.plugin-install__channel-tab-badge {
  border-radius: 999px;
  padding: 4px 7px;
  font-size: 0.58rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 800;
  color: #39ff14;
  background: rgba(57, 255, 20, 0.08);
  border: 1px solid rgba(57, 255, 20, 0.22);
}

.plugin-install__channel-tab--active .plugin-install__channel-tab-badge {
  color: #07141a;
  background: rgba(7, 20, 26, 0.08);
  border-color: rgba(7, 20, 26, 0.08);
}

.plugin-install__facts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.plugin-install__fact {
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.03);
  padding: 14px;
  display: grid;
  align-content: start;
  grid-auto-rows: max-content;
  gap: 8px;
}

.plugin-install__fact-label {
  color: #7dd3fc;
}

.plugin-install__fact-value {
  color: #e0f2fe;
  font-size: 0.95rem;
  line-height: 1.45;
}

.plugin-install__fact-value--stacked {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.plugin-install__fact-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 6px 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #e0f2fe;
  font-size: 0.84rem;
  line-height: 1.3;
  font-weight: 600;
}

.plugin-install__boundary-group {
  display: grid;
  gap: 10px;
}

.plugin-install__boundary {
  margin: 12px 0 0;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(250, 204, 21, 0.18);
  background: rgba(250, 204, 21, 0.08);
  color: #fde68a;
  line-height: 1.55;
  display: grid;
  gap: 8px;
}

.plugin-install__boundary--subtle {
  border-color: rgba(125, 211, 252, 0.14);
  background: rgba(125, 211, 252, 0.08);
  color: #d9f6ff;
}

.plugin-install__boundary-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.plugin-install__boundary-label {
  font-size: 0.72rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.plugin-install__boundary-text {
  display: block;
}

.plugin-install__boundary-chip {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 7px 11px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #e0f2fe;
  font-size: 0.8rem;
  font-weight: 700;
}

.plugin-install__boundary-chip--neutral {
  background: rgba(125, 211, 252, 0.08);
  border-color: rgba(125, 211, 252, 0.18);
}

.plugin-install__cta-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 18px;
}

.plugin-install__secondary-cta {
  border-color: rgba(125, 211, 252, 0.2) !important;
  color: #dffaff !important;
}

.plugin-install__manual-links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 10px;
}

.plugin-install__manual-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 56px;
  padding: 14px 16px;
  border-radius: 16px;
  border: 1px solid rgba(125, 211, 252, 0.14);
  background: rgba(255, 255, 255, 0.03);
  color: #dffaff;
  text-decoration: none;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease,
    transform 0.2s ease;
}

.plugin-install__manual-link:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--install-accent) 24%, rgba(255, 255, 255, 0.1));
  background: rgba(255, 255, 255, 0.05);
}

.plugin-install__manual-link--repo {
  grid-column: 1 / -1;
}

.plugin-install__manual-link-main {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.plugin-install__manual-link-label {
  font-size: 0.96rem;
  font-weight: 700;
  color: inherit;
  letter-spacing: 0.01em;
}

.plugin-install__manual-link-icon {
  width: 18px;
  height: 18px;
  object-fit: contain;
  flex-shrink: 0;
}

.plugin-install__manage-grid {
  display: grid;
  gap: 14px;
}

.v-theme--light .plugin-install__supported,
.v-theme--light .plugin-install__card {
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--install-accent) 10%, transparent) 0%,
      transparent 38%
    ),
    rgba(255, 255, 255, 0.92);
}

.v-theme--light .plugin-install__card-title,
.v-theme--light .plugin-install__fact-value,
.v-theme--light :deep(.agent-badge) {
  color: #0f172a;
}

.v-theme--light .plugin-install__subtitle,
.v-theme--light .plugin-install__card-copy,
.v-theme--light .plugin-install__channel-description,
.v-theme--light .plugin-install__muted-note,
.v-theme--light .plugin-install__microcopy {
  color: #475569;
}

.v-theme--light .plugin-install__platform-note {
  color: #92400e;
}

.v-theme--light .plugin-install__target-tab,
.v-theme--light .plugin-install__channel-tab {
  color: #0f172a;
  border-color: rgba(15, 23, 42, 0.08);
  background: rgba(241, 245, 249, 0.8);
}

.v-theme--light .plugin-install__expand-cta {
  color: #0f172a !important;
}

.v-theme--light .plugin-install__target-divider {
  background: rgba(15, 23, 42, 0.1);
}

.v-theme--light .plugin-install__section-divider {
  background: rgba(15, 23, 42, 0.08);
}

.v-theme--light .plugin-install__fact {
  background: rgba(248, 250, 252, 0.88);
  border-color: rgba(15, 23, 42, 0.06);
}

.v-theme--light .plugin-install__manual-link {
  color: #0f172a;
  border-color: rgba(15, 23, 42, 0.08);
  background: rgba(248, 250, 252, 0.88);
}

.v-theme--light .plugin-install__manual-link:hover {
  background: rgba(241, 245, 249, 0.96);
}

.v-theme--light .plugin-install__fact-pill {
  color: #0f172a;
  background: rgba(241, 245, 249, 0.92);
  border-color: rgba(15, 23, 42, 0.08);
}

.v-theme--light .plugin-install__boundary {
  color: #854d0e;
}

.v-theme--light .plugin-install__boundary--subtle {
  color: #0f766e;
}

@media (max-width: 960px) {
  .plugin-install__grid {
    grid-template-columns: 1fr;
  }

  .plugin-install__facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .plugin-install__facts {
    grid-template-columns: 1fr;
  }

  .plugin-install__manual-links {
    grid-template-columns: 1fr;
  }

  .plugin-install__manual-link--repo {
    grid-column: auto;
  }
}

@media (max-width: 640px) {
  .plugin-install__card {
    padding: 20px;
    border-radius: 20px;
  }

  .plugin-install__toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .plugin-install__card-head {
    grid-template-columns: 1fr;
  }

  .plugin-install__step-index {
    width: fit-content;
    min-width: 54px;
  }

  .plugin-install__secondary-cta,
  .plugin-install__manual-link {
    width: 100%;
  }

  .plugin-install__target-divider {
    display: none;
  }
}
</style>
