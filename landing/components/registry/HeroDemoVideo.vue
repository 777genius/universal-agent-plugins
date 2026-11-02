<script setup lang="ts">
const { t } = useI18n();
const config = useRuntimeConfig();
const playbackId = computed(() => config.public.muxPlaybackId.trim());
const playing = ref(false);
const fallbackFullscreen = ref(false);
const demoContainer = ref<HTMLElement | null>(null);
const playerFrame = ref<HTMLIFrameElement | null>(null);
const closeButton = ref<HTMLButtonElement | null>(null);
let enteredFullscreen = false;
let previousBodyOverflow = '';

const thumbnailUrl = computed(
  () => `https://image.mux.com/${playbackId.value}/thumbnail.webp?time=1&width=1280`,
);
// "any" first tries sound, then retries muted when browser autoplay policy
// blocks sound. Unlike autoplay=true, it needs no second click to start.
const playerUrl = computed(
  () => `https://player.mux.com/${playbackId.value}?autoplay=any&preload=auto`,
);
const muxOrigin = 'https://player.mux.com';

// Mux iframe embeds support Player.js. Only trust events from our iframe
// at Mux's exact origin; an unrelated frame may not stop the demo.
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
  if (message.event === 'ended') stopDemo();
}

function stopDemo() {
  playing.value = false;
  fallbackFullscreen.value = false;
  if (document.fullscreenElement === demoContainer.value)
    void document.exitFullscreen().catch(() => {});
}

function onFullscreenChange() {
  if (document.fullscreenElement === demoContainer.value) {
    enteredFullscreen = true;
    fallbackFullscreen.value = false;
    return;
  }
  // Exiting native fullscreen (Escape, browser or media controls) stops the
  // embedded media rather than leaving it running off-screen.
  if (!document.fullscreenElement && enteredFullscreen) {
    enteredFullscreen = false;
    stopDemo();
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && fallbackFullscreen.value) stopDemo();
}

function playDemo() {
  if (!playbackId.value || playing.value) return;
  // Fullscreen MUST be requested synchronously during this same click/key
  // gesture, before mounting the iframe. On browsers without the Fullscreen
  // API, a viewport-filling overlay provides the same one-click experience.
  const container = demoContainer.value;
  if (!container?.requestFullscreen) {
    fallbackFullscreen.value = true;
  } else {
    try {
      void container.requestFullscreen().catch(() => {
        if (playing.value) fallbackFullscreen.value = true;
      });
    } catch {
      fallbackFullscreen.value = true;
    }
  }
  playing.value = true;
}

watch(fallbackFullscreen, async (open, wasOpen) => {
  if (!import.meta.client) return;
  if (open) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    await nextTick();
    closeButton.value?.focus();
  } else if (wasOpen) {
    document.body.style.overflow = previousBodyOverflow;
  }
});

onMounted(() => {
  window.addEventListener('message', onPlayerMessage);
  document.addEventListener('fullscreenchange', onFullscreenChange);
  document.addEventListener('keydown', onKeydown);
});
onBeforeUnmount(() => {
  window.removeEventListener('message', onPlayerMessage);
  document.removeEventListener('fullscreenchange', onFullscreenChange);
  document.removeEventListener('keydown', onKeydown);
  if (fallbackFullscreen.value) document.body.style.overflow = previousBodyOverflow;
  if (document.fullscreenElement === demoContainer.value)
    void document.exitFullscreen().catch(() => {});
});
</script>

<template>
  <figure
    id="demo"
    ref="demoContainer"
    class="hero-demo-video"
    :class="{ 'hero-demo-video--fallback': fallbackFullscreen }"
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
    <button
      v-if="playing && fallbackFullscreen"
      ref="closeButton"
      type="button"
      class="hero-demo-video__close"
      :aria-label="t('shell.hero.closeDemo')"
      @click="stopDemo"
    >×</button>
    <template v-if="!playing && playbackId">
      <img :src="thumbnailUrl" :alt="t('shell.hero.demoVideoTitle')" loading="lazy" />
      <button
        type="button"
        class="hero-demo-video__launch"
        :aria-label="t('shell.hero.watchDemo')"
        @click="playDemo"
      >
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="m9 7 8 5-8 5V7Z" fill="currentColor" stroke="none" />
        </svg>
        <span>{{ t('shell.hero.watchDemo') }}</span>
      </button>
    </template>
    <div v-else-if="!playing" class="hero-demo-video__preview">
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
