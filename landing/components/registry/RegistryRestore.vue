<script setup lang="ts">
const { t } = useI18n();
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
        <a
          class="registry-restore__guide"
          href="https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md#restore-on-another-machine"
          target="_blank"
          rel="noreferrer"
          >{{ t('shell.restore.guide') }} <span aria-hidden="true">↗</span></a
        >
      </header>

      <div class="registry-restore__flow">
        <div class="registry-restore__machine">
          <div class="registry-restore__screen">
            <p class="registry-restore__stage-label">
              <span class="registry-restore__step" aria-hidden="true">01</span>
              {{ t('shell.restore.from') }}
            </p>
            <div class="registry-restore__screen-body">
              <CommandSnippet command="agentplugins deps export" inline />
            </div>
          </div>
          <div class="registry-restore__stand" aria-hidden="true" />
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
          <div class="registry-restore__screen">
            <p class="registry-restore__stage-label">
              <span class="registry-restore__step" aria-hidden="true">02</span>
              {{ t('shell.restore.to') }}
            </p>
            <div class="registry-restore__screen-body">
              <CommandSnippet command="agentplugins deps import --install" inline />
            </div>
          </div>
          <div class="registry-restore__stand" aria-hidden="true" />
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
    padding: clamp(20px, 4vw, 36px);
    border: 1px solid var(--line-strong);
    border-radius: 16px;
    background: var(--surface);
    overflow: hidden;

    &::before {
      content: '';
      position: absolute;
      inset-inline: 0;
      top: 0;
      height: 2px;
      background: linear-gradient(90deg, var(--cyan), var(--primary-bright));
      opacity: 0.8;
    }
  }

  &__header {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 14px 24px;
    margin-block-end: 26px;
  }

  h2 {
    margin: 0;
    font-size: clamp(1.5rem, 3vw, 2rem);
    line-height: 1.2;
    letter-spacing: -0.035em;
    text-wrap: balance;
  }

  &__guide {
    color: var(--primary-bright);
    font-size: 0.9rem;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  &__flow {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(220px, 250px) minmax(0, 1fr);
    align-items: stretch;
    gap: 16px;
    padding-block-end: 30px;
  }

  &__machine {
    position: relative;
    min-width: 0;
    padding: 12px 12px 26px;
    border: 1px solid var(--line-strong);
    border-radius: 18px;
    background: var(--surface-raised);
    box-shadow: 0 12px 28px color-mix(in srgb, var(--text) 9%, transparent);

    &::before {
      content: '';
      position: absolute;
      top: 5px;
      left: 50%;
      translate: -50% 0;
      width: 4px;
      height: 4px;
      border-radius: 50%;
      background: var(--line-strong);
    }

    &::after {
      content: '';
      position: absolute;
      bottom: 9px;
      left: 50%;
      translate: -50% 0;
      width: 5px;
      height: 5px;
      border-radius: 50%;
      background: var(--cyan);
      box-shadow: 0 0 8px color-mix(in srgb, var(--cyan) 75%, transparent);
    }
  }

  &__screen {
    // A monitor screen keeps the dark display palette in every theme.
    --surface-solid: #101522;
    --surface-raised: #172033;
    --text: #e0e6ff;
    --muted: #9aa6c5;
    --subtle: #7f8dac;
    --line: rgba(203, 211, 255, 0.12);
    --cyan: #00f0ff;

    overflow: hidden;
    border-radius: 10px;
    background:
      linear-gradient(155deg, color-mix(in srgb, var(--cyan) 7%, transparent), transparent 48%),
      var(--surface-solid);
    box-shadow:
      inset 0 0 0 1px var(--line),
      inset 0 0 30px color-mix(in srgb, var(--cyan) 6%, transparent);
  }

  &__stage-label {
    display: flex;
    align-items: center;
    gap: 9px;
    margin: 0;
    padding: 9px 13px;
    border-block-end: 1px solid var(--line);
    background: color-mix(in srgb, var(--surface-raised) 55%, transparent);
    color: var(--muted);
    font-size: 0.68rem;
    font-weight: 750;
    letter-spacing: 0.09em;
    text-transform: uppercase;
  }

  &__step {
    color: color-mix(in srgb, var(--text) 38%, transparent);
    font-size: 1.35rem;
    font-weight: 450;
    letter-spacing: -0.05em;
    line-height: 1;
    font-variant-numeric: tabular-nums;
  }

  &__screen-body {
    padding: 15px 13px 17px;

    // Flat on the screen: no card chrome, just the command row.
    :deep(.command-snippet.command-snippet--inline) {
      border: 0;
      border-radius: 0;
      background: transparent;
    }

    :deep(.command-snippet__header) {
      min-height: 0;
      padding: 0;
      border: 0;
      background: transparent;
    }

    :deep(.command-snippet button) {
      padding: 2px 4px;
      color: var(--muted);
    }

    :deep(pre) {
      direction: ltr;
      text-align: left;
      white-space: pre-wrap;
      overflow-wrap: break-word;
      padding: 0;
    }
  }

  &__stand {
    position: absolute;
    top: 100%;
    left: 50%;
    translate: -50% 0;
    width: 92px;
    height: 18px;
    background: var(--line-strong);
    clip-path: polygon(38% 0, 62% 0, 66% 58%, 94% 58%, 100% 100%, 0 100%, 6% 58%, 34% 58%);
  }

  &__handoff {
    position: relative;
    display: grid;
    grid-template-rows: 1fr auto 1fr;
    justify-items: center;
    min-width: 0;
  }

  &__files {
    position: absolute;
    left: 50%;
    bottom: calc(50% + 27px);
    translate: -50% 0;
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
    width: max-content;
    max-width: 100%;
  }

  &__file {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 8px 11px;
    border: 1px solid var(--line-strong);
    border-radius: 10px;
    background: color-mix(in srgb, var(--surface-solid) 55%, var(--surface-raised));
    font-size: 0.76rem;

    svg {
      width: 13px;
      height: 13px;
      flex: 0 0 13px;
      color: var(--primary-bright);
      stroke: currentColor;
      stroke-width: 1.8;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    code {
      color: var(--text);
    }
  }

  &__arrow {
    grid-row: 2;
    width: calc(100% + 32px);
    margin-inline: -16px;
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
    margin: 22px 0 0;
    padding-block-start: 16px;
    border-block-start: 1px solid var(--line);
    color: var(--muted);
    font-size: 0.78rem;
    line-height: 1.6;
  }
}

@media (max-width: 980px) {
  .registry-restore__flow {
    grid-template-columns: minmax(0, 1fr);
    padding-block-end: 26px;
  }

  .registry-restore__handoff {
    grid-template-rows: auto auto;
    gap: 12px;
    padding-block: 4px;
  }

  .registry-restore__files {
    position: static;
    translate: none;
    width: auto;
  }

  .registry-restore__arrow,
  :dir(rtl) .registry-restore__arrow {
    transform: rotate(90deg);
    width: 92px;
    height: 36px;
    // Rotation does not affect layout: reserve the full 92px visual height.
    margin-block: 28px;
    margin-inline: 0;
  }
}
</style>
