<script setup lang="ts">
import { clientLandingById } from '~/data/clients';

const { asset } = useSite();
</script>

<template>
  <ul class="client-strip" aria-label="Supported agents">
    <li v-for="client in clients" :key="client.id">
      <NuxtLink :to="`/agents/${clientLandingById.get(client.id)?.slug}/`">
        <span class="client-strip__icon"
          ><img :src="asset(`client-icons/${client.icon}`)" alt="" width="24" height="24"
        /></span>
        <span class="client-strip__copy"
          ><strong>{{ client.name }}</strong
          ><small>{{ client.status }}</small></span
        >
      </NuxtLink>
    </li>
  </ul>
</template>
