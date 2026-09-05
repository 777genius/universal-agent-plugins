<script setup lang="ts">
import { mdiArrowLeft, mdiOpenInNew } from '@mdi/js';
import { clientLandingBySlug } from '~/data/clients';

const route = useRoute();
const registry = useRegistry();
const config = useRuntimeConfig();
const { asset, pluginIcon } = useSite();
const client = clientLandingBySlug.get(String(route.params.client));

if (!client) {
  throw createError({ statusCode: 404, statusMessage: 'Agent not found' });
}

const reviewedPlugins = registry.plugins.filter(
  (plugin) =>
    plugin.trust_state !== 'conformant_unreviewed' &&
    plugin.client_support.clients.includes(client.id),
);
const siteUrl = String(config.public.siteUrl).replace(/\/+$/, '');
const pageUrl = `${siteUrl}/agents/${client.slug}/`;
const breadcrumbId = `${pageUrl}#breadcrumb`;
const listId = `${pageUrl}#plugin-list`;
const title = `Agent Plugins for ${client.name} | Universal Agent Plugins`;
const installCommand = `npx universal-agent-plugins add context7 --target ${client.id}`;

usePageSeo(title, client.intro, {
  translate: false,
  pageType: 'CollectionPage',
  canonicalPath: `/agents/${client.slug}/`,
  pageProperties: {
    breadcrumb: { '@id': breadcrumbId },
    mainEntity: { '@id': listId },
  },
  structuredData: [
    {
      '@type': 'ItemList',
      '@id': listId,
      name: `Reviewed Agent Plugins 1.0 packages for ${client.name}`,
      numberOfItems: reviewedPlugins.length,
      itemListElement: reviewedPlugins.map((plugin, index) => ({
        '@type': 'ListItem',
        position: index + 1,
        name: plugin.display_name,
        url: `${siteUrl}/plugins/${plugin.name}/`,
      })),
    },
    {
      '@type': 'BreadcrumbList',
      '@id': breadcrumbId,
      itemListElement: [
        {
          '@type': 'ListItem',
          position: 1,
          name: 'Universal Agent Plugins',
          item: `${siteUrl}/`,
        },
        {
          '@type': 'ListItem',
          position: 2,
          name: client.name,
          item: pageUrl,
        },
      ],
    },
  ],
});
</script>

<template>
  <main class="agent-page registry-surface">
    <PageBackground />
    <section class="agent-page__hero section">
      <v-container>
        <nav class="agent-page__breadcrumbs" aria-label="Breadcrumb">
          <NuxtLink to="/" aria-label="Back to Universal Agent Plugins">
            <v-icon :icon="mdiArrowLeft" size="18" /> Home
          </NuxtLink>
          <span aria-hidden="true">/</span>
          <span>{{ client.name }}</span>
        </nav>

        <div class="agent-page__hero-grid">
          <div class="agent-page__copy">
            <div class="agent-page__identity">
              <span class="agent-page__icon">
                <img :src="asset(`client-icons/${client.icon}`)" alt="" width="54" height="54" >
              </span>
              <div>
                <p class="eyebrow">Agent Plugins 1.0</p>
                <h1>Install Agent Plugins for {{ client.name }}</h1>
              </div>
            </div>
            <p class="agent-page__intro">{{ client.intro }}</p>
            <span class="agent-page__status">{{ client.status }}</span>
          </div>

          <aside class="agent-page__install" aria-labelledby="agent-install-title">
            <p class="eyebrow">One command</p>
            <h2 id="agent-install-title">Choose a plugin and install it</h2>
            <CommandSnippet :command="installCommand" kind="add" label="Install" />
            <p>
              Try Context7, or replace <code>context7</code> with another compatible plugin's
              reviewed short name or pinned GitHub package source.
            </p>
          </aside>
        </div>
      </v-container>
    </section>

    <section class="agent-page__flow section" aria-labelledby="delivery-title">
      <v-container>
        <div class="section-heading">
          <p class="eyebrow">Native delivery</p>
          <h2 id="delivery-title">What happens for {{ client.name }}</h2>
        </div>
        <div class="agent-page__steps">
          <article>
            <span>01</span>
            <h3>Package delivery</h3>
            <p>{{ client.delivery }}</p>
          </article>
          <article>
            <span>02</span>
            <h3>Activation</h3>
            <p>{{ client.activation }}</p>
          </article>
          <article>
            <span>03</span>
            <h3>Lifecycle</h3>
            <p>Use the same CLI to inspect, update, repair, switch source, or remove the plugin.</p>
          </article>
        </div>
        <a
          v-if="client.vendorDocsUrl"
          class="agent-page__vendor-link"
          :href="client.vendorDocsUrl"
          target="_blank"
          rel="noreferrer noopener"
        >
          Read {{ client.name }} documentation
          <v-icon :icon="mdiOpenInNew" size="16" />
        </a>
      </v-container>
    </section>

    <section class="agent-page__plugins section" aria-labelledby="agent-plugins-title">
      <v-container>
        <div class="section-heading">
          <p class="eyebrow">Reviewed directory</p>
          <h2 id="agent-plugins-title">Plugins available for {{ client.name }}</h2>
          <p>
            {{ reviewedPlugins.length }} reviewed
            {{ reviewedPlugins.length === 1 ? 'package supports' : 'packages support' }} this
            client.
          </p>
        </div>
        <ul class="agent-page__plugin-grid">
          <li v-for="plugin in reviewedPlugins" :key="plugin.name">
            <NuxtLink :to="`/plugins/${plugin.name}/`">
              <span class="agent-page__plugin-icon">
                <img
                  v-if="pluginIcon(plugin)"
                  :src="pluginIcon(plugin)"
                  alt=""
                  width="34"
                  height="34"
                  loading="lazy"
                >
                <span v-else aria-hidden="true">{{ plugin.display_name.slice(0, 1) }}</span>
              </span>
              <span>
                <strong>{{ plugin.display_name }}</strong>
                <small>{{ plugin.description }}</small>
              </span>
            </NuxtLink>
          </li>
        </ul>
        <NuxtLink class="button button--secondary agent-page__directory-link" to="/plugins/">
          Explore the full plugin directory <span aria-hidden="true">→</span>
        </NuxtLink>
      </v-container>
    </section>
  </main>
</template>

<style scoped>
.agent-page {
  position: relative;
  min-height: 100vh;
}

.agent-page__hero {
  padding-top: 24px;
}

.agent-page__breadcrumbs {
  margin-bottom: 22px;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--subtle);
  font-size: 0.82rem;
}

.agent-page__breadcrumbs a {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--muted);
  text-decoration: none;
}

.agent-page__hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(360px, 0.95fr);
  gap: 28px;
  align-items: stretch;
}

.agent-page__copy,
.agent-page__install {
  border: 1px solid var(--line);
  border-radius: 28px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  box-shadow: 0 24px 72px rgba(0, 0, 0, 0.2);
  backdrop-filter: blur(14px);
}

.agent-page__copy {
  padding: clamp(28px, 4vw, 46px);
}

.agent-page__identity {
  display: flex;
  align-items: center;
  gap: 20px;
}

.agent-page__icon,
.agent-page__plugin-icon {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  background: #fff;
  border: 1px solid rgba(255, 255, 255, 0.72);
}

.agent-page__icon {
  width: 82px;
  height: 82px;
  border-radius: 24px;
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.22);
}

.agent-page h1 {
  max-width: 720px;
  margin: 7px 0 0;
  color: var(--text);
  font-size: clamp(2.1rem, 5vw, 4.2rem);
  line-height: 0.98;
  letter-spacing: -0.055em;
}

.agent-page__intro {
  max-width: 760px;
  margin: 28px 0 20px;
  color: var(--muted);
  font-size: 1.08rem;
  line-height: 1.7;
}

.agent-page__status {
  display: inline-flex;
  padding: 7px 12px;
  border: 1px solid rgba(0, 240, 255, 0.24);
  border-radius: 999px;
  color: #67e8f9;
  background: rgba(0, 240, 255, 0.08);
  font-size: 0.76rem;
  font-weight: 750;
}

.agent-page__install {
  padding: 30px;
}

.agent-page__install h2 {
  margin: 8px 0 24px;
  color: var(--text);
  font-size: clamp(1.5rem, 3vw, 2.1rem);
}

.agent-page__install > p:last-child {
  margin: 17px 0 0;
  color: var(--subtle);
  font-size: 0.82rem;
  line-height: 1.6;
}

.agent-page__flow,
.agent-page__plugins {
  padding-block: 72px;
}

.agent-page__steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.agent-page__steps article {
  padding: 25px;
  border: 1px solid var(--line);
  border-radius: 20px;
  background: color-mix(in srgb, var(--surface) 82%, transparent);
}

.agent-page__steps article > span {
  color: #c084fc;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  font-weight: 700;
}

.agent-page__steps h3 {
  margin: 15px 0 8px;
  color: var(--text);
}

.agent-page__steps p {
  margin: 0;
  color: var(--muted);
  line-height: 1.65;
}

.agent-page__vendor-link {
  margin-top: 20px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: #67e8f9;
  text-decoration: none;
}

.agent-page__plugins {
  padding-top: 30px;
}

.agent-page__plugin-grid {
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  list-style: none;
}

.agent-page__plugin-grid a {
  height: 100%;
  padding: 16px;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  border: 1px solid var(--line);
  border-radius: 16px;
  color: var(--text);
  background: color-mix(in srgb, var(--surface) 82%, transparent);
  text-decoration: none;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease;
}

.agent-page__plugin-grid a:hover {
  transform: translateY(-2px);
  border-color: rgba(0, 240, 255, 0.34);
}

.agent-page__plugin-icon {
  width: 46px;
  height: 46px;
  border-radius: 13px;
  color: #080810;
  font-weight: 800;
}

.agent-page__plugin-grid strong,
.agent-page__plugin-grid small {
  display: block;
}

.agent-page__plugin-grid small {
  margin-top: 5px;
  display: -webkit-box;
  overflow: hidden;
  color: var(--subtle);
  font-size: 0.74rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.agent-page__directory-link {
  margin-top: 24px;
}

@media (max-width: 900px) {
  .agent-page__hero-grid,
  .agent-page__steps {
    grid-template-columns: 1fr;
  }

  .agent-page__plugin-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .agent-page__identity {
    align-items: flex-start;
  }

  .agent-page__icon {
    width: 66px;
    height: 66px;
  }

  .agent-page__icon img {
    width: 44px;
    height: 44px;
  }

  .agent-page__copy,
  .agent-page__install {
    padding: 22px;
    border-radius: 22px;
  }

  .agent-page__plugin-grid {
    grid-template-columns: 1fr;
  }
}
</style>
