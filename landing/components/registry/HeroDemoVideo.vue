<script setup lang="ts">
const { t } = useI18n();
const config = useRuntimeConfig();
const playbackId = computed(() => config.public.muxPlaybackId.trim());
const playing = ref(false);
const demoContainer = ref<HTMLElement | null>(null);
const playerFrame = ref<HTMLIFrameElement | null>(null);
const thumbnailUrl = computed(
  () => `https://image.mux.com/${playbackId.value}/thumbnail.webp?time=1&width=640`,
);
const playerUrl = computed(() => `https://player.mux.com/${playbackId.value}?autoplay=true`);
const muxOrigin = 'https://player.mux.com';

// Mux iframe embeds support Player.js. We subscribe to 'ended' without loading
// any third-party JavaScript into our page, and only accept messages from our
// own iframe at Mux's fixed origin.
function onPlayerMessage(event: MessageEvent) {
  if (event.origin !== muxOrigin || event.source !== playerFrame.value?.contentWindow) return;
  let message: Record<string, unknown>;
  try {
    message = typeof event.data === 'string' ? JSON.parse(event.data) : event.data;
    if (!message || typeof message !== 'object' || message.context !== 'player.js') return;
  } catch {
    return;
  }
  if (message.event === 'ready') {
    playerFrame.value?.contentWindow?.postMessage(
      JSON.stringify({
        context: 'player.js',
        version: '0.0.11',
        method: 'addEventListener',
        value: 'ended',
        listener: 'uap-demo-ended',
      }),
      muxOrigin,
    );
  }
  if (message.event === 'ended') {
    if (document.fullscreenElement === demoContainer.value)
      void document.exitFullscreen().catch(() => {});
    playing.value = false;
  }
}

function playDemo() {
  if (!playbackId.value) return;
  // Must request fullscreen in the same synchronous user-gesture call stack.
  const request = demoContainer.value?.requestFullscreen?.();
  if (request) void request.catch(() => {});
  playing.value = true;
}

onMounted(() => window.addEventListener('message', onPlayerMessage));
onBeforeUnmount(() => {
  window.removeEventListener('message', onPlayerMessage);
  if (document.fullscreenElement === demoContainer.value)
    void document.exitFullscreen().catch(() => {});
});
</script>

<template>
  <figure
    id="demo"
    ref="demoContainer"
    class="hero-demo-video"
    :aria-label="t('shell.hero.demoVideoTitle')"
  >
    <iframe
      v-if="playing"
      ref="playerFrame"
      :src="playerUrl"
      :title="t('shell.hero.demoVideoTitle')"
      allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
      allowfullscreen
    />
    <template v-else-if="playbackId">
      <img :src="thumbnailUrl" :alt="t('shell.hero.demoVideoTitle')" loading="lazy" />
      <button type="button" :aria-label="t('shell.hero.watchDemo')" @click="playDemo">
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
