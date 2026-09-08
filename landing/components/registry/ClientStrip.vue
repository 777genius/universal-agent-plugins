<script setup lang="ts">
import { isKnownLocale } from '~/data/i18n';
import { localizedPath } from '~/utils/localizedRoutes';
import { clientLandingById } from '~/data/clients';

const { t, locale } = useI18n();
const clientPath = (slug: string) =>
  localizedPath(`/agents/${slug}/`, isKnownLocale(locale.value) ? locale.value : 'en');
const { asset } = useSite();
</script>

<template>
  <ul class="client-strip" :aria-label="t('shell.accessibility.supportedAgents')">
    <li v-for="client in clients" :key="client.id">
      <NuxtLink :to="clientPath(clientLandingById.get(client.id)!.slug)">
        <span class="client-strip__icon"
          ><img :src="asset(`client-icons/${client.icon}`)" alt="" width="24" height="24"
        ></span>
        <span class="client-strip__copy"
          ><strong>{{ client.name }}</strong
          ><small>{{ t(`registryUi.clients.${client.id}.status`) }}</small></span
        >
      </NuxtLink>
    </li>
  </ul>
</template>
