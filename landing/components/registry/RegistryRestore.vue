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
        <div>
          <p class="eyebrow">{{ t('shell.restore.eyebrow') }}</p>
          <h2 id="restore-title">{{ t('shell.restore.title') }}</h2>
        </div>
        <div class="registry-restore__meta">
          <AppTooltip>
            <template #trigger>
              <button type="button" class="registry-restore__availability">
                {{ t('shell.restore.availability') }}
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <circle cx="12" cy="12" r="9" />
                  <path d="M12 11v5M12 8h.01" />
                </svg>
              </button>
            </template>
            {{ t('shell.restore.releaseNote') }}
          </AppTooltip>
          <a
            class="registry-restore__guide"
            href="https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md#restore-on-another-machine"
            target="_blank"
            rel="noreferrer"
            >{{ t('shell.restore.guide') }} <span aria-hidden="true">↗</span></a
          >
        </div>
      </header>

      <div class="registry-restore__flow">
        <div class="registry-restore__stage">
          <p class="registry-restore__stage-label">
            <span class="registry-restore__step" aria-hidden="true">01</span>
            <svg
              class="registry-restore__stage-icon"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <rect x="3" y="5" width="18" height="12" rx="2" />
              <path d="M9 21h6M12 17v4" />
            </svg>
            {{ t('shell.restore.from') }}
          </p>
          <CommandSnippet command="agentplugins deps export" inline />
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
          <svg class="registry-restore__arrow" viewBox="0 0 96 24" fill="none" aria-hidden="true">
            <path d="M2 12h88M82 5l8 7-8 7" />
          </svg>
          <i18n-t keypath="shell.restore.transfer" tag="p" class="sr-only">
            <template #manifest><code dir="ltr">uap.toml</code></template>
            <template #lockfile><code dir="ltr">uap.lock</code></template>
          </i18n-t>
        </div>

        <div class="registry-restore__stage">
          <p class="registry-restore__stage-label">
            <span class="registry-restore__step" aria-hidden="true">02</span>
            <svg
              class="registry-restore__stage-icon"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <rect x="3" y="5" width="18" height="12" rx="2" />
              <path d="M9 21h6M12 17v4" />
            </svg>
            {{ t('shell.restore.to') }}
          </p>
          <CommandSnippet command="agentplugins deps import --install" inline />
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
    align-items: flex-end;
    justify-content: space-between;
    gap: 14px 24px;
    margin-block-end: 26px;
  }

  h2 {
    margin-block: 8px 0;
    font-size: clamp(1.5rem, 3vw, 2rem);
    line-height: 1.2;
    letter-spacing: -0.035em;
    text-wrap: balance;
  }

  .eyebrow {
    color: var(--cyan);
  }

  &__meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px 16px;
  }

  &__availability {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 5px 11px;
    border: 1px solid var(--line-strong);
    border-radius: 8px;
    color: var(--text);
    background: var(--surface-raised);
    font-size: 0.78rem;
    font-weight: 700;
    cursor: help;

    svg {
      width: 13px;
      height: 13px;
      color: var(--muted);
      stroke: currentColor;
      stroke-width: 1.9;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
  }

  &__guide {
    color: var(--primary-bright);
    font-size: 0.9rem;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  &__flow {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(150px, 200px) minmax(0, 1fr);
    align-items: stretch;
    gap: 16px;
  }

  &__stage {
    display: grid;
    align-content: start;
    gap: 12px;
    min-width: 0;
    padding: 16px;
    border: 1px solid var(--line);
    border-radius: 14px;
    background: var(--surface-raised);

    :deep(pre) {
      direction: ltr;
      text-align: left;
      overflow-wrap: anywhere;
    }
  }

  &__stage-label {
    display: flex;
    align-items: center;
    gap: 9px;
    margin: 0;
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

  &__stage-icon {
    width: 17px;
    height: 17px;
    flex: 0 0 17px;
    color: var(--cyan);
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  &__handoff {
    display: grid;
    align-content: center;
    justify-items: center;
    gap: 12px;
    min-width: 0;
  }

  &__files {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
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
    width: min(100%, 140px);
    height: 22px;
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
  }

  .registry-restore__stage :deep(.command-snippet--inline pre),
  .registry-restore__stage :deep(.command-snippet--inline code) {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .registry-restore__handoff {
    gap: 10px;
    padding-block: 2px;
  }

  .registry-restore__arrow,
  :dir(rtl) .registry-restore__arrow {
    transform: rotate(90deg);
    width: 44px;
    height: 30px;
  }
}
</style>
