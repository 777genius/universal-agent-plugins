<script setup lang="ts">
const { t } = useI18n();
const config = useRuntimeConfig();
const playbackId = computed(() => config.public.muxPlaybackId.trim());
const playing = ref(false);
const thumbnailUrl = computed(
  () => `https://image.mux.com/${playbackId.value}/thumbnail.webp?time=1&width=640`,
);
const playerUrl = computed(() => `https://player.mux.com/${playbackId.value}?autoplay=true`);
</script>

<template>
  <figure class="hero-demo-video" :aria-label="t('shell.hero.demoVideoTitle')">
    <iframe
      v-if="playing"
      :src="playerUrl"
      :title="t('shell.hero.demoVideoTitle')"
      allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
      allowfullscreen
    />
    <template v-else-if="playbackId">
      <img :src="thumbnailUrl" :alt="t('shell.hero.demoVideoTitle')" loading="lazy" />
      <button type="button" :aria-label="t('shell.hero.watchDemo')" @click="playing = true">
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="m9 7 8 5-8 5V7Z" fill="currentColor" stroke="none" />
        </svg>
        <span>{{ t('shell.hero.watchDemo') }}</span>
      </button>
    </template>
    <div v-else class="hero-demo-video__preview">
      <div class="hero-demo-video__terminal" aria-hidden="true">
        <span>● ● ●</span>
        <code>$ npx universal-agent-plugins</code>
        <code>✓ Codex &nbsp; ✓ Cursor &nbsp; ✓ VS Code</code>
      </div>
      <div class="hero-demo-video__caption">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="m9 7 8 5-8 5V7Z" fill="currentColor" />
        </svg>
        <span>{{ t('shell.hero.demoVideoTitle') }}</span>
      </div>
    </div>
  </figure>
</template>
