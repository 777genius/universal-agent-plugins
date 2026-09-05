<script setup lang="ts">
const registry = await useRegistryPage({ discovery: true });
const config = useRuntimeConfig();
const description =
  'Search reviewed and community Agent Plugins 1.0 for Codex, Claude Code, Cursor, Gemini CLI, OpenCode, and more.';
const siteUrl = String(config.public.siteUrl).replace(/\/+$/, '');
const listId = `${siteUrl}/plugins/#plugin-list`;
const reviewedPlugins = registry.plugins.filter(
  (plugin) => plugin.trust_state !== 'conformant_unreviewed',
);

usePageSeo('Agent Plugins 1.0 Directory | Search 2,500+ Plugins', description, {
  translate: false,
  pageType: 'CollectionPage',
  pageProperties: { mainEntity: { '@id': listId } },
  structuredData: [
    {
      '@type': 'ItemList',
      '@id': listId,
      name: 'Reviewed Agent Plugins 1.0 packages',
      numberOfItems: reviewedPlugins.length,
      itemListElement: reviewedPlugins.map((plugin, index) => ({
        '@type': 'ListItem',
        position: index + 1,
        name: plugin.display_name,
        url: `${siteUrl}/plugins/${plugin.name}/`,
      })),
    },
  ],
});
</script>

<template>
  <div class="registry-surface directory-page">
    <PageBackground />
    <div class="container">
      <div class="page-intro">
        <p class="eyebrow">Plugin directory</p>
        <h1>Find Agent Plugins 1.0.<br ><em>Use them everywhere.</em></h1>
        <p>
          Search reviewed packages and community plugins discovered from public GitHub repositories.
        </p>
      </div>
      <PluginCatalog
        :plugins="registry.plugins"
        heading="Explore plugins"
        intro="Filter by capability, source, authentication, or supported agent."
      />
    </div>
  </div>
</template>
