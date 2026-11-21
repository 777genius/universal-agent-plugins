<script setup lang="ts">
import { clientLandingById } from '~/data/clients';
import { isKnownLocale, localeMetadata } from '~/data/i18n';

// Ported from the product film's "Creators" and "Notifications" scenes
// (video/uap-product-film: Creators.tsx, ManagementDiagram.tsx). The scenes are
// illustrations: nothing here runs an installer or calls an engine.
const { t, locale } = useI18n();
const { asset } = useSite();
// The Vuetify shell is LTR; right-to-left locales opt in per section.
const direction = computed(() => {
  const metadata = isKnownLocale(locale.value) ? localeMetadata[locale.value] : undefined;
  return metadata && 'dir' in metadata ? metadata.dir : 'ltr';
});
const {
  sdkGuideUrl,
  sdkGuideEnglishFallback,
  sdkManagementGuideUrl,
  sdkManagementGuideEnglishFallback,
  docsLabel,
} = useDocsLinks();
const agents = (['codex', 'claude', 'gemini'] as const).map((id) => clientLandingById.get(id)!);
const features = ['detect', 'install', 'cli', 'hooks'] as const;
const installRoutes = [
  'M88 66H116C128 66 128 32 140 32H156',
  'M88 66H156',
  'M88 66H116C128 66 128 100 140 100H156',
];
const hookWires = [
  'M58 34H72C84 34 90 40 90 52V66H104',
  'M58 98H72C84 98 90 92 90 80V66H104',
  'M150 66H176',
];
const fanRoutes = ['M0 90C28 90 28 30 56 30', 'M0 90H56', 'M0 90C28 90 28 150 56 150'];
const showcase = {
  website: 'https://agent-notifications.com/',
  repository: 'https://github.com/777genius/agent-notifications',
};

// Loops run only while the section is on screen and motion is allowed. Without
// JavaScript or with reduced motion every scene shows its settled final frame.
const root = ref<HTMLElement>();
const motion = ref(false);
const inView = ref(false);
let observer: IntersectionObserver | undefined;
onMounted(() => {
  if (
    window.matchMedia('(prefers-reduced-motion: reduce)').matches ||
    !('IntersectionObserver' in window) ||
    !root.value
  )
    return;
  motion.value = true;
  observer = new IntersectionObserver(
    (entries) => {
      inView.value = entries.some((entry) => entry.isIntersecting);
    },
    { rootMargin: '80px 0px' },
  );
  observer.observe(root.value);
});
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
  <section
    id="sdk"
    ref="root"
    class="registry-sdk container anchor-offset"
    :class="{ 'registry-sdk--motion': motion, 'registry-sdk--playing': motion && inView }"
    aria-labelledby="sdk-title"
    :dir="direction"
    data-scroll-reveal
  >
    <header class="section-heading section-heading--center registry-sdk__heading">
      <p class="eyebrow">UAP SDK + CLI</p>
      <h2 id="sdk-title">{{ t('shell.sdk.title') }}</h2>
      <p class="registry-sdk__tagline">
        <img :src="asset('icon.svg')" alt="" width="30" height="30" >
        <span>{{ t('shell.sdk.tagline') }}</span>
      </p>
      <p>{{ t('shell.sdk.intro') }}</p>
    </header>

    <ul class="registry-sdk__features" :aria-label="t('shell.sdk.featuresLabel')">
      <li
        v-for="feature in features"
        :key="feature"
        class="registry-sdk__feature"
        :class="`registry-sdk__feature--${feature}`"
      >
        <div class="sdk-art" dir="ltr" aria-hidden="true">
          <svg v-if="feature === 'detect'" viewBox="0 0 240 132" fill="none">
            <defs>
              <linearGradient id="sdk-scan-beam" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0" stop-color="#81dfef" stop-opacity="0" />
                <stop offset="0.5" stop-color="#81dfef" />
                <stop offset="1" stop-color="#81dfef" stop-opacity="0" />
              </linearGradient>
            </defs>
            <path class="sdk-art__frame" d="M40 34V22h14M186 22h14v12M40 98v12h14M186 110h14V98" />
            <g
              v-for="(agent, index) in agents"
              :key="agent.id"
              class="sdk-art__scan-agent"
              :style="{ '--step': index }"
            >
              <g class="sdk-art__scan-tile">
                <rect
                  class="sdk-art__tile"
                  :x="60 + index * 44"
                  y="40"
                  width="32"
                  height="32"
                  rx="8"
                />
                <image
                  :href="asset(`client-icons/${agent.icon}`)"
                  :x="66 + index * 44"
                  y="46"
                  width="20"
                  height="20"
                />
              </g>
              <circle class="sdk-art__pop sdk-art__dot" :cx="76 + index * 44" cy="88" r="3.5" />
            </g>
            <rect
              class="sdk-art__beam"
              x="46"
              y="28"
              width="2.5"
              height="76"
              rx="1.25"
              fill="url(#sdk-scan-beam)"
            />
          </svg>

          <svg v-else-if="feature === 'install'" viewBox="0 0 240 132" fill="none">
            <image :href="asset('icon.svg')" x="38" y="44" width="44" height="44" />
            <path
              v-for="(route, index) in installRoutes"
              :key="route"
              class="sdk-art__draw sdk-art__route"
              :style="{ '--step': index }"
              pathLength="1"
              :d="route"
            />
            <g
              v-for="(agent, index) in agents"
              :key="agent.id"
              class="sdk-art__target"
              :style="{ '--step': index }"
            >
              <g class="sdk-art__grow">
                <rect
                  class="sdk-art__tile"
                  x="158"
                  :y="16 + index * 34"
                  width="32"
                  height="32"
                  rx="8"
                />
                <image
                  :href="asset(`client-icons/${agent.icon}`)"
                  x="164"
                  :y="22 + index * 34"
                  width="20"
                  height="20"
                />
              </g>
              <g class="sdk-art__pop">
                <circle class="sdk-art__badge" cx="192" :cy="44 + index * 34" r="7" />
                <path class="sdk-art__check" :d="`m188.5 ${44 + index * 34} 2.4 2.4 4.6-4.6`" />
              </g>
            </g>
          </svg>

          <svg v-else-if="feature === 'cli'" viewBox="0 0 240 132" fill="none">
            <defs>
              <linearGradient id="sdk-typing" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0.6" stop-color="#b5a9ff" />
                <stop offset="0.6" stop-color="#81dfef" />
              </linearGradient>
            </defs>
            <rect class="sdk-art__window" x="30" y="16" width="180" height="100" rx="10" />
            <path class="sdk-art__window-bar" d="M30 38H210" />
            <path class="sdk-art__window-glyph" d="m42 23 4 4-4 4m6 0h6" />
            <path class="sdk-art__prompt" d="m44 61 6 6-6 6" />
            <rect
              class="sdk-art__typing"
              x="58"
              y="64.5"
              width="122"
              height="5"
              rx="2.5"
              fill="url(#sdk-typing)"
            />
            <rect class="sdk-art__cursor" x="58" y="59" width="3" height="16" rx="1" />
            <g class="sdk-art__done">
              <path class="sdk-art__check" d="m44 93 3 3 6-6" />
              <rect class="sdk-art__done-line" x="58" y="91" width="74" height="4" rx="2" />
            </g>
          </svg>

          <svg v-else viewBox="0 0 240 132" fill="none">
            <path
              class="sdk-art__wire"
              d="M58 34H72C84 34 90 40 90 52V66H104M58 98H72C84 98 90 92 90 80V66M150 66H176"
            />
            <path
              v-for="(wire, index) in hookWires"
              :key="wire"
              class="sdk-art__draw sdk-art__wire-live"
              :class="{ 'sdk-art__wire-live--out': index === 2 }"
              pathLength="1"
              :d="wire"
            />
            <rect
              class="sdk-art__node sdk-art__node--violet"
              x="40"
              y="25"
              width="18"
              height="18"
              rx="5"
            />
            <circle class="sdk-art__node sdk-art__node--cyan" cx="49" cy="98" r="9" />
            <rect class="sdk-art__code-box" x="104" y="42" width="46" height="48" rx="12" />
            <path class="sdk-art__code" d="m123 59-7 7 7 7m8-14 7 7-7 7m-3-17-3 20" />
            <g class="sdk-art__pop sdk-art__hook-done">
              <rect class="sdk-art__badge" x="176" y="57" width="18" height="18" rx="5" />
              <path class="sdk-art__check" d="m180.5 66 3 3 6-6" />
            </g>
          </svg>
        </div>
        <h3>{{ t(`shell.sdk.features.${feature}.title`) }}</h3>
        <p>{{ t(`shell.sdk.features.${feature}.description`) }}</p>
      </li>
    </ul>

    <article class="registry-sdk__apps" aria-labelledby="sdk-apps-title">
      <div class="registry-sdk__apps-copy">
        <div class="registry-sdk__apps-meta">
          <p class="eyebrow">{{ t('shell.sdk.apps.eyebrow') }}</p>
          <span class="registry-sdk__badge">{{ t('shell.sdk.apps.badge') }}</span>
        </div>
        <h3 id="sdk-apps-title">{{ t('shell.sdk.apps.title') }}</h3>
        <p>{{ t('shell.sdk.apps.description') }}</p>
        <p class="registry-sdk__package">
          <span aria-hidden="true">npm</span>
          <code dir="ltr">universal-agent-plugins-management-sdk</code>
        </p>
        <a
          class="registry-sdk__link"
          :href="sdkManagementGuideUrl"
          target="_blank"
          rel="noreferrer"
        >
          {{ docsLabel(t('shell.sdk.apps.guide'), sdkManagementGuideEnglishFallback) }}
          <span aria-hidden="true">↗</span>
        </a>
      </div>

      <figure class="sdk-flow">
        <div
          class="sdk-flow__graph"
          dir="ltr"
          role="img"
          :aria-label="t('shell.sdk.apps.diagramLabel')"
        >
          <div class="sdk-flow__app">
            <div class="sdk-flow__app-header">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="m8 5-7 7 7 7m8-14 7 7-7 7m-3-17-3 20" />
              </svg>
              <strong dir="auto">{{ t('shell.sdk.apps.yourApp') }}</strong>
              <span dir="auto">{{ t('shell.sdk.apps.tag') }}</span>
            </div>
            <div class="sdk-flow__package">
              <img :src="asset('icon.svg')" alt="" width="40" height="40" >
              <span>
                <strong>marketing-kit</strong>
                <small dir="auto">{{ t('shell.sdk.apps.kind') }}</small>
              </span>
            </div>
            <span class="sdk-flow__status sdk-flow__status--app">
              <span class="sdk-flow__status-installing" dir="auto">{{
                t('shell.sdk.apps.installing')
              }}</span>
              <span class="sdk-flow__status-installed" dir="auto">{{
                t('shell.sdk.apps.installed')
              }}</span>
            </span>
            <div class="sdk-flow__actions">
              <span class="sdk-flow__action sdk-flow__action--install" dir="auto">{{
                t('shell.sdk.apps.install')
              }}</span>
              <span class="sdk-flow__action sdk-flow__action--update" dir="auto">{{
                t('shell.sdk.apps.update')
              }}</span>
              <span class="sdk-flow__action sdk-flow__action--remove" dir="auto">{{
                t('shell.sdk.apps.remove')
              }}</span>
            </div>
          </div>

          <span class="sdk-flow__link" />

          <div class="sdk-flow__engine">
            <span class="sdk-flow__engine-mark">
              <svg class="sdk-flow__ring" viewBox="0 0 80 80" fill="none" aria-hidden="true">
                <circle cx="40" cy="40" r="37" stroke-dasharray="32 11 6 11" />
                <circle cx="40" cy="40" r="32" stroke-opacity="0.3" />
              </svg>
              <img :src="asset('icon.svg')" alt="" width="44" height="44" >
            </span>
            <strong>UAP</strong>
            <span>Management SDK</span>
          </div>

          <div class="sdk-flow__fanout">
            <svg
              class="sdk-flow__fan"
              viewBox="0 0 56 180"
              preserveAspectRatio="none"
              fill="none"
              aria-hidden="true"
            >
              <path v-for="route in fanRoutes" :key="route" pathLength="1" :d="route" />
              <path
                v-for="route in fanRoutes"
                :key="`${route}-packet`"
                class="sdk-flow__packet"
                pathLength="1"
                :d="route"
              />
            </svg>
            <div class="sdk-flow__targets">
              <div v-for="agent in agents" :key="agent.id" class="sdk-flow__target">
                <span class="sdk-flow__target-card">
                  <span class="sdk-flow__target-icon">
                    <img :src="asset(`client-icons/${agent.icon}`)" alt="" width="20" height="20" >
                  </span>
                  <span class="sdk-flow__target-copy">
                    <strong>{{ agent.name }}</strong>
                    <span class="sdk-flow__status">
                      <span class="sdk-flow__status-installing" dir="auto">{{
                        t('shell.sdk.apps.installing')
                      }}</span>
                      <span class="sdk-flow__status-installed" dir="auto">{{
                        t('shell.sdk.apps.installed')
                      }}</span>
                    </span>
                  </span>
                  <span class="sdk-flow__target-check">✓</span>
                </span>
              </div>
            </div>
          </div>
        </div>
        <figcaption>{{ t('shell.sdk.apps.caption') }}</figcaption>
      </figure>
    </article>

    <article class="registry-sdk__showcase" aria-labelledby="sdk-showcase-title">
      <span class="registry-sdk__mascot">
        <img
          :src="asset('showcase/agent-notifications.webp')"
          alt=""
          width="112"
          height="112"
          loading="lazy"
          decoding="async"
        >
      </span>
      <div class="registry-sdk__showcase-copy">
        <p class="registry-sdk__showcase-eyebrow">
          <img :src="asset('icon.svg')" alt="" width="20" height="20" >
          {{ t('shell.sdk.showcase.eyebrow') }}
        </p>
        <h3 id="sdk-showcase-title">Agent Notifications</h3>
        <p>{{ t('shell.sdk.showcase.description') }}</p>
        <p class="registry-sdk__built-on">{{ t('shell.sdk.showcase.builtOn') }}</p>
      </div>
      <div class="registry-sdk__showcase-links">
        <a :href="showcase.website" target="_blank" rel="noopener noreferrer">
          agent-notifications.com <span aria-hidden="true">↗</span>
        </a>
        <a :href="showcase.repository" target="_blank" rel="noopener noreferrer">
          {{ t('shell.sdk.showcase.repository') }} <span aria-hidden="true">↗</span>
        </a>
      </div>
    </article>

    <p class="registry-sdk__footer">
      <a class="button button--primary" :href="sdkGuideUrl" target="_blank" rel="noreferrer">
        {{ docsLabel(t('shell.sdk.guides'), sdkGuideEnglishFallback) }}
        <span aria-hidden="true">↗</span>
      </a>
    </p>
  </section>
</template>

<style scoped lang="scss">
// Film palette: these scenes stay dark in both themes, like the product video.
$film-panel: #0d1322;
$film-card: #101624;
$film-line: #c6d6ff2c;
$film-text: #e1e8f6;
$film-muted: #9aa8bf;
$film-cyan: #81dfef;
$film-violet: #b5a9ff;
$film-green: #b8e8ad;
$film-amber: #edc68d;
$loop: 5.6s;
$flow-loop: 6.4s;
$ease: cubic-bezier(0.2, 0.75, 0.2, 1);

.registry-sdk {
  padding-block: 64px 88px;

  h2 {
    font-size: clamp(2rem, 4vw, 3.1rem);
    text-wrap: balance;
  }
}

.registry-sdk__heading {
  max-width: 720px;
  margin-bottom: 36px;
}

.registry-sdk__tagline {
  margin: 0 0 14px;
  display: inline-flex;
  align-items: center;
  gap: 12px;
  color: var(--text);
  font-size: clamp(1.02rem, 1.6vw, 1.2rem);
  font-weight: 600;

  img {
    width: 30px;
    height: 30px;
    flex: none;
  }
}

.registry-sdk__features {
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  list-style: none;
}

.registry-sdk__feature {
  min-width: 0;
  padding: 14px 14px 22px;
  border: 1px solid var(--line);
  border-radius: 20px;
  background: linear-gradient(
    155deg,
    color-mix(in srgb, var(--surface-solid) 85%, transparent),
    var(--surface)
  );
  box-shadow: 0 16px 38px color-mix(in srgb, var(--text) 5%, transparent);

  h3 {
    margin: 18px 8px 8px;
    color: var(--text);
    font-size: 1.04rem;
    line-height: 1.3;
    letter-spacing: -0.025em;
  }

  p {
    margin: 0 8px;
    color: var(--muted);
    font-size: 0.86rem;
    line-height: 1.6;
  }
}

.sdk-art {
  border: 1px solid $film-line;
  border-radius: 14px;
  background:
    radial-gradient(ellipse at 20% 0%, rgba(118, 60, 213, 0.22), transparent 60%),
    radial-gradient(ellipse at 90% 100%, rgba(0, 196, 219, 0.14), transparent 60%), $film-panel;
  overflow: hidden;

  svg {
    display: block;
    width: 100%;
    height: auto;
  }
}

.sdk-art__frame {
  stroke: #91c9db66;
  stroke-width: 2;
  stroke-linecap: round;
}
.sdk-art__tile {
  fill: #fff;
  stroke: #c6d6ff55;
}
.sdk-art__dot {
  fill: $film-green;
}
.sdk-art__route {
  stroke: $film-violet;
  stroke-width: 2;
}
.sdk-art__draw {
  stroke-dasharray: 1;
  stroke-dashoffset: 0;
}
.sdk-art__badge {
  fill: #13241d;
  stroke: #b8e8ad99;
}
.sdk-art__check {
  stroke: $film-green;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.sdk-art__window {
  fill: #090c12;
  stroke: #acbcda40;
}
.sdk-art__window-bar {
  stroke: #aab9d21f;
}
.sdk-art__window-glyph {
  stroke: #7f8da7;
  stroke-width: 1.4;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.sdk-art__prompt {
  stroke: $film-cyan;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.sdk-art__cursor {
  fill: $film-violet;
  opacity: 0;
}
.sdk-art__done-line {
  fill: #b8e8ad66;
}
.sdk-art__wire {
  stroke: #7c9ebd66;
  stroke-width: 2;
}
.sdk-art__wire-live {
  stroke: $film-cyan;
  stroke-width: 2;
}
.sdk-art__node {
  stroke-width: 1.5;
}
.sdk-art__node--violet {
  fill: #242039;
  stroke: $film-violet;
}
.sdk-art__node--cyan {
  fill: #122b33;
  stroke: $film-cyan;
}
.sdk-art__code-box {
  fill: #151d2c;
  stroke: #aab5ff99;
}
.sdk-art__code {
  stroke: $film-violet;
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

// Settled frame is the default; the loop only starts in motion mode.
.sdk-art__beam {
  opacity: 0;
}
.sdk-art__pop,
.sdk-art__grow,
.sdk-art__typing,
.sdk-art__cursor,
.sdk-art__beam {
  transform-box: fill-box;
}
.sdk-art__pop,
.sdk-art__grow {
  transform-origin: center;
}
.sdk-art__typing {
  transform-origin: left center;
}

.registry-sdk__apps {
  margin-top: 28px;
  padding: 28px;
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 2fr);
  align-items: center;
  gap: 32px;
  border: 1px solid var(--line-strong);
  border-radius: 24px;
  background: var(--surface);
  box-shadow: 0 16px 38px color-mix(in srgb, var(--text) 5%, transparent);

  h3 {
    margin-bottom: 12px;
    font-size: clamp(1.4rem, 2.4vw, 1.9rem);
    text-wrap: balance;
  }
}

.registry-sdk__apps-copy > p {
  color: var(--muted);
  font-size: 0.9rem;
  line-height: 1.65;
}

.registry-sdk__apps-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  margin-bottom: 12px;

  .eyebrow {
    margin: 0;
  }
}

.registry-sdk__badge {
  padding: 3px 10px;
  border: 1px solid color-mix(in srgb, #d99a3a 55%, transparent);
  border-radius: 999px;
  color: color-mix(in srgb, #d99a3a 75%, var(--text));
  background: color-mix(in srgb, #d99a3a 10%, transparent);
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
}

.registry-sdk__package {
  margin: 12px 0 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;

  span {
    padding: 2px 6px;
    border-radius: 6px;
    background: color-mix(in srgb, var(--coral) 16%, transparent);
    color: var(--coral);
    font-size: 0.68rem;
    font-weight: 800;
  }

  code {
    min-width: 0;
    color: var(--text);
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }
}

.registry-sdk__link,
.registry-sdk__showcase-links a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--cyan);
  font-size: 0.875rem;
  font-weight: 700;
  text-decoration: none;

  &:hover,
  &:focus-visible {
    text-decoration: underline;
    text-underline-offset: 3px;
  }
}

.sdk-flow {
  margin: 0;
  min-width: 0;

  figcaption {
    margin-top: 16px;
    color: var(--text);
    font-size: 1rem;
    font-weight: 600;
    text-align: center;
  }
}

.sdk-flow__graph {
  padding: 22px;
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) 36px minmax(0, 0.8fr) minmax(0, 1.25fr);
  align-items: center;
  border: 1px solid $film-line;
  border-radius: 18px;
  background:
    radial-gradient(ellipse at 24% 20%, rgba(118, 60, 213, 0.2), transparent 55%),
    radial-gradient(ellipse at 85% 80%, rgba(0, 196, 219, 0.12), transparent 55%), $film-panel;
  color: $film-text;
}

.sdk-flow__app,
.sdk-flow__engine,
.sdk-flow__target-card {
  border: 1px solid $film-line;
  background: $film-card;
}

.sdk-flow__app {
  min-width: 0;
  border-radius: 14px;
  overflow: hidden;
}

.sdk-flow__app-header {
  padding: 10px 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid #b6c4e622;

  svg {
    width: 16px;
    height: 16px;
    flex: none;
    stroke: $film-cyan;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  strong {
    font-size: 0.82rem;
  }

  span {
    margin-inline-start: auto;
    color: $film-violet;
    font-size: 0.66rem;
    text-align: end;
  }
}

.sdk-flow__package {
  padding: 12px 12px 4px;
  display: flex;
  align-items: center;
  gap: 10px;

  img {
    width: 34px;
    height: 34px;
    flex: none;
  }

  > span {
    display: grid;
    min-width: 0;
  }

  strong {
    font-size: 0.86rem;
    font-weight: 600;
  }

  small {
    color: $film-muted;
    font-size: 0.68rem;
  }
}

// Two labels share one grid cell, so the swap never changes the layout.
.sdk-flow__status {
  display: inline-grid;
  font-size: 0.66rem;

  > span {
    grid-area: 1 / 1;
  }
}
.sdk-flow__status-installing {
  color: $film-cyan;
  opacity: 0;
}
.sdk-flow__status-installed {
  color: $film-green;
}

.sdk-flow__status--app {
  padding: 4px 12px 10px;
  display: grid;

  > span::before {
    content: '';
    display: inline-block;
    width: 6px;
    height: 6px;
    margin-inline-end: 7px;
    border-radius: 50%;
    background: currentColor;
    vertical-align: 1px;
  }
}

.sdk-flow__actions {
  padding: 0 12px 12px;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 6px;
}

.sdk-flow__action {
  --action: #{$film-cyan};
  min-width: 0;
  padding: 6px 4px;
  border: 1px solid color-mix(in srgb, var(--action) 35%, transparent);
  border-radius: 8px;
  color: var(--action);
  background: color-mix(in srgb, var(--action) 6%, transparent);
  font-size: 0.66rem;
  font-weight: 650;
  text-align: center;
  overflow-wrap: anywhere;
}
.sdk-flow__action--install {
  border-color: color-mix(in srgb, var(--action) 75%, transparent);
  color: #0b101a;
  background: var(--action);
}
.sdk-flow__action--update {
  --action: #{$film-violet};
}
.sdk-flow__action--remove {
  --action: #{$film-amber};
}

.sdk-flow__link {
  height: 2px;
  background:
    linear-gradient(90deg, transparent 40%, $film-cyan 50%, transparent 60%) 120% 0 / 300% 100%
      no-repeat,
    #a99bfa66;
}

.sdk-flow__engine {
  min-width: 0;
  padding: 16px 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  border-radius: 18px;
  text-align: center;

  strong {
    margin-top: 8px;
    font-size: 0.95rem;
  }

  > span:last-child {
    color: #d0d7ee;
    font-size: 0.74rem;
  }
}

.sdk-flow__engine-mark {
  position: relative;
  width: 62px;
  height: 62px;
  display: grid;
  place-items: center;

  img {
    width: 40px;
    height: 40px;
  }
}

.sdk-flow__ring {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  stroke: $film-violet;
  stroke-width: 2;
}

.sdk-flow__fanout {
  min-width: 0;
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  align-items: stretch;
}

.sdk-flow__fan {
  width: 100%;
  height: 100%;

  path {
    stroke: #a99bfa66;
    stroke-width: 2;
    vector-effect: non-scaling-stroke;
  }

  .sdk-flow__packet {
    stroke: $film-cyan;
    stroke-dasharray: 0.16 1.2;
    stroke-dashoffset: 0.16;
    opacity: 0;
  }
}

.sdk-flow__targets {
  min-width: 0;
  display: grid;
  grid-auto-rows: 1fr;
}

// Equal rows keep each fan curve aimed at a card center.
.sdk-flow__target {
  min-width: 0;
  padding-block: 4px;
}

.sdk-flow__target-card {
  height: 100%;
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 9px;
  border-radius: 12px;
}

.sdk-flow__target-icon {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: #fff;

  img {
    width: 18px;
    height: 18px;
    object-fit: contain;
  }
}

.sdk-flow__target-copy {
  min-width: 0;
  display: grid;
  line-height: 1.25;

  strong {
    font-size: 0.78rem;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
}

.sdk-flow__target-check {
  margin-inline-start: auto;
  width: 18px;
  height: 18px;
  flex: none;
  display: grid;
  place-items: center;
  border: 1px solid #b8e8ad88;
  border-radius: 50%;
  color: $film-green;
  background: #13241d;
  font-size: 0.6rem;
}

.registry-sdk__showcase {
  margin-top: 28px;
  padding: 28px;
  display: grid;
  grid-template-columns: 112px minmax(0, 1fr) auto;
  align-items: center;
  gap: 28px;
  border: 1px solid var(--line-strong);
  border-radius: 24px;
  background: var(--surface);

  h3 {
    margin-bottom: 8px;
    font-size: 1.5rem;
  }
}

.registry-sdk__showcase-copy {
  min-width: 0;

  p {
    margin-bottom: 0;
    color: var(--muted);
  }

  .registry-sdk__showcase-eyebrow {
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--primary-bright);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;

    img {
      width: 20px;
      height: 20px;
    }
  }

  .registry-sdk__built-on {
    margin-top: 8px;
    color: var(--text);
    font-size: 0.9rem;
    font-weight: 600;
  }
}

.registry-sdk__mascot {
  width: 112px;
  height: 112px;
  display: grid;
  place-items: center;
  border: 1px solid $film-line;
  border-radius: 28px;
  background: radial-gradient(circle at 50% 40%, #1b2440, $film-panel 72%);

  img {
    width: 92px;
    height: 92px;
    object-fit: contain;
  }
}

.registry-sdk__showcase-links {
  display: grid;
  justify-items: start;
  gap: 10px;
}

.registry-sdk__footer {
  margin: 32px 0 0;
  text-align: center;
}

// Loops exist only while the section is on screen, so nothing animates off
// screen. Each loop starts from its first frame and returns to the settled one.
.registry-sdk--playing {
  .sdk-art__beam {
    animation: sdk-scan-beam $loop linear infinite both;
  }
  .sdk-art__scan-tile {
    animation: sdk-scan-tile $loop $ease infinite both;
    animation-delay: calc(0.35s + var(--step) * 0.55s);
  }
  .sdk-art__scan-agent .sdk-art__pop {
    animation: sdk-pop $loop $ease infinite both;
    animation-delay: calc(0.45s + var(--step) * 0.55s);
  }
  .sdk-art__route {
    animation: sdk-draw $loop $ease infinite both;
    animation-delay: calc(var(--step) * 0.18s);
  }
  .sdk-art__target .sdk-art__grow {
    animation: sdk-grow $loop $ease infinite both;
    animation-delay: calc(0.5s + var(--step) * 0.25s);
  }
  .sdk-art__target .sdk-art__pop {
    animation: sdk-pop $loop $ease infinite both;
    animation-delay: calc(1.1s + var(--step) * 0.25s);
  }
  .sdk-art__typing {
    animation: sdk-type $loop $ease infinite both;
  }
  .sdk-art__cursor {
    animation:
      sdk-cursor $loop $ease infinite both,
      sdk-blink 0.9s steps(1) infinite;
  }
  .sdk-art__done {
    animation: sdk-done $loop $ease infinite both;
  }
  .sdk-art__wire-live {
    animation: sdk-draw $loop $ease infinite both;
  }
  .sdk-art__wire-live--out {
    animation-delay: 0.9s;
  }
  .sdk-art__code-box {
    animation: sdk-code-box $loop $ease infinite both;
  }
  .sdk-art__hook-done {
    animation: sdk-pop $loop $ease infinite both;
    animation-delay: 1.5s;
  }

  .sdk-flow__action--install {
    animation: sdk-press $flow-loop $ease infinite both;
  }
  .sdk-flow__link {
    animation: sdk-link $flow-loop linear infinite both;
  }
  .sdk-flow__ring {
    animation:
      sdk-spin 2.4s linear infinite,
      sdk-ring $flow-loop $ease infinite both;
  }
  .sdk-flow__packet {
    animation: sdk-packet $flow-loop linear infinite both;
  }
  .sdk-flow__status-installing {
    animation: sdk-status-installing $flow-loop steps(1) infinite both;
  }
  .sdk-flow__status-installed {
    animation: sdk-status-installed $flow-loop steps(1) infinite both;
  }
  .sdk-flow__targets .sdk-flow__status-installing {
    animation-name: sdk-target-installing;
  }
  .sdk-flow__target-check {
    animation: sdk-target-check $flow-loop $ease infinite both;
  }
}

@keyframes sdk-scan-beam {
  0% {
    opacity: 0;
    transform: translateX(0);
  }
  5% {
    opacity: 1;
  }
  38% {
    opacity: 1;
    transform: translateX(146px);
  }
  46%,
  100% {
    opacity: 0;
    transform: translateX(146px);
  }
}
@keyframes sdk-scan-tile {
  0% {
    opacity: 0.25;
  }
  6%,
  84% {
    opacity: 1;
  }
  94%,
  100% {
    opacity: 0.25;
  }
}
@keyframes sdk-pop {
  0% {
    opacity: 0;
    transform: scale(0.3);
  }
  6% {
    opacity: 1;
    transform: scale(1.2);
  }
  10%,
  80% {
    opacity: 1;
    transform: scale(1);
  }
  90%,
  100% {
    opacity: 0;
    transform: scale(0.3);
  }
}
@keyframes sdk-grow {
  0% {
    opacity: 0;
    transform: scale(0.82);
  }
  8%,
  84% {
    opacity: 1;
    transform: scale(1);
  }
  94%,
  100% {
    opacity: 0;
    transform: scale(0.82);
  }
}
@keyframes sdk-draw {
  0% {
    opacity: 1;
    stroke-dashoffset: 1;
  }
  24%,
  86% {
    opacity: 1;
    stroke-dashoffset: 0;
  }
  96%,
  100% {
    opacity: 0;
    stroke-dashoffset: 0;
  }
}
@keyframes sdk-type {
  0%,
  6% {
    opacity: 1;
    transform: scaleX(0);
  }
  46%,
  86% {
    opacity: 1;
    transform: scaleX(1);
  }
  96%,
  100% {
    opacity: 0;
    transform: scaleX(1);
  }
}
@keyframes sdk-cursor {
  0%,
  6% {
    opacity: 1;
    transform: translateX(0);
  }
  46%,
  86% {
    opacity: 1;
    transform: translateX(124px);
  }
  96%,
  100% {
    opacity: 0;
    transform: translateX(124px);
  }
}
@keyframes sdk-blink {
  50% {
    fill-opacity: 0;
  }
}
@keyframes sdk-done {
  0%,
  50% {
    opacity: 0;
  }
  56%,
  86% {
    opacity: 1;
  }
  96%,
  100% {
    opacity: 0;
  }
}
@keyframes sdk-code-box {
  0%,
  18% {
    stroke: #aab5ff40;
  }
  28%,
  86% {
    stroke: #{$film-violet};
  }
  100% {
    stroke: #aab5ff40;
  }
}
@keyframes sdk-press {
  0% {
    transform: scale(1);
  }
  3% {
    transform: scale(0.94);
  }
  7%,
  100% {
    transform: scale(1);
  }
}
@keyframes sdk-link {
  0% {
    background-position:
      120% 0,
      0 0;
  }
  16%,
  100% {
    background-position:
      -20% 0,
      0 0;
  }
}
@keyframes sdk-spin {
  to {
    transform: rotate(1turn);
  }
}
@keyframes sdk-ring {
  0%,
  10% {
    opacity: 0.35;
  }
  16%,
  52% {
    opacity: 1;
  }
  62%,
  100% {
    opacity: 0.35;
  }
}
@keyframes sdk-packet {
  0%,
  14% {
    opacity: 0;
    stroke-dashoffset: 0.16;
  }
  15% {
    opacity: 1;
    stroke-dashoffset: 0.16;
  }
  34% {
    opacity: 1;
    stroke-dashoffset: -1;
  }
  35%,
  100% {
    opacity: 0;
    stroke-dashoffset: -1;
  }
}
@keyframes sdk-status-installing {
  0% {
    opacity: 1;
  }
  55%,
  100% {
    opacity: 0;
  }
}
@keyframes sdk-status-installed {
  0% {
    opacity: 0;
  }
  55% {
    opacity: 1;
  }
}
@keyframes sdk-target-installing {
  0% {
    opacity: 0;
  }
  18% {
    opacity: 1;
  }
  55%,
  100% {
    opacity: 0;
  }
}
@keyframes sdk-target-check {
  0%,
  54% {
    opacity: 0;
    transform: scale(0.4);
  }
  58% {
    opacity: 1;
    transform: scale(1.2);
  }
  62%,
  94% {
    opacity: 1;
    transform: scale(1);
  }
  100% {
    opacity: 0;
    transform: scale(0.4);
  }
}

@media (max-width: 1100px) {
  .registry-sdk__apps {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 980px) {
  .registry-sdk__features {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .sdk-flow__graph {
    grid-template-columns: minmax(0, 1fr);
    justify-items: center;
    padding: 16px;
  }
  .sdk-flow__app,
  .sdk-flow__fanout {
    width: min(100%, 360px);
  }
  .sdk-flow__link {
    width: 2px;
    height: 26px;
    background:
      linear-gradient(180deg, transparent 40%, $film-cyan 50%, transparent 60%) 0 120% / 100% 300%
        no-repeat,
      #a99bfa66;
  }
  .sdk-flow__engine {
    width: min(100%, 220px);
  }
  // Stacked: one trunk line from the engine to the agent list.
  .sdk-flow__fanout {
    grid-template-columns: minmax(0, 1fr);
    padding-top: 26px;
    position: relative;

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: 50%;
      width: 2px;
      height: 26px;
      background: #a99bfa66;
    }
  }
  .sdk-flow__fan {
    display: none;
  }
  .registry-sdk--playing .sdk-flow__link {
    animation-name: sdk-link-vertical;
  }
}

@keyframes sdk-link-vertical {
  0% {
    background-position:
      0 120%,
      0 0;
  }
  16%,
  100% {
    background-position:
      0 -20%,
      0 0;
  }
}

@media (max-width: 720px) {
  .registry-sdk {
    padding-block: 48px 64px;
  }
  .registry-sdk__features {
    grid-template-columns: minmax(0, 1fr);
  }
  .registry-sdk__apps,
  .registry-sdk__showcase {
    padding: 20px;
  }
  .registry-sdk__showcase {
    grid-template-columns: 72px minmax(0, 1fr);
    align-items: start;
    gap: 18px;
  }
  .registry-sdk__mascot {
    width: 72px;
    height: 72px;
    border-radius: 20px;

    img {
      width: 60px;
      height: 60px;
    }
  }
  .registry-sdk__showcase-links {
    grid-column: 1 / -1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .registry-sdk .sdk-art *,
  .registry-sdk .sdk-flow__graph * {
    animation: none !important;
  }
}
</style>
