<script setup lang="ts">
const { t } = useI18n();
const { asset } = useSite();
// The film's final dependency scene: both demonstration packages share Playwright.
const packages = [
  { id: 'marketing', name: 'marketing-kit', glyph: 'plugin' },
  { id: 'design', name: 'Web Design Kit', glyph: 'code' },
];
const dependencies = [
  { id: 'playwright', name: 'Playwright', shared: true, delay: 520 },
  { id: 'hubspot', name: 'HubSpot', delay: 320 },
  { id: 'notion', name: 'Notion', delay: 420 },
  { id: 'figma', name: 'Figma', delay: 620 },
  { id: 'context7', name: 'Context7', existing: true, delay: 720 },
];
const scene = ref<HTMLElement>();
const animationReady = ref(false);
const revealed = ref(false);
let observer: IntersectionObserver | undefined;
onMounted(() => {
  if (
    window.matchMedia('(prefers-reduced-motion: reduce)').matches ||
    !('IntersectionObserver' in window) ||
    !scene.value
  )
    return;
  animationReady.value = true;
  observer = new IntersectionObserver(
    (entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return;
      revealed.value = true;
      observer?.disconnect();
    },
    { threshold: 0.2 },
  );
  observer.observe(scene.value);
});
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
  <section
    id="plugin-dependencies"
    class="registry-dependencies container"
    aria-labelledby="dependencies-title"
    dir="auto"
  >
    <header class="section-heading section-heading--center">
      <p class="eyebrow">{{ t('shell.dependencies.eyebrow') }}</p>
      <h2 id="dependencies-title">{{ t('shell.dependencies.title') }}</h2>
      <p>{{ t('shell.dependencies.description') }}</p>
    </header>

    <figure
      ref="scene"
      class="dependency-scene"
      :class="{
        'dependency-scene--animate': animationReady,
        'dependency-scene--revealed': revealed,
      }"
    >
      <div
        class="dependency-scene__graph"
        dir="ltr"
        role="img"
        :aria-label="t('shell.dependencies.graphDescription')"
      >
        <svg
          class="dependency-scene__edges dependency-scene__edges--desktop"
          viewBox="0 0 1000 440"
          preserveAspectRatio="none"
          fill="none"
          aria-hidden="true"
        >
          <g class="dependency-scene__marketing-edges">
            <path pathLength="1" d="M290 126C365 126 350 220 430 220" />
            <path pathLength="1" d="M290 106C460 106 570 52 760 52" />
            <path pathLength="1" d="M290 146C460 75 590 164 760 164" />
          </g>
          <g class="dependency-scene__design-edges">
            <path pathLength="1" d="M290 320C365 320 350 220 430 220" />
            <path pathLength="1" d="M290 340C465 340 565 276 760 276" />
            <path pathLength="1" d="M290 360C465 360 565 388 760 388" />
          </g>
        </svg>
        <svg
          class="dependency-scene__edges dependency-scene__edges--mobile"
          viewBox="0 0 360 680"
          preserveAspectRatio="none"
          fill="none"
          aria-hidden="true"
        >
          <g class="dependency-scene__marketing-edges">
            <path pathLength="1" d="M162 65C180 65 180 44 202 44" />
            <path pathLength="1" d="M162 90C183 90 180 154 202 154" />
            <path pathLength="1" d="M162 116C180 116 180 330 202 330" />
          </g>
          <g class="dependency-scene__design-edges">
            <path pathLength="1" d="M162 444C180 444 180 330 202 330" />
            <path pathLength="1" d="M162 470C182 470 180 506 202 506" />
            <path pathLength="1" d="M162 495C182 495 180 616 202 616" />
          </g>
        </svg>

        <div
          v-for="plugin in packages"
          :key="plugin.id"
          class="dependency-scene__package"
          :class="`dependency-scene__package--${plugin.id}`"
          aria-hidden="true"
        >
          <div class="dependency-scene__package-name">
            <svg viewBox="0 0 32 32" fill="none" aria-hidden="true">
              <path
                v-if="plugin.glyph === 'plugin'"
                d="m16 3 11 6v14l-11 6-11-6V9Z M5 9l11 6 11-6 M16 15v14 M10.5 6l11 6v6"
              />
              <path v-else d="m11 8-8 8 8 8 M21 8l8 8-8 8 M19 4l-6 24" />
            </svg>
            <strong>{{ plugin.name }}</strong>
          </div>
          <p class="dependency-scene__status">
            <span class="dependency-scene__check">✓</span>{{ t('shell.dependencies.installed') }}
          </p>
          <div class="dependency-scene__progress" />
        </div>

        <div
          v-for="dependency in dependencies"
          :key="dependency.id"
          class="dependency-scene__dependency"
          :class="`dependency-scene__dependency--${dependency.id}`"
          :style="{ '--arrival-delay': `${dependency.delay}ms` }"
          aria-hidden="true"
        >
          <img
            :src="asset(`dependency-icons/${dependency.id}.svg`)"
            alt=""
            width="36"
            height="36"
            loading="lazy"
          />
          <div class="dependency-scene__dependency-copy">
            <strong>{{ dependency.name }}</strong>
            <span class="dependency-scene__status" dir="auto">{{
              t(
                dependency.existing
                  ? 'shell.dependencies.alreadyInstalled'
                  : 'shell.dependencies.installed',
              )
            }}</span>
            <span v-if="dependency.shared" class="dependency-scene__shared">{{
              t('shell.dependencies.shared')
            }}</span>
          </div>
          <span class="dependency-scene__check">✓</span>
        </div>
      </div>
      <figcaption>{{ t('shell.dependencies.caption') }}</figcaption>
    </figure>
  </section>
</template>

<style scoped lang="scss">
.registry-dependencies {
  width: min(calc(100% - 32px), 944px);
  padding-block: 27.2px 51.2px;

  .section-heading {
    max-width: 544px;
    margin-block-end: 24px;
  }
  .section-heading > p:last-child {
    font-size: 0.8rem;
  }
  .eyebrow {
    font-size: 0.6rem;
  }
  h2 {
    font-size: clamp(1.6rem, 3.2vw, 2.68rem);
    text-wrap: balance;
  }
}

.dependency-scene {
  margin: 0;
  padding: clamp(14.4px, 2.8vw, 32px);
  border: 0.8px solid var(--line-strong);
  border-radius: 16px;
  background:
    radial-gradient(ellipse at 8% 20%, #a98bfa0d, transparent 50%),
    radial-gradient(ellipse at 55% 60%, #81dfef09, transparent 45%), #0b101c;
  color: #e1e8f6;
  box-shadow: 0 16px 48px #00000012;

  &__graph {
    position: relative;
    height: 352px;
  }
  &__edges {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    stroke-width: 1.5;
  }
  &__edges--mobile {
    display: none;
  }
  &__marketing-edges {
    stroke: #ad92ef;
    stroke-opacity: 0.6;
  }
  &__design-edges {
    stroke: #81dfef;
    stroke-opacity: 0.6;
  }
  &__package,
  &__dependency {
    position: absolute;
    border: 0.8px solid #c6d6ff2c;
    background: #101624;
  }
  &__package {
    left: 0;
    width: 29%;
    padding: 17.6px;
    border-radius: 12.8px;
  }
  &__package--marketing {
    top: 33.6px;
    --package-color: #ad92ef;
  }
  &__package--design {
    top: 211.2px;
    --package-color: #81dfef;
  }
  &__package-name {
    display: flex;
    align-items: center;
    gap: 9.6px;
  }
  &__package-name svg {
    width: 23.2px;
    height: 23.2px;
    flex-shrink: 0;
    stroke: var(--package-color);
    stroke-width: 1.5;
    stroke-linejoin: round;
    stroke-linecap: round;
  }
  strong {
    font-size: clamp(0.72rem, 1.28vw, 0.96rem);
    font-weight: 600;
    letter-spacing: -0.025em;
  }
  &__status {
    display: flex;
    align-items: center;
    gap: 5.6px;
    color: #8bdfb0;
    font-size: 0.608rem;
    margin: 0;
  }
  &__check {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 15.2px;
    height: 15.2px;
    flex-shrink: 0;
    color: #8bdfb0;
    border: 0.8px solid #8bdfb066;
    border-radius: 50%;
    background: #14231f;
    font-size: 8.8px;
  }
  &__package-name + &__status {
    margin-top: 14.4px;
  }
  &__progress {
    height: 4px;
    margin-top: 14.4px;
    background: #8bdfb0;
    border-radius: 3.2px;
    box-shadow: 0 0 11.2px #8bdfb02b;
  }
  &__dependency {
    left: 76%;
    width: 24%;
    min-height: 70.4px;
    padding: 12px;
    display: flex;
    align-items: center;
    gap: 9.6px;
    border-radius: 10.4px;
  }
  &__dependency img {
    width: 28.8px;
    height: 28.8px;
    flex-shrink: 0;
    object-fit: contain;
  }
  &__dependency-copy {
    min-width: 0;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 5.6px;
  }
  &__dependency-copy strong {
    flex-basis: 100%;
  }
  &__dependency > &__check {
    margin-left: auto;
  }
  &__dependency--playwright {
    left: 43%;
    top: 140.8px;
    width: 26%;
    border-color: #81dfef88;
    box-shadow: 0 0 28px #81dfef12;
  }
  &__dependency--hubspot {
    top: 6.4px;
  }
  &__dependency--notion {
    top: 96px;
  }
  &__dependency--figma {
    top: 185.6px;
  }
  &__dependency--context7 {
    top: 275.2px;
  }
  &__shared {
    padding: 1.6px 5.6px;
    border: 0.8px solid #81dfef99;
    border-radius: 799.2px;
    background: #81dfef16;
    color: #81dfef;
    font-size: 0.6rem;
    font-weight: 700;
  }
  figcaption {
    margin-top: 18.4px;
    padding-top: 14.4px;
    border-top: 0.8px solid #c6d6ff1a;
    color: #92a0b9;
    font-size: 0.608rem;
    line-height: 1.6;
    text-align: center;
  }
}

@media (max-width: 900px) {
  .registry-dependencies {
    width: calc(100% - 28px);
  }
  .dependency-scene {
    padding: 16px 11.2px;
    &__graph {
      height: 544px;
    }
    &__edges--desktop {
      display: none;
    }
    &__edges--mobile {
      display: block;
    }
    &__package {
      width: 45%;
      padding: 12px 8.8px;
    }
    &__package--marketing {
      top: 19.2px;
    }
    &__package--design {
      top: 321.6px;
    }
    &__package-name {
      flex-direction: column;
      align-items: flex-start;
      gap: 6.4px;
    }
    strong {
      font-size: clamp(0.7rem, 2.32vw, 0.8rem);
      overflow-wrap: anywhere;
    }
    &__status {
      font-size: 0.6rem;
    }
    &__dependency {
      left: 56%;
      width: 44%;
      padding: 9.6px 7.2px;
      gap: 6.4px;
      min-height: 70.4px;
    }
    &__dependency img {
      width: 20px;
      height: 20px;
    }
    &__dependency > &__check {
      display: none;
    }
    &__dependency--playwright {
      top: 228.8px;
    }
    &__dependency--hubspot {
      top: 0;
    }
    &__dependency--notion {
      top: 88px;
    }
    &__dependency--figma {
      top: 369.6px;
    }
    &__dependency--context7 {
      top: 457.6px;
    }
  }
}

/* Progressive enhancement: the complete diagram stays visible without JS. */
.dependency-scene--animate:not(.dependency-scene--revealed) {
  .dependency-scene__dependency {
    opacity: 0;
  }
  .dependency-scene__edges path {
    stroke-dasharray: 1;
    stroke-dashoffset: 1;
  }
}
.dependency-scene--animate.dependency-scene--revealed {
  .dependency-scene__edges path {
    stroke-dasharray: 1;
    animation: dependency-edge-arrive 650ms ease-out 120ms both;
  }
  .dependency-scene__design-edges path {
    animation-delay: 240ms;
  }
  .dependency-scene__dependency {
    animation: dependency-node-arrive 480ms ease-out var(--arrival-delay) both;
  }
}
@keyframes dependency-edge-arrive {
  from {
    stroke-dashoffset: 1;
  }
  to {
    stroke-dashoffset: 0;
  }
}
@keyframes dependency-node-arrive {
  from {
    opacity: 0;
    transform: translateX(-24px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}
@media (prefers-reduced-motion: reduce) {
  .dependency-scene .dependency-scene__dependency {
    animation: none;
    opacity: 1;
    transform: none;
  }
  .dependency-scene .dependency-scene__edges path {
    animation: none;
    stroke-dashoffset: 0;
  }
}
</style>
