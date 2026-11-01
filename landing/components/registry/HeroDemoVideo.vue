<script setup lang="ts">
const { t } = useI18n();
const config = useRuntimeConfig();
const playbackId = computed(() => config.public.muxPlaybackId.trim());
const playing = ref(false);
const fallback = ref(false);
const demoContainer = ref<HTMLElement | null>(null);
const playerHost = ref<HTMLElement | null>(null);
const thumbnailUrl = computed(
  () => `https://image.mux.com/${playbackId.value}/thumbnail.webp?time=1&width=640`,
);
const playerUrl = computed(() => `https://player.mux.com/${playbackId.value}?autoplay=true`);
// Pinned official player, loaded only if the visitor requests playback.
// The iframe fallback plays normally but cannot observe a cross-origin ended event.
const muxPlayerScript = 'https://cdn.jsdelivr.net/npm/@mux/mux-player@3.14.1';
let playerLoader: Promise<void> | undefined;

function loadPlayer() {
  if (customElements.get('mux-player')) return Promise.resolve();
  if (!playerLoader)
    playerLoader = new Promise<void>((resolve, reject) => {
      const script = document.createElement('script');
      script.src = muxPlayerScript;
      script.async = true;
      script.onload = () => void customElements.whenDefined('mux-player').then(resolve, reject);
      script.onerror = () => reject(new Error('Mux player unavailable'));
      document.head.appendChild(script);
    });
  return playerLoader;
}

const onEnded = () => {
  if (document.fullscreenElement === demoContainer.value)
    void document.exitFullscreen().catch(() => {});
  playing.value = false;
};

async function startPlayer() {
  await nextTick();
  try {
    await loadPlayer();
    if (!playing.value || !playerHost.value) return;
    const element = document.createElement('mux-player') as HTMLElement & {
      play?: () => Promise<void>;
    };
    element.setAttribute('playback-id', playbackId.value);
    element.setAttribute('autoplay', '');
    element.setAttribute('metadata-video-title', t('shell.hero.demoVideoTitle'));
    element.addEventListener('ended', onEnded, { once: true });
    playerHost.value.replaceChildren(element);
    void element.play?.().catch(() => {
      // A browser may require a second gesture; native player controls remain available.
    });
  } catch {
    // Preserve playback if the optional player script cannot load.
    fallback.value = true;
  }
}

const playDemo = () => {
  if (!playbackId.value) return;
  playing.value = true;
  fallback.value = false;
  // Fullscreen must be requested synchronously while the user's gesture is active.
  const request = demoContainer.value?.requestFullscreen?.();
  if (request) void request.catch(() => {});
  void startPlayer();
};

onBeforeUnmount(() => {
  if (typeof document !== 'undefined' && document.fullscreenElement === demoContainer.value)
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
    <div v-if="playing" class="hero-demo-video__player">
      <iframe
        v-if="fallback"
        :src="playerUrl"
        :title="t('shell.hero.demoVideoTitle')"
        allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
        allowfullscreen
      />
      <div v-else ref="playerHost" class="hero-demo-video__player-host" />
    </div>
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
