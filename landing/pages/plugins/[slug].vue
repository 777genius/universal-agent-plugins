<script setup lang="ts">
import { mdiArrowLeft, mdiDownload, mdiOpenInNew } from '@mdi/js';
import { clientLandingById } from '~/data/clients';
import { seoDescription, spdxLicenseUrl } from '~/utils/seo';
import type { ClientID } from '~/types/registry';

const route = useRoute();
const slug = String(route.params.slug);
const registry = await useRegistryPage({ projection: { kind: 'plugin', value: slug } });
const config = useRuntimeConfig();
const { asset, pluginIcon, sourceUrl } = useSite();
const plugin = registry.plugins.find((item) => item.name === slug);

if (!plugin || plugin.trust_state === 'conformant_unreviewed') {
  throw createError({ statusCode: 404, statusMessage: 'Plugin not found' });
}

const accent = '#00f0ff';
const iconURL = pluginIcon(plugin);
const supportedClients = clients.filter((client) =>
  plugin.client_support.clients.includes(client.id),
);
const clientGroups = [
  {
    label: 'Managed by CLI',
    clients: supportedClients.filter(
      (client) => plugin.client_support.delivery[client.id] === 'managed',
    ),
  },
  {
    label: 'Requires a final step in the app',
    clients: supportedClients.filter((client) =>
      ['prepared', 'manual_activation'].includes(plugin.client_support.delivery[client.id] ?? ''),
    ),
  },
  {
    label: 'Delivery details not specified',
    clients: supportedClients.filter((client) => !plugin.client_support.delivery[client.id]),
  },
].filter((group) => group.clients.length > 0);
const initialTarget =
  supportedClients.find((client) => client.id === 'cursor')?.id ?? supportedClients[0]?.id;
const targets = ref<ClientID[]>(initialTarget ? [initialTarget] : []);
const autoDetect = ref(true);
const trustLabel = 'Reviewed listing';
const siteUrl = String(config.public.siteUrl).replace(/\/+$/, '');
const pluginUrl = `${siteUrl}/plugins/${plugin.name}/`;
const pluginSchemaId = `${pluginUrl}#plugin`;
const breadcrumbId = `${pluginUrl}#breadcrumb`;
const clientNames = supportedClients.map((client) => client.name);
const description = seoDescription([
  `Install the ${plugin.display_name} Agent Plugin for ${clientNames.join(', ')}.`,
  plugin.description,
]);
const licenseUrl = spdxLicenseUrl(plugin.license);

function openInstallSection() {
  document.getElementById('plugin-install')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

usePageSeo(`${plugin.display_name} Agent Plugin | Universal Agent Plugins`, description, {
  translate: false,
  pageProperties: {
    breadcrumb: { '@id': breadcrumbId },
    mainEntity: { '@id': pluginSchemaId },
  },
  structuredData: [
    {
      '@type': 'SoftwareSourceCode',
      '@id': pluginSchemaId,
      name: `${plugin.display_name} Agent Plugin`,
      description: plugin.description,
      url: pluginUrl,
      codeRepository: sourceUrl(plugin),
      softwareVersion: plugin.version,
      runtimePlatform: clientNames,
      keywords: plugin.keywords.join(', '),
      author: {
        '@type': 'Organization',
        name: plugin.author.name,
        ...(plugin.author.url ? { url: plugin.author.url } : {}),
      },
      ...(licenseUrl ? { license: licenseUrl } : {}),
      isPartOf: { '@id': `${siteUrl}/plugins/#webpage` },
    },
    {
      '@type': 'BreadcrumbList',
      '@id': breadcrumbId,
      itemListElement: [
        {
          '@type': 'ListItem',
          position: 1,
          name: 'Plugin directory',
          item: `${siteUrl}/plugins/`,
        },
        {
          '@type': 'ListItem',
          position: 2,
          name: plugin.display_name,
          item: pluginUrl,
        },
      ],
    },
  ],
});
</script>

<template>
  <div class="plugin-detail registry-surface" :style="{ '--accent': accent }">
    <PageBackground />
    <section class="plugin-detail__hero section">
      <v-container>
        <div class="plugin-detail__hero-topbar">
          <v-btn
            to="/plugins/"
            variant="text"
            icon
            aria-label="Back to plugin directory"
            class="plugin-detail__back-cta"
          >
            <v-icon :icon="mdiArrowLeft" size="22" />
          </v-btn>
          <nav class="breadcrumbs" aria-label="Breadcrumb">
            <NuxtLink to="/plugins/">Plugins</NuxtLink><span aria-hidden="true">/</span
            ><span>{{ plugin.display_name }}</span>
          </nav>
        </div>

        <div class="plugin-detail__hero-shell">
          <div class="plugin-detail__hero-copy">
            <div class="plugin-detail__headline">
              <span class="plugin-detail__logo-wrap plugin-detail__logo-wrap--light">
                <img
                  v-if="iconURL"
                  :src="iconURL"
                  alt=""
                  class="plugin-detail__logo"
                  loading="eager"
                >
                <span v-else aria-hidden="true">{{ plugin.display_name.slice(0, 1) }}</span>
              </span>
              <div>
                <h1 class="plugin-detail__title">{{ plugin.display_name }}</h1>
                <p class="plugin-detail__tagline">{{ plugin.description }}</p>
              </div>
            </div>

            <div class="plugin-detail__chips">
              <span class="plugin-detail__type">Agent Plugins 1.0</span>
              <span class="plugin-detail__status">{{ trustLabel }}</span>
              <span
                v-for="category in plugin.categories"
                :key="category"
                class="plugin-detail__category"
                >{{ category }}</span
              >
            </div>

            <p class="plugin-detail__summary">
              The listing and metadata were reviewed. An automated security assessment, when
              present, applies only to the exact scanned revision. Runtime behavior is not audited.
            </p>

            <div class="plugin-detail__actions">
              <v-btn size="x-large" class="plugin-detail__install-cta" @click="openInstallSection">
                Install plugin <v-icon :icon="mdiDownload" end size="20" />
              </v-btn>
              <v-btn
                :href="sourceUrl(plugin)"
                target="_blank"
                rel="noreferrer noopener"
                size="large"
                class="plugin-detail__primary-cta"
              >
                View source <v-icon :icon="mdiOpenInNew" end size="18" />
              </v-btn>
            </div>
          </div>

          <div class="plugin-detail__summary-card">
            <p class="plugin-detail__summary-eyebrow">Plugin overview</p>
            <div
              v-for="group in clientGroups"
              :key="group.label"
              class="plugin-detail__summary-block"
            >
              <div class="plugin-detail__summary-title">{{ group.label }}</div>
              <div class="plugin-detail__client-list">
                <NuxtLink
                  v-for="client in group.clients"
                  :key="client.id"
                  :to="`/agents/${clientLandingById.get(client.id)?.slug}/`"
                  class="plugin-detail__client"
                >
                  <img :src="asset(`client-icons/${client.icon}`)" alt="" width="22" height="22" >
                  {{ client.name }}
                </NuxtLink>
              </div>
            </div>
            <div class="plugin-detail__summary-block">
              <div class="plugin-detail__summary-title">Components</div>
              <ul class="plugin-detail__list">
                <li
                  v-for="component in plugin.components"
                  :key="component"
                  class="plugin-detail__list-item"
                >
                  {{ component }}
                </li>
              </ul>
            </div>
            <div class="plugin-detail__summary-block">
              <div class="plugin-detail__summary-title">Package</div>
              <ul class="plugin-detail__list">
                <li class="plugin-detail__list-item">Version {{ plugin.version }}</li>
                <li class="plugin-detail__list-item">
                  {{ plugin.license || 'License not specified' }}
                </li>
              </ul>
            </div>
          </div>
        </div>
      </v-container>
    </section>

    <section id="plugin-install" class="plugin-detail__installer section">
      <v-container
        ><InstallPanel v-model:targets="targets" v-model:auto-detect="autoDetect" :plugin="plugin"
      /></v-container>
    </section>
  </div>
</template>

<style scoped>
.plugin-detail {
  position: relative;
  min-height: 100vh;
}

.plugin-detail__hero {
  padding-top: 12px;
  padding-bottom: 16px;
}

.plugin-detail__hero-topbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  padding-left: 4px;
}

.plugin-detail__hero-shell {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(300px, 0.75fr);
  gap: 28px;
  align-items: start;
}

.plugin-detail__hero-copy,
.plugin-detail__summary-card,
.plugin-detail__panel {
  position: relative;
  border-radius: 28px;
  border: 1px solid color-mix(in srgb, var(--accent) 16%, rgba(255, 255, 255, 0.08));
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--accent) 14%, transparent) 0%,
      transparent 36%
    ),
    rgba(10, 10, 15, 0.82);
  box-shadow: 0 24px 72px rgba(0, 0, 0, 0.26);
  backdrop-filter: blur(14px);
}

.plugin-detail__hero-copy {
  padding: 32px 30px 30px;
}

.plugin-detail__summary-card {
  padding: 24px;
  height: 100%;
}

.plugin-detail__panel-eyebrow,
.plugin-detail__summary-eyebrow,
.plugin-detail__summary-title {
  margin: 0;
  font-size: 0.72rem;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.plugin-detail__panel-eyebrow {
  color: var(--accent);
}

.plugin-detail__summary-eyebrow,
.plugin-detail__summary-title {
  color: #7dd3fc;
}

.plugin-detail__headline {
  margin-top: 18px;
  display: flex;
  gap: 18px;
  align-items: flex-start;
}

.plugin-detail__logo-wrap {
  width: 72px;
  height: 72px;
  border-radius: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, rgba(255, 255, 255, 0.08));
  flex-shrink: 0;
}

.plugin-detail__logo-wrap--light {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(243, 246, 255, 0.96));
  border-color: rgba(255, 255, 255, 0.64);
  box-shadow:
    0 12px 28px rgba(0, 0, 0, 0.22),
    inset 0 1px 0 rgba(255, 255, 255, 0.88);
}

.plugin-detail__logo {
  width: 38px;
  height: 38px;
  object-fit: contain;
}

.plugin-detail__title {
  margin: 0 0 12px;
  font-size: clamp(2.2rem, 5vw, 4rem);
  line-height: 0.98;
  letter-spacing: -0.05em;
  color: #eff6ff;
}

.plugin-detail__tagline {
  margin: 0;
  color: #d9ecff;
  font-size: 1.05rem;
  line-height: 1.7;
  max-width: 680px;
}

.plugin-detail__chips {
  margin-top: 20px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.plugin-detail__type,
.plugin-detail__status,
.plugin-detail__category {
  border-radius: 999px;
  padding: 9px 12px;
  font-size: 0.8rem;
  font-weight: 700;
}

.plugin-detail__type {
  background: rgba(0, 240, 255, 0.08);
  color: #7dd3fc;
  border: 1px solid rgba(125, 211, 252, 0.16);
}

.plugin-detail__status {
  background: rgba(57, 255, 20, 0.1);
  color: #39ff14;
  border: 1px solid rgba(57, 255, 20, 0.2);
}

.plugin-detail__category {
  background: rgba(255, 255, 255, 0.05);
  color: #dbe7ff;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.plugin-detail__summary {
  margin: 22px 0 0;
  color: #91a0bf;
  font-size: 1rem;
  line-height: 1.8;
  max-width: 720px;
}

.plugin-detail__back-cta {
  color: #dffaff !important;
  background: rgba(255, 255, 255, 0.03) !important;
  border: 1px solid rgba(255, 255, 255, 0.05) !important;
}

.plugin-detail__actions {
  margin-top: 24px;
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.plugin-detail__install-cta {
  min-height: 58px !important;
  padding-inline: 24px !important;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--accent) 82%, #ffffff 18%),
    #d9fbff
  ) !important;
  color: #04131d !important;
  font-weight: 900 !important;
  letter-spacing: 0.03em !important;
  box-shadow: 0 18px 36px color-mix(in srgb, var(--accent) 22%, transparent) !important;
}

.plugin-detail__primary-cta {
  min-height: 54px !important;
  border-color: color-mix(in srgb, var(--accent) 20%, rgba(255, 255, 255, 0.08)) !important;
  background: rgba(255, 255, 255, 0.03) !important;
  color: #dffaff !important;
  font-weight: 800 !important;
}

.plugin-detail__secondary-cta {
  border-color: rgba(125, 211, 252, 0.2) !important;
  color: #dffaff !important;
}

.plugin-detail__summary-block + .plugin-detail__summary-block {
  margin-top: 22px;
}

.plugin-detail__summary-title {
  margin-bottom: 12px;
}

.plugin-detail__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.plugin-detail__client-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.plugin-detail__client {
  min-width: 0;
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  color: #cbd5e1;
  font-size: 0.74rem;
  text-decoration: none;
  transition:
    border-color 0.2s ease,
    color 0.2s ease;
}

.plugin-detail__client:hover {
  color: #fff;
  border-color: rgba(0, 240, 255, 0.32);
}

.plugin-detail__client img {
  padding: 2px;
  border-radius: 6px;
  background: #fff;
}

.plugin-detail__installer {
  padding-top: 18px;
  padding-bottom: 72px;
}

.plugin-detail__installer :deep(.install-panel) {
  position: static;
}

.plugin-detail__sections {
  padding-top: 8px;
}

.plugin-detail__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}

.plugin-detail__panel {
  padding: 26px;
}

.plugin-detail__panel-title {
  margin: 12px 0 0;
  color: #eff6ff;
  font-size: 1.5rem;
}

.plugin-detail__list {
  margin: 18px 0 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 14px;
}

.plugin-detail__list-item {
  position: relative;
  padding-left: 18px;
  color: #a9b7d7;
  line-height: 1.72;
}

.plugin-detail__list-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.75em;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--accent);
  box-shadow: 0 0 18px color-mix(in srgb, var(--accent) 54%, transparent);
}

.v-theme--light .plugin-detail__hero-copy,
.v-theme--light .plugin-detail__summary-card,
.v-theme--light .plugin-detail__panel {
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--accent) 10%, transparent) 0%,
      transparent 36%
    ),
    rgba(255, 255, 255, 0.92);
}

.v-theme--light .plugin-detail__title,
.v-theme--light .plugin-detail__panel-title,
.v-theme--light .plugin-detail__category,
.v-theme--light :deep(.agent-badge) {
  color: #0f172a;
}

.v-theme--light .plugin-detail__logo-wrap--light {
  background: #ffffff;
  border-color: rgba(148, 163, 184, 0.3);
  box-shadow:
    0 10px 26px rgba(15, 23, 42, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.92);
}

.v-theme--light .plugin-detail__tagline,
.v-theme--light .plugin-detail__summary,
.v-theme--light .plugin-detail__list-item {
  color: #475569;
}

.v-theme--light .plugin-detail__install-cta {
  color: #0f172a !important;
}

.v-theme--light .plugin-detail__back-cta,
.v-theme--light .plugin-detail__primary-cta {
  color: #0f172a !important;
}

@media (max-width: 960px) {
  .plugin-detail__hero {
    padding-top: 10px;
    padding-bottom: 8px;
  }

  .plugin-detail__hero-topbar {
    margin-bottom: 8px;
    padding-left: 0;
  }

  .plugin-detail__hero-shell,
  .plugin-detail__grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .plugin-detail__hero {
    padding-top: 8px;
  }

  .plugin-detail__hero-copy,
  .plugin-detail__summary-card,
  .plugin-detail__panel {
    padding: 20px;
    border-radius: 22px;
  }

  .plugin-detail__headline {
    gap: 14px;
  }

  .plugin-detail__logo-wrap {
    width: 58px;
    height: 58px;
    border-radius: 18px;
  }

  .plugin-detail__logo {
    width: 30px;
    height: 30px;
  }

  .plugin-detail__tagline,
  .plugin-detail__summary {
    font-size: 0.95rem;
    line-height: 1.66;
  }

  .plugin-detail__hero-topbar {
    margin-bottom: 6px;
    padding-left: 0;
  }

  .plugin-detail__actions :deep(.v-btn) {
    width: 100%;
  }
}
</style>
