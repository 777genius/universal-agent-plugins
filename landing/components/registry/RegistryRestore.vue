<script setup lang="ts">
import monitorImage from '~/assets/images/restore-monitor.png';

// Real Agent Plugins 1.0 package. The Git SHA pins the source, while the
// version below comes from that revision's plugin.json; they are not interchangeable.
const pluginName = 'modern-java-development';
const pluginVersion = '1.0.0';
const pluginSource =
  'github:brunoborges/javaevolved@4bfa3c268a4abb026b16c136330e389fb9cce091//agent-plugins/modern-java-development';
const addCommand = `agentplugins add ${pluginSource} --target codex --scope project`;
const restoreCommand = 'agentplugins install --locked';

const { t } = useI18n();
const { restoreGuideUrl, restoreGuideEnglishFallback, docsLabel } = useDocsLinks();
</script>

<template>
  <section
    id="restore-environment"
    class="registry-restore container"
    aria-labelledby="restore-title"
    dir="auto"
  >
    <div class="registry-restore__panel">
      <header class="registry-restore__header">
        <h2 id="restore-title">{{ t('shell.restore.title') }}</h2>
        <a class="registry-restore__guide" :href="restoreGuideUrl" target="_blank" rel="noreferrer"
          >{{ docsLabel(t('shell.restore.guide'), restoreGuideEnglishFallback) }}
          <span aria-hidden="true">↗</span></a
        >
      </header>
      <p class="registry-restore__subtitle">{{ t('shell.restore.subtitle') }}</p>

      <div class="registry-restore__flow">
        <div class="registry-restore__machine">
          <img
            class="registry-restore__monitor"
            :src="monitorImage"
            width="1577"
            height="997"
            alt=""
            loading="lazy"
            decoding="async"
            aria-hidden="true"
          />
          <div class="registry-restore__screen">
            <p class="registry-restore__stage-label" dir="auto">
              <span class="registry-restore__step" aria-hidden="true">01</span>
              {{ t('shell.restore.from') }}
            </p>
            <p class="registry-restore__package" dir="ltr">
              <span>{{ pluginName }}</span>
              <strong>v{{ pluginVersion }}</strong>
            </p>
            <div class="registry-restore__screen-body">
              <CommandSnippet :command="addCommand" inline>
                agentplugins <span class="registry-restore__accent">add</span>{{ ' ' }}
                <span class="registry-restore__source" :title="pluginSource"
                  >github:brunoborges/javaevolved…</span
                >
                <span class="registry-restore__source-compact">github:brunoborges/…</span>{{ ' ' }}
                <span class="registry-restore__argument"
                  ><span class="registry-restore__accent">--target</span> codex{{ ' ' }}
                  <span class="registry-restore__accent">--scope</span> project</span
                >
              </CommandSnippet>
              <p class="registry-restore__commit" dir="auto">{{ t('shell.restore.commit') }}</p>
            </div>
          </div>
        </div>

        <div class="registry-restore__handoff">
          <div class="registry-restore__files">
            <span class="registry-restore__file">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z" />
                <path d="M14 3v5h5" />
              </svg>
              <code dir="ltr">uap.toml</code>
            </span>
            <span class="registry-restore__file">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z" />
                <path d="M14 3v5h5" />
              </svg>
              <code dir="ltr">uap.lock</code>
            </span>
          </div>
          <code class="registry-restore__locked-version" dir="ltr">v{{ pluginVersion }}</code>
          <svg class="registry-restore__arrow" viewBox="0 0 160 36" fill="none" aria-hidden="true">
            <path d="M4 26C36 26 46 12 84 12h52" stroke-dasharray="3.5 7" />
            <path d="M138 5 157 12 138 19 141.5 12Z" fill="currentColor" stroke="none" />
          </svg>
          <i18n-t keypath="shell.restore.transfer" tag="p" class="sr-only">
            <template #manifest><code dir="ltr">uap.toml</code></template>
            <template #lockfile><code dir="ltr">uap.lock</code></template>
          </i18n-t>
        </div>

        <div class="registry-restore__machine">
          <img
            class="registry-restore__monitor"
            :src="monitorImage"
            width="1577"
            height="997"
            alt=""
            loading="lazy"
            decoding="async"
            aria-hidden="true"
          />
          <div class="registry-restore__screen">
            <p class="registry-restore__stage-label" dir="auto">
              <span class="registry-restore__step" aria-hidden="true">02</span>
              {{ t('shell.restore.to') }}
            </p>
            <p class="registry-restore__package" dir="ltr">
              <span>{{ pluginName }}</span>
              <strong>v{{ pluginVersion }}</strong>
            </p>
            <div class="registry-restore__screen-body">
              <CommandSnippet :command="restoreCommand" inline>
                agentplugins <span class="registry-restore__accent">install</span>{{ ' ' }}
                <span class="registry-restore__accent">--locked</span>
              </CommandSnippet>
            </div>
          </div>
        </div>
      </div>

      <p class="registry-restore__boundary">
        {{ t('shell.restore.requirements') }} {{ t('shell.restore.excludes') }}
      </p>
    </div>
  </section>
</template>

<style scoped lang="scss">
.registry-restore {
  padding-block: 32px 48px;

  &__panel {
    position: relative;
    padding: clamp(16px, 3vw, 36px);
    border: 1px solid var(--line-strong);
    border-radius: 16px;
    background: var(--surface);
  }

  &__header {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 14px 24px;
  }

  h2 {
    margin: 0;
    font-size: clamp(1.5rem, 3vw, 2.5rem);
    line-height: 1.2;
    letter-spacing: -0.035em;
    text-wrap: balance;
    background: linear-gradient(100deg, var(--text) 42%, var(--primary-bright) 85%);
    background-clip: text;
    color: transparent;
  }

  &__guide {
    color: var(--primary-bright);
    font-size: 0.9rem;
    text-decoration: underline;
    text-underline-offset: 4px;
  }

  &__subtitle {
    margin: 12px 0 20px;
    color: var(--muted);
    font-size: clamp(0.9rem, 1.5vw, 1.1rem);
    line-height: 1.6;
    text-wrap: pretty;
  }

  &__flow {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(120px, 0.36fr) minmax(0, 1fr);
    align-items: center;
    gap: 16px;
  }

  &__machine {
    position: relative;
    min-width: 0;
    container-type: inline-size;
    // Coordinates refer to the immutable 1577 × 997 monitor asset.
    // Match the approved 671 × 425 silhouette after the asset's alpha margins
    // (1496 × 872 visible pixels); compare at uniform scale, never stretch QA images.
    aspect-ratio: 1577 / 1083;
    direction: ltr;
  }

  &__monitor {
    display: block;
    width: 100%;
    height: 100%;
    pointer-events: none;
    user-select: none;
  }

  &__screen {
    position: absolute;
    inset: 21% 10% 29%;
    color: #e0e6ff;
    text-align: start;
    direction: inherit;
  }

  &__stage-label {
    display: flex;
    align-items: center;
    gap: 3cqw;
    margin: 0;
    color: #acb9ed;
    font-size: clamp(11px, 2.8cqw, 20px);
    font-weight: 600;
    line-height: 1.3;
    text-transform: uppercase;
  }

  &__step {
    flex: none;
    color: #7885ba;
    font-size: 6.5cqw;
    font-weight: 400;
    letter-spacing: -0.05em;
    line-height: 1;
  }

  &__package {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 1.5cqw;
    margin: 2cqw 0 0;
    color: #e0e6ff;
    font-family: 'JetBrains Mono', monospace;
    font-size: clamp(10px, 2.7cqw, 16px);
    line-height: 1.35;
    overflow-wrap: anywhere;

    strong {
      color: #00f0ff;
      font-weight: 650;
    }
  }

  &__screen-body {
    margin-block-start: 3.2cqw;

    :deep(.command-snippet.command-snippet--inline) {
      position: relative;
      display: block;
      overflow: visible;
      border: 0;
      border-radius: 0;
      background: transparent;
    }

    :deep(.command-snippet__header) {
      position: absolute;
      inset-inline-end: 0;
      bottom: calc(100% + 3px);
      min-height: 0;
      padding: 0;
      border: 0;
      background: transparent;
    }

    :deep(.command-snippet button) {
      padding: 3px;
      color: #acb9ed;
      font-size: max(9px, 1.8cqw);
      opacity: 0.7;
    }

    :deep(.command-snippet button:hover),
    :deep(.command-snippet button:focus-visible) {
      opacity: 1;
    }

    :deep(pre) {
      display: block;
      padding: 0;
      direction: ltr;
      text-align: left;
      white-space: normal;
      overflow: visible;
      overflow-wrap: normal;
      font-family: 'JetBrains Mono', monospace;
      font-size: clamp(10px, 2.8cqw, 18px);
      letter-spacing: -0.045em;
      line-height: 1.4;
    }

    :deep(code) {
      color: #e0e6ff;
      font: inherit;
    }
  }

  &__accent {
    color: #00f0ff;
  }
  &__source {
    display: block;
    font-size: 0.84em;
    overflow-wrap: anywhere;
  }

  &__source-compact {
    display: none;
  }

  &__argument {
    display: block;
  }

  &__commit {
    margin: 2cqw 0 0;
    color: #9198a8;
    font-family: 'JetBrains Mono', monospace;
    font-size: clamp(10px, 2.3cqw, 16px);
    line-height: 1.5;
  }

  &__handoff {
    display: grid;
    justify-items: center;
    gap: 14px;
    padding-block-end: 14%;
    min-width: 0;
  }

  &__files {
    display: flex;
    justify-content: center;
    gap: clamp(10px, 2vw, 26px);
  }

  &__file {
    display: grid;
    justify-items: center;
    gap: 7px;
    font-size: clamp(11px, 1vw, 16px);

    svg {
      width: clamp(25px, 2.7vw, 42px);
      height: clamp(30px, 3.3vw, 50px);
      color: var(--primary-bright);
      stroke: currentColor;
      stroke-width: 1.6;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    code {
      color: var(--text);
    }
  }

  &__locked-version {
    color: var(--primary-bright);
    font-size: clamp(12px, 1vw, 15px);
    font-weight: 700;
  }

  &__arrow {
    width: 100%;
    height: 36px;
    color: var(--cyan);
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  :dir(rtl) &__arrow {
    transform: scaleX(-1);
  }

  &__boundary {
    margin: 18px 0 0;
    padding-block-start: 20px;
    border-block-start: 1px solid var(--line);
    color: var(--muted);
    font-size: 0.78rem;
    line-height: 1.6;
    text-align: center;
  }
}

@media (max-width: 1100px) {
  .registry-restore {
    &__flow {
      grid-template-columns: minmax(0, 1fr);
      gap: 12px;
    }
    &__machine {
      width: 100%;
      max-width: 500px;
      margin-inline: auto;
    }
    &__handoff {
      padding: 0;
      gap: 4px;
    }
    &__files {
      gap: 24px;
    }
    &__file {
      font-size: 14px;
    }
    &__file svg {
      width: 30px;
      height: 36px;
    }
    &__arrow,
    :dir(rtl) &__arrow {
      transform: rotate(90deg);
      width: 70px;
      height: 36px;
      margin-block: 17px;
    }
  }
}

@media (max-width: 480px) {
  .registry-restore {
    &__panel {
      padding-inline: 8px;
    }
    &__header,
    &__subtitle {
      margin-inline: 8px;
    }
    &__screen {
      inset: 20% 10% 27%;
    }
    &__screen-body {
      margin-block-start: 2cqw;
    }
    &__source {
      display: none;
    }
    &__source-compact {
      display: inline;
    }
    &__argument {
      display: inline-block;
    }
    &__commit {
      margin-block-start: 2cqw;
    }
  }
}
</style>
