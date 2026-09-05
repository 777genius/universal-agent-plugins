<script setup lang="ts">
import CommandSnippetCard from '~/components/shared/CommandSnippetCard.vue';
import { clientLandingPages } from '~/data/clients';
import { applyCliInvocation, getCliInvocation } from '~/utils/cliInvocation';

withDefaults(defineProps<{ headingTag?: 'h1' | 'h2' }>(), { headingTag: 'h2' });

const { content } = useLandingContent();
const { t, locale } = useI18n();
const { data: releaseData, fallbackUrl } = useReleaseDownloads();
const { quickstartUrl, supportBoundaryUrl } = useDocsLinks();

const releaseVersion = computed(() => {
  const version = releaseData.value?.version;
  return (
    version?.match(/^(?:agentplugins-)?v?(\d+\.\d+\.\d+(?:-[\w.-]+)?(?:\+[\w.-]+)?)$/)?.[1] || null
  );
});
const releaseDate = computed(() => {
  if (!releaseData.value?.pubDate) {
    return '';
  }

  return formatReleaseDate(releaseData.value.pubDate, locale.value);
});

const supportAccent = ['#39ff14', '#00f0ff', '#ffb703', '#f472b6', '#94a3b8'];

const installChannels = computed(() =>
  content.value.installChannels
    .filter((channel) => ['brew', 'script', 'powershell', 'npm'].includes(channel.id))
    .map((channel) =>
      channel.id === 'docs' ? { ...channel, href: quickstartUrl.value } : channel,
    ),
);

const quickstartInstallChannels = computed(() =>
  installChannels.value.filter(
    (channel) => channel.command && !['docs', 'releases'].includes(channel.id),
  ),
);

const {
  detectedInstallPlatform,
  recommendedChannelId,
  selectedInstallChannelId: selectedInstallId,
  selectInstallChannel,
} = useInstallChannelSelection(quickstartInstallChannels);

const selectedInstallChannel = computed(
  () =>
    quickstartInstallChannels.value.find((channel) => channel.id === selectedInstallId.value) ||
    quickstartInstallChannels.value[0] ||
    null,
);

const selectedCliInvocation = computed(() =>
  getCliInvocation(selectedInstallChannel.value?.invocation),
);

const quickstartSteps = computed(() => {
  const steps =
    selectedInstallChannel.value?.id === 'npm'
      ? content.value.quickstartSteps.filter((step) => step.id !== 'install-cli')
      : content.value.quickstartSteps;

  return steps.map((step) => {
    if (step.id === 'install-cli' && selectedInstallChannel.value?.command) {
      const command =
        selectedInstallChannel.value.id === 'npm'
          ? selectedInstallChannel.value.command
          : `${selectedInstallChannel.value.command}\n${selectedCliInvocation.value} version`;
      return {
        ...step,
        command,
        note: selectedInstallChannel.value.note ?? step.note,
      };
    }

    if (step.id === 'try-plugin') {
      return {
        ...step,
        command: applyCliInvocation(step.command, selectedCliInvocation.value),
      };
    }

    if (step.id === 'build-plugin' || step.id === 'validate') {
      return {
        ...step,
        command: applyCliInvocation(step.command, selectedCliInvocation.value),
      };
    }

    return step;
  });
});
</script>

<template>
  <section id="download" class="download-section section anchor-offset">
    <v-container>
      <div class="download-section__header">
        <component :is="headingTag" class="download-section__title">{{
          content.download.title
        }}</component>
        <p class="download-section__subtitle">{{ content.download.note }}</p>
        <p v-if="releaseVersion" class="download-section__release-info">
          {{ t('download.latestRelease') }} ·
          <a :href="fallbackUrl" target="_blank" rel="noopener noreferrer">v{{ releaseVersion }}</a>
          <template v-if="releaseDate"> · {{ releaseDate }}</template>
        </p>
      </div>

      <div class="download-section__overview">
        <article
          class="download-section__overview-card download-section__overview-card--quickstart"
        >
          <div class="download-section__overview-top">
            <h3 class="download-section__overview-title">
              {{ t('download.quickstartTitle') }}
            </h3>
            <p class="download-section__overview-subtitle">
              {{ t('download.quickstartSubtitle') }}
            </p>
          </div>

          <div class="download-section__install-tabs">
            <div class="download-section__install-tabs-label">
              {{ t('download.installTabsLabel') }}
            </div>
            <div class="download-section__install-tabs-row">
              <button
                v-for="channel in quickstartInstallChannels"
                :key="channel.id"
                type="button"
                class="download-section__install-tab"
                :class="{
                  'download-section__install-tab--active': channel.id === selectedInstallId,
                }"
                :aria-pressed="channel.id === selectedInstallId"
                @click="selectInstallChannel(channel.id)"
              >
                <span>{{ channel.title }}</span>
                <span
                  v-if="channel.id === recommendedChannelId"
                  class="download-section__install-tab-badge"
                >
                  {{ t('download.recommended') }}
                </span>
              </button>
            </div>
            <p
              v-if="detectedInstallPlatform === 'mobile'"
              class="download-section__platform-note download-section__platform-note--warning"
            >
              {{ t('download.mobileUnsupported') }}
            </p>
            <p
              v-else-if="detectedInstallPlatform && detectedInstallPlatform !== 'other'"
              class="download-section__platform-note"
            >
              {{
                t('download.detectedRecommendation', {
                  platform: t(`download.platforms.${detectedInstallPlatform}`),
                })
              }}
            </p>
            <p v-if="selectedInstallChannel" class="download-section__install-tabs-note">
              {{ selectedInstallChannel.description }}
            </p>
          </div>

          <div class="download-section__steps">
            <div
              v-for="(step, index) in quickstartSteps"
              :key="step.id"
              class="download-section__step"
            >
              <div class="download-section__step-index">0{{ index + 1 }}</div>
              <div class="download-section__step-body">
                <h4 class="download-section__step-title">{{ step.title }}</h4>
                <CommandSnippetCard
                  :label="t('download.command')"
                  :command="step.command"
                  :copy-label="t('download.copy')"
                  :copied-label="t('download.copied')"
                />
                <p class="download-section__step-note">{{ step.note }}</p>
              </div>
            </div>
          </div>
        </article>
      </div>

      <article class="download-section__overview-card download-section__overview-card--support">
        <div class="download-section__overview-top">
          <h3 class="download-section__overview-title">
            {{ t('download.supportTitle') }}
          </h3>
          <p class="download-section__overview-subtitle">
            {{ t('download.supportSubtitle') }}
          </p>
        </div>

        <div class="download-section__support-list">
          <div
            v-for="(client, index) in clientLandingPages"
            :key="client.id"
            class="download-section__support-item"
            :style="{ '--accent': supportAccent[index % supportAccent.length] }"
          >
            <div class="download-section__support-main">
              <h4 class="download-section__support-name">
                <NuxtLink :to="`/agents/${client.slug}/`">{{ client.name }}</NuxtLink>
              </h4>
              <span class="download-section__support-status">{{ client.status }}</span>
            </div>
            <p class="download-section__support-note">{{ client.note }}. {{ client.activation }}</p>
          </div>
        </div>

        <a
          class="download-section__support-link"
          :href="supportBoundaryUrl"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ t('download.supportLink') }}
        </a>
      </article>
    </v-container>
  </section>
</template>

<style scoped>
.download-section {
  position: relative;
}

.download-section__header {
  text-align: center;
  max-width: 620px;
  margin: 0 auto 56px;
  position: relative;
  z-index: 1;
}

.download-section__title {
  font-size: 2.4rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #e0e6ff 0%, #00f0ff 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.download-section__subtitle {
  font-size: 1.1rem;
  color: #8892b0;
  line-height: 1.6;
  margin: 0;
}

.download-section__release-info {
  text-align: center;
  margin: 16px 0 0;
  font-size: 0.82rem;
  color: #8892b0;
  font-family: 'JetBrains Mono', monospace;
}

.download-section__release-info a {
  color: #00f0ff;
  text-decoration: none;
}

.download-section__release-info a:hover {
  text-decoration: underline;
}

.download-section__overview {
  display: block;
  max-width: 1040px;
  margin: 0 auto 24px;
  position: relative;
  z-index: 1;
}

.download-section__overview-card {
  border-radius: 20px;
  border: 1px solid rgba(0, 240, 255, 0.1);
  background: rgba(10, 10, 15, 0.82);
  backdrop-filter: blur(16px);
  padding: 24px;
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.2);
}

.download-section__overview-card--quickstart {
  background: linear-gradient(180deg, rgba(0, 240, 255, 0.07), rgba(10, 10, 15, 0.82));
}

.download-section__overview-card--support {
  max-width: 1040px;
  margin: 24px auto 0;
}

.download-section__overview-top {
  margin-bottom: 18px;
}

.download-section__overview-title {
  margin: 0 0 8px;
  font-size: 1.2rem;
  font-weight: 700;
  color: #e0e6ff;
}

.download-section__overview-subtitle {
  margin: 0;
  font-size: 0.92rem;
  line-height: 1.6;
  color: #95a3c4;
}

.download-section__install-tabs {
  display: grid;
  gap: 12px;
  margin-bottom: 22px;
}

.download-section__install-tabs-label {
  color: #8892b0;
  font-size: 0.74rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.download-section__install-tabs-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.download-section__install-tab {
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

.download-section__install-tab:hover {
  transform: translateY(-1px);
  border-color: rgba(0, 240, 255, 0.18);
}

.download-section__install-tab--active {
  color: #0a0a0f;
  border-color: rgba(0, 240, 255, 0.28);
  background: linear-gradient(135deg, #00f0ff, #62f1ff);
  box-shadow: 0 8px 24px rgba(0, 240, 255, 0.18);
}

.download-section__install-tab-badge {
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

.download-section__install-tab--active .download-section__install-tab-badge {
  color: #0a0a0f;
  background: rgba(10, 10, 15, 0.1);
  border-color: rgba(10, 10, 15, 0.1);
}

.download-section__install-tabs-note {
  margin: 0;
  color: #a8b3d1;
  font-size: 0.88rem;
  line-height: 1.55;
}

.download-section__platform-note {
  margin: 0;
  color: #00f0ff;
  font-size: 0.8rem;
  font-family: 'JetBrains Mono', monospace;
}

.download-section__platform-note--warning {
  color: #ffb703;
}

.download-section__steps {
  display: grid;
  gap: 14px;
}

.download-section__step {
  display: grid;
  grid-template-columns: 54px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}

.download-section__step-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  border-radius: 12px;
  border: 1px solid rgba(0, 240, 255, 0.16);
  background: rgba(0, 240, 255, 0.06);
  color: #00f0ff;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  font-family: 'JetBrains Mono', monospace;
}

.download-section__step-body {
  display: grid;
  gap: 8px;
}

.download-section__step-title {
  margin: 0;
  font-size: 0.98rem;
  color: #e0e6ff;
}

.download-section__step-command {
  display: block;
  margin: 0;
  padding: 14px 16px;
  border-radius: 12px;
  background:
    linear-gradient(180deg, rgba(0, 240, 255, 0.02), rgba(0, 0, 0, 0.2)), rgba(0, 0, 0, 0.26);
  border: 1px solid rgba(255, 255, 255, 0.07);
  font-size: 0.8rem;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-x: auto;
  font-family: 'JetBrains Mono', monospace;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
}

.download-section__step-note {
  margin: 0;
  color: #95a3c4;
  line-height: 1.55;
  font-size: 0.85rem;
}

.download-section__support-list {
  display: grid;
  gap: 12px;
}

.download-section__support-link {
  display: inline-flex;
  align-items: center;
  margin-top: 16px;
  color: #00f0ff;
  text-decoration: none;
  font-size: 0.85rem;
  font-weight: 600;
}

.download-section__support-link:hover {
  text-decoration: underline;
}

.download-section__support-item {
  padding: 14px 16px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--accent) 18%, transparent);
  background: color-mix(in srgb, var(--accent) 7%, rgba(255, 255, 255, 0.02));
}

.download-section__support-main {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.download-section__support-name {
  margin: 0;
  font-size: 0.96rem;
  font-weight: 700;
  color: #e0e6ff;
}

.download-section__support-name a {
  color: inherit;
  text-underline-offset: 3px;
}

.download-section__support-status {
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 0.68rem;
  line-height: 1;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 700;
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 32%, transparent);
  background: color-mix(in srgb, var(--accent) 8%, transparent);
}

.download-section__support-note {
  margin: 0;
  color: #95a3c4;
  font-size: 0.85rem;
  line-height: 1.55;
}

.download-section__command-wrap {
  margin-bottom: 14px;
}

.download-section__command-head {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.download-section__command-label {
  display: block;
  color: #8892b0;
  font-size: 0.7rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.download-section__copy-btn {
  appearance: none;
  border: 1px solid rgba(0, 240, 255, 0.16);
  background: rgba(0, 240, 255, 0.07);
  color: #00f0ff;
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 0.68rem;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.download-section__copy-btn:hover {
  transform: translateY(-1px);
  border-color: rgba(0, 240, 255, 0.28);
  background: rgba(0, 240, 255, 0.12);
}

.download-section__copy-btn:focus-visible {
  outline: 2px solid rgba(0, 240, 255, 0.4);
  outline-offset: 2px;
}

.download-section__command-line {
  display: block;
}

.download-section__token {
  color: #dbeafe;
}

.download-section__token--command {
  color: #67e8f9;
}

.download-section__token--action {
  color: #f0abfc;
}

.download-section__token--flag {
  color: #facc15;
}

.download-section__token--path {
  color: #86efac;
}

.download-section__token--url {
  color: #38bdf8;
}

.download-section__token--operator {
  color: #c084fc;
}

@keyframes downloadFadeUp {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.v-theme--light .download-section__title {
  background: linear-gradient(135deg, #1e293b 0%, #0891b2 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.v-theme--light .download-section__subtitle,
.v-theme--light .download-section__release-info {
  color: #475569;
}

.v-theme--light .download-section__overview-card {
  background: rgba(255, 255, 255, 0.9);
  border-color: rgba(15, 23, 42, 0.08);
  box-shadow: 0 18px 45px rgba(15, 23, 42, 0.08);
}

.v-theme--light .download-section__overview-card--quickstart {
  background: linear-gradient(180deg, rgba(14, 165, 233, 0.08), rgba(255, 255, 255, 0.9));
}

.v-theme--light .download-section__overview-title,
.v-theme--light .download-section__step-title,
.v-theme--light .download-section__support-name {
  color: #0f172a;
}

.v-theme--light .download-section__overview-subtitle,
.v-theme--light .download-section__install-tabs-label,
.v-theme--light .download-section__install-tabs-note,
.v-theme--light .download-section__step-note,
.v-theme--light .download-section__support-note,
.v-theme--light .download-section__command-label {
  color: #475569;
}

.v-theme--light .download-section__step-command {
  background: rgba(241, 245, 249, 0.9);
  border-color: rgba(15, 23, 42, 0.06);
}

.v-theme--light .download-section__token {
  color: #1e293b;
}

.v-theme--light .download-section__token--command {
  color: #0891b2;
}

.v-theme--light .download-section__token--action {
  color: #c026d3;
}

.v-theme--light .download-section__token--flag {
  color: #ca8a04;
}

.v-theme--light .download-section__token--path {
  color: #15803d;
}

.v-theme--light .download-section__token--url {
  color: #2563eb;
}

.v-theme--light .download-section__token--operator {
  color: #7c3aed;
}

.v-theme--light .download-section__copy-btn {
  color: #0891b2;
  border-color: rgba(8, 145, 178, 0.18);
  background: rgba(8, 145, 178, 0.08);
}

.v-theme--light .download-section__install-tab {
  color: #0f172a;
  border-color: rgba(15, 23, 42, 0.08);
  background: rgba(241, 245, 249, 0.8);
}

.v-theme--light .download-section__install-tab--active {
  color: #082f49;
  border-color: rgba(8, 145, 178, 0.2);
  background: linear-gradient(135deg, #67e8f9, #22d3ee);
}

.v-theme--light .download-section__platform-note--warning {
  color: #92400e;
}

@media (max-width: 700px) {
  .download-section__step {
    grid-template-columns: 1fr;
  }

  .download-section__step-index {
    width: fit-content;
    min-width: 54px;
  }
}
</style>
