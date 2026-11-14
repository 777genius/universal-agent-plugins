<script setup lang="ts">
import { clients } from '~/data/clients';

const { asset } = useSite();
// Shared branding appears once here; actual client support and selection stay unchanged.
const orbitClients = clients.filter(
  (client, index) => clients.findIndex((candidate) => candidate.icon === client.icon) === index,
);
</script>

<template>
  <div class="hero-agent-field__plane">
    <div class="hero-agent-field__track">
      <div class="hero-agent-field__orbit">
        <span
          v-for="(client, index) in orbitClients"
          :key="client.id"
          class="hero-agent-field__spoke"
          :data-client-id="client.id"
          :style="{ '--angle': `${(index * 360) / orbitClients.length}deg` }"
        >
          <span class="hero-agent-field__node">
            <span class="hero-agent-field__rotor">
              <span
                v-for="depth in 4"
                :key="depth"
                class="hero-agent-field__rim"
                :style="{ '--rim-depth': `${-depth * 1.5}px` }"
              />
              <span class="hero-agent-field__face">
                <img :src="asset(`client-icons/${client.icon}`)" alt="" width="26" height="26" />
              </span>
            </span>
          </span>
        </span>
      </div>
    </div>
    <span class="hero-agent-field__hub">
      <img :src="asset('icon.svg')" alt="" width="38" height="38" />
    </span>
  </div>
</template>
