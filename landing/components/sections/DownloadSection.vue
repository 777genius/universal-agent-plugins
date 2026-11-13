<script setup lang="ts">
import CommandSnippetCard from '~/components/shared/CommandSnippetCard.vue';

withDefaults(defineProps<{ headingTag?: 'h1' | 'h2' }>(), { headingTag: 'h2' });

const { t, locale } = useI18n();
const { content } = await useDownloadContent();
const { data: releaseData, fallbackUrl } = useReleaseDownloads();

const releaseVersion = computed(() => {
  const version = releaseData.value?.version;
  return version?.match(/^(?:agentplugins-)?v?(\d+\.\d+\.\d+(?:-[\w.-]+)?(?:\+[\w.-]+)?)$/)?.[1] || null;
});
const releaseDate = computed(() =>
  releaseData.value?.pubDate ? formatReleaseDate(releaseData.value.pubDate, locale.value) : '',
);

// Put npx first, but keep platform-aware native recommendations and manual choice.
const channelOrder = ['npm', 'brew', 'script', 'powershell'];
const installChannels = computed(() =>
  content.value.installChannels
    .filter((channel) => channel.command && channelOrder.includes(channel.id))
    .sort((a, b) => channelOrder.indexOf(a.id) - channelOrder.indexOf(b.id)),
);
const {
  detectedInstallPlatform,
  recommendedChannelId,
  selectedInstallChannelId: selectedInstallId,
  selectInstallChannel,
} = useInstallChannelSelection(installChannels);
const selectedInstallChannel = computed(
  () =>
    installChannels.value.find((channel) => channel.id === selectedInstallId.value) ??
    installChannels.value[0] ??
    null,
);
const selectedCommand = computed(() => {
  const channel = selectedInstallChannel.value;
  if (!channel) return '';
  // npx executes a plugin command without installing a persistent CLI.
  return channel.id === 'npm' ? channel.invocation + ' add context7' : (channel.command ?? '');
});

function onMethodTabKeydown(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;

  const tablist = event.currentTarget;
  if (!(tablist instanceof HTMLElement)) return;
  const tabs = Array.from(tablist.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
  const activeIndex = tabs.indexOf(event.target as HTMLButtonElement);
  if (activeIndex < 0 || tabs.length === 0) return;

  event.preventDefault();
  const nextIndex =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? tabs.length - 1
        : (activeIndex + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
  const channel = installChannels.value[nextIndex];
  if (channel) {
    selectInstallChannel(channel.id);
    tabs[nextIndex]?.focus();
  }
}
</script>

<template>
  <section id="download" class="download-section section anchor-offset">
    <v-container>
      <header class="download-section__header">
        <component :is="headingTag" class="download-section__title">
          {{ content.download.title }}
        </component>
        <p class="download-section__subtitle">{{ content.download.note }}</p>
        <p v-if="releaseVersion" class="download-section__release-info">
          {{ t('download.latestRelease') }} ·
          <a :href="fallbackUrl" target="_blank" rel="noopener noreferrer">v{{ releaseVersion }}</a>
          <template v-if="releaseDate"> · {{ releaseDate }}</template>
        </p>
      </header>

      <div class="download-section__overview">
        <article class="download-section__card">
          <div class="download-section__install-tabs">
            <p id="download-methods-label" class="download-section__install-tabs-label">
              {{ t('download.installTabsLabel') }}
            </p>
            <div class="download-section__install-tabs-row" role="tablist" aria-labelledby="download-methods-label" @keydown="onMethodTabKeydown">
              <button
                v-for="channel in installChannels"
                :key="channel.id"
                type="button"
                class="download-section__install-tab"
                :class="{ 'download-section__install-tab--active': channel.id === selectedInstallId }"
                role="tab"
                :id="'download-method-tab-' + channel.id"
                aria-controls="download-method-panel"
                :aria-selected="channel.id === selectedInstallId"
                :tabindex="channel.id === selectedInstallId ? 0 : -1"
                @click="selectInstallChannel(channel.id)"
              >
                <span>{{ channel.title }}</span>
                <span v-if="channel.id === recommendedChannelId" class="download-section__install-tab-badge">
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
              v-else-if="detectedInstallPlatform && detectedInstallPlatform !== 'other' && selectedInstallId === recommendedChannelId"
              class="download-section__platform-note"
            >
              {{ t('download.detectedRecommendation', { platform: t('download.platforms.' + detectedInstallPlatform) }) }}
            </p>
          </div>

          <div
            v-if="selectedInstallChannel"
            id="download-method-panel"
            class="download-section__method"
            role="tabpanel"
            :aria-labelledby="'download-method-tab-' + selectedInstallChannel.id"
            tabindex="0"
          >
            <div class="download-section__method-header">
              <div class="download-section__method-copy">
                <h2 class="download-section__method-title">{{ selectedInstallChannel.title }}</h2>
                <p class="download-section__method-description">{{ selectedInstallChannel.description }}</p>
              </div>
              <a
                class="download-section__method-link"
                :href="selectedInstallChannel.href"
                target="_blank"
                rel="noopener noreferrer"
              >
                {{ t('download.open') }} ↗
              </a>
            </div>
            <div class="download-section__command">
              <CommandSnippetCard
                :label="t('download.command')"
                :command="selectedCommand"
                :copy-label="t('download.copy')"
                :copied-label="t('download.copied')"
              />
            </div>
            <p v-if="selectedInstallChannel.note" class="download-section__method-note">
              {{ selectedInstallChannel.note }}
            </p>
          </div>
        </article>
      </div>
    </v-container>
  </section>
</template>

<style scoped>
.download-section__header {
  max-width: 760px;
  margin: 0 auto 30px;
  text-align: center;
}

.download-section__title {
  display: block;
  margin: 0 0 12px;
  font-size: clamp(2rem, 4vw, 2.65rem);
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
  background: linear-gradient(135deg, #e0e6ff, #00f0ff);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.download-section__subtitle {
  margin: 0;
  color: #a8b3d1;
  font-size: 1rem;
  line-height: 1.6;
}

.download-section__release-info {
  margin: 12px 0 0;
  color: #8892b0;
  font-size: 0.78rem;
  line-height: 1.5;
}

.download-section__release-info a,
.download-section__method-link {
  color: #00f0ff;
  text-underline-offset: 3px;
}

.download-section__overview {
  max-width: 880px;
  margin: 0 auto;
}

.download-section__card {
  min-width: 0;
  padding: clamp(18px, 3vw, 32px);
  border: 1px solid rgba(0, 240, 255, 0.14);
  border-radius: 20px;
  background: linear-gradient(180deg, rgba(0, 240, 255, 0.055), rgba(10, 10, 15, 0.85));
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.12);
}

.download-section__install-tabs {
  display: grid;
  gap: 12px;
}

.download-section__install-tabs-label {
  margin: 0;
  color: #a8b3d1;
  font-size: 0.85rem;
  font-weight: 600;
}

.download-section__install-tabs-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.download-section__install-tab {
  display: flex;
  min-width: 0;
  min-height: 64px;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 6px;
  padding: 11px 14px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.035);
  color: #dbeafe;
  cursor: pointer;
  font-size: 0.82rem;
  font-weight: 650;
  line-height: 1.35;
  text-align: start;
  overflow-wrap: anywhere;
  transition: border-color 0.15s, background 0.15s;
}

.download-section__install-tab:hover {
  border-color: rgba(0, 240, 255, 0.4);
}

.download-section__install-tab:focus-visible,
.download-section__method-link:focus-visible {
  outline: 2px solid #00f0ff;
  outline-offset: 3px;
}

.download-section__install-tab--active {
  border-color: rgba(0, 240, 255, 0.68);
  background: rgba(0, 240, 255, 0.12);
  color: #e8fbff;
}

.download-section__install-tab-badge {
  color: #99e8d7;
  font-size: 0.67rem;
  font-weight: 700;
}

.download-section__platform-note {
  margin: 0;
  color: #7dd3fc;
  font-size: 0.79rem;
  line-height: 1.45;
}

.download-section__platform-note--warning {
  color: #fcd34d;
}

.download-section__method {
  display: grid;
  min-width: 0;
  gap: 16px;
  margin-top: 22px;
  padding-top: 22px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.download-section__method-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.download-section__method-copy {
  min-width: 0;
}

.download-section__method-title {
  margin: 0 0 6px;
  color: #e0e6ff;
  font-size: 1.18rem;
  font-weight: 700;
}

.download-section__method-description,
.download-section__method-note {
  margin: 0;
  color: #a8b3d1;
  font-size: 0.88rem;
  line-height: 1.55;
}

.download-section__method-link {
  flex-shrink: 0;
  font-size: 0.82rem;
  line-height: 1.5;
}

.download-section__command {
  min-width: 0;
}

.download-section__command :deep(.command-snippet__body) {
  direction: ltr;
  text-align: left;
}

.v-theme--light .download-section__title {
  background: linear-gradient(135deg, #1e293b, #0891b2);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.v-theme--light .download-section__subtitle,
.v-theme--light .download-section__release-info,
.v-theme--light .download-section__install-tabs-label,
.v-theme--light .download-section__method-description,
.v-theme--light .download-section__method-note {
  color: #475569;
}

.v-theme--light .download-section__card {
  border-color: rgba(15, 23, 42, 0.1);
  background: linear-gradient(180deg, rgba(8, 145, 178, 0.07), rgba(255, 255, 255, 0.95));
  box-shadow: 0 18px 45px rgba(15, 23, 42, 0.08);
}

.v-theme--light .download-section__install-tab {
  border-color: rgba(15, 23, 42, 0.12);
  background: rgba(241, 245, 249, 0.75);
  color: #0f172a;
}

.v-theme--light .download-section__install-tab--active {
  border-color: #0891b2;
  background: rgba(8, 145, 178, 0.12);
}

.v-theme--light .download-section__install-tab-badge,
.v-theme--light .download-section__platform-note {
  color: #0e7490;
}

.v-theme--light .download-section__method-title {
  color: #0f172a;
}

.v-theme--light .download-section__method-link,
.v-theme--light .download-section__release-info a {
  color: #0e7490;
}

.v-theme--light .download-section__method {
  border-color: rgba(15, 23, 42, 0.12);
}

@media (max-width: 900px) {
  .download-section__install-tabs-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 520px) {
  .download-section__header {
    margin-bottom: 22px;
  }

  .download-section__method-header {
    flex-direction: column;
    gap: 6px;
  }
}
</style>
