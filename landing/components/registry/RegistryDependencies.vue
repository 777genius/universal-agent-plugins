<script setup lang="ts">
const { t } = useI18n();
const { asset } = useSite();
// The film's final dependency scene: both demonstration packages share Playwright.
const packages = [
  { id: 'marketing', name: 'marketing-kit', glyph: 'plugin' },
  { id: 'design', name: 'Web Design Kit', glyph: 'code' },
];
const dependencies = [
  { id: 'playwright', name: 'Playwright', shared: true },
  { id: 'hubspot', name: 'HubSpot' },
  { id: 'notion', name: 'Notion' },
  { id: 'figma', name: 'Figma' },
  { id: 'context7', name: 'Context7', existing: true },
];
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

    <figure class="dependency-scene">
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
            <path d="M290 126C365 126 350 220 430 220" />
            <path d="M290 106C460 106 570 52 760 52" />
            <path d="M290 146C460 75 590 164 760 164" />
          </g>
          <g class="dependency-scene__design-edges">
            <path d="M290 320C365 320 350 220 430 220" />
            <path d="M290 340C465 340 565 276 760 276" />
            <path d="M290 360C465 360 565 388 760 388" />
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
            <path d="M162 65C180 65 180 44 202 44" />
            <path d="M162 90C183 90 180 154 202 154" />
            <path d="M162 116C180 116 180 330 202 330" />
          </g>
          <g class="dependency-scene__design-edges">
            <path d="M162 444C180 444 180 330 202 330" />
            <path d="M162 470C182 470 180 506 202 506" />
            <path d="M162 495C182 495 180 616 202 616" />
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
          <p class="dependency-scene__demo" dir="auto">{{ t('shell.dependencies.demoPackage') }}</p>
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
          aria-hidden="true"
        >
          <img
            :src="asset(`dependency-icons/${dependency.id}.svg`)"
            alt=""
            width="36"
            height="36"
            loading="lazy"
          >
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
  padding-block: 34px 64px;

  .section-heading {
    margin-block-end: 30px;
  }
  h2 {
    text-wrap: balance;
  }
}

.dependency-scene {
  margin: 0;
  padding: clamp(18px, 3.5vw, 40px);
  border: 1px solid var(--line-strong);
  border-radius: 20px;
  background:
    radial-gradient(ellipse at 8% 20%, #a98bfa0d, transparent 50%),
    radial-gradient(ellipse at 55% 60%, #81dfef09, transparent 45%), #0b101c;
  color: #e1e8f6;
  box-shadow: 0 20px 60px #00000012;

  &__graph {
    position: relative;
    height: 440px;
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
    border: 1px solid #c6d6ff2c;
    background: #101624;
  }
  &__package {
    left: 0;
    width: 29%;
    padding: 22px;
    border-radius: 16px;
  }
  &__package--marketing {
    top: 42px;
    --package-color: #ad92ef;
  }
  &__package--design {
    top: 264px;
    --package-color: #81dfef;
  }
  &__package-name {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  &__package-name svg {
    width: 29px;
    height: 29px;
    flex-shrink: 0;
    stroke: var(--package-color);
    stroke-width: 1.5;
    stroke-linejoin: round;
    stroke-linecap: round;
  }
  strong {
    font-size: clamp(0.9rem, 1.6vw, 1.2rem);
    font-weight: 600;
    letter-spacing: -0.025em;
  }
  &__demo {
    margin: 12px 0 18px;
    color: #92a0b9;
    font-size: 0.76rem;
  }
  &__status {
    display: flex;
    align-items: center;
    gap: 7px;
    color: #8bdfb0;
    font-size: 0.76rem;
    margin: 0;
  }
  &__check {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 19px;
    height: 19px;
    flex-shrink: 0;
    color: #8bdfb0;
    border: 1px solid #8bdfb066;
    border-radius: 50%;
    background: #14231f;
    font-size: 11px;
  }
  &__progress {
    height: 5px;
    margin-top: 18px;
    background: #8bdfb0;
    border-radius: 4px;
    box-shadow: 0 0 14px #8bdfb02b;
  }
  &__dependency {
    left: 76%;
    width: 24%;
    min-height: 88px;
    padding: 15px;
    display: flex;
    align-items: center;
    gap: 12px;
    border-radius: 13px;
  }
  &__dependency img {
    flex-shrink: 0;
    object-fit: contain;
  }
  &__dependency-copy {
    min-width: 0;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 5px 7px;
  }
  &__dependency-copy strong {
    flex-basis: 100%;
  }
  &__dependency > &__check {
    margin-left: auto;
  }
  &__dependency--playwright {
    left: 43%;
    top: 176px;
    width: 26%;
    border-color: #81dfef88;
    box-shadow: 0 0 35px #81dfef12;
  }
  &__dependency--hubspot {
    top: 8px;
  }
  &__dependency--notion {
    top: 120px;
  }
  &__dependency--figma {
    top: 232px;
  }
  &__dependency--context7 {
    top: 344px;
  }
  &__shared {
    padding: 2px 7px;
    border: 1px solid #81dfef99;
    border-radius: 999px;
    background: #81dfef16;
    color: #81dfef;
    font-size: 0.65rem;
    font-weight: 700;
  }
  figcaption {
    margin-top: 23px;
    padding-top: 18px;
    border-top: 1px solid #c6d6ff1a;
    color: #92a0b9;
    font-size: 0.76rem;
    line-height: 1.6;
    text-align: center;
  }
}

@media (max-width: 900px) {
  .dependency-scene {
    padding: 20px 14px;
    &__graph {
      height: 680px;
    }
    &__edges--desktop {
      display: none;
    }
    &__edges--mobile {
      display: block;
    }
    &__package {
      width: 45%;
      padding: 15px 11px;
    }
    &__package--marketing {
      top: 24px;
    }
    &__package--design {
      top: 402px;
    }
    &__package-name {
      flex-direction: column;
      align-items: flex-start;
      gap: 8px;
    }
    strong {
      font-size: clamp(0.76rem, 2.9vw, 1rem);
      overflow-wrap: anywhere;
    }
    &__demo {
      font-size: 0.65rem;
      margin: 10px 0 14px;
    }
    &__status {
      font-size: 0.65rem;
    }
    &__dependency {
      left: 56%;
      width: 44%;
      padding: 12px 9px;
      gap: 8px;
      min-height: 88px;
    }
    &__dependency img {
      width: 25px;
      height: 25px;
    }
    &__dependency > &__check {
      display: none;
    }
    &__dependency--playwright {
      top: 286px;
    }
    &__dependency--hubspot {
      top: 0;
    }
    &__dependency--notion {
      top: 110px;
    }
    &__dependency--figma {
      top: 462px;
    }
    &__dependency--context7 {
      top: 572px;
    }
  }
}
</style>
