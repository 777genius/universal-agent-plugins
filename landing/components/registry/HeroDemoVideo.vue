<script setup lang="ts">
const { t } = useI18n();
const config = useRuntimeConfig();
const playbackId = computed(() => config.public.muxPlaybackId.trim());
const playing = ref(false);
const preloading = ref(false);
const fallbackFullscreen = ref(false);
const demoContainer = ref<HTMLElement | null>(null);
const playerFrame = ref<HTMLIFrameElement | null>(null);
const closeButton = ref<HTMLButtonElement | null>(null);
const launchButton = ref<HTMLButtonElement | null>(null);
let enteredFullscreen = false;
let previousBodyOverflow = '';
let preloadTimer: ReturnType<typeof setTimeout> | undefined;

const thumbnailUrl = computed(
  () => `https://image.mux.com/${playbackId.value}/thumbnail.webp?time=1&width=1280`,
);
// The idle embed loads player/media resources into the browser cache without
// autoplay. After an explicit gesture, retain Mux's proven sound→muted fallback
// rather than relying on cross-origin postMessage to transfer user activation.
const playerUrl = computed(() =>
  `https://player.mux.com/${playbackId.value}?${playing.value ? 'autoplay=any&' : ''}preload=auto`,
);
const muxOrigin = 'https://player.mux.com';
function cancelPreload() {
  if (preloadTimer !== undefined) clearTimeout(preloadTimer);
  preloadTimer = undefined;
}

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
        context: 'player.js', version: '0.0.11', method: 'addEventListener',
        value: 'ended', listener: 'uap-demo-ended',
      }),
      muxOrigin,
    );
  }
  if (message.event === 'ended' && playing.value) stopDemo();
}

function stopDemo() {
  cancelPreload();
  const wasPlaying = playing.value;
  preloading.value = false;
  playing.value = false;
  fallbackFullscreen.value = false;
  if (document.fullscreenElement === demoContainer.value)
    void document.exitFullscreen().catch(() => {});
  if (wasPlaying) void nextTick(() => launchButton.value?.focus({ preventScroll: true }));
}

function onFullscreenChange() {
  if (document.fullscreenElement === demoContainer.value) {
    enteredFullscreen = true;
    fallbackFullscreen.value = false;
    return;
  }
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
  cancelPreload();
  // Fullscreen must be requested synchronously in the user's click/key gesture.
  const container = demoContainer.value;
  if (!container?.requestFullscreen) {
    fallbackFullscreen.value = true;
  } else {
    try {
      const request = container.requestFullscreen();
      if (request) void request.catch(() => {
        if (playing.value) fallbackFullscreen.value = true;
      });
    } catch {
      fallbackFullscreen.value = true;
    }
  }
  playing.value = true;
  preloading.value = false;
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
  if (playbackId.value) {
    // Count from navigation, not from an arbitrarily late hydration callback.
    preloadTimer = setTimeout(() => {
      preloadTimer = undefined;
      if (!playing.value && playbackId.value) preloading.value = true;
    }, Math.max(0, 7000 - performance.now()));
  }
});
onBeforeUnmount(() => {
  cancelPreload();
  window.removeEventListener('message', onPlayerMessage);
  document.removeEventListener('fullscreenchange', onFullscreenChange);
  document.removeEventListener('keydown', onKeydown);
  if (fallbackFullscreen.value) document.body.style.overflow = previousBodyOverflow;
  if (document.fullscreenElement === demoContainer.value)
    void document.exitFullscreen().catch(() => {});
});
</script>

<template>
  <Teleport to="body" :disabled="!fallbackFullscreen">
    <figure
      id="demo"
      ref="demoContainer"
      class="hero-demo-video"
      :class="{ 'hero-demo-video--fallback': fallbackFullscreen }"
      :aria-label="t('shell.hero.demoVideoTitle')"
    >
      <iframe
        v-if="playbackId && (playing || preloading)"
        ref="playerFrame"
        :src="playerUrl"
        :class="{ 'hero-demo-video__preload': !playing }"
        :aria-hidden="!playing ? 'true' : undefined"
        :inert="!playing"
        :tabindex="playing ? 0 : -1"
        :title="t('shell.hero.demoVideoTitle')"
        :allow="playing ? 'autoplay; fullscreen; encrypted-media; picture-in-picture' : 'fullscreen; encrypted-media; picture-in-picture'"
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
          ref="launchButton"
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
  </Teleport>
</template>

<style scoped>
/* Keep the warm embed out of layout, focus and hit testing without pausing its loading. */
.hero-demo-video__preload {
  position: absolute;
  inset: 0;
  visibility: hidden;
  pointer-events: none;
}
</style>
