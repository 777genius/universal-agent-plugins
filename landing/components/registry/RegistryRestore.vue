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
        <div class="registry-restore__stage">
          <p class="registry-restore__stage-label">
            <span class="registry-restore__step" aria-hidden="true">01</span>
            <svg
              class="registry-restore__stage-icon"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <rect x="3" y="4.5" width="18" height="12.5" rx="2.2" />
              <path d="M9 21h6M12 17v4" />
              <path d="M7 8.8l2.2 1.9-2.2 1.9" />
              <path d="M8.8 12.6h4.8" />
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
          <svg class="registry-restore__arrow" viewBox="0 0 96 32" fill="none" aria-hidden="true">
            <path d="M5 27.5C18 26 27 10 48 8.5h27" stroke-dasharray="3 6" />
            <path d="M78 2.5 92 8.5 78 14.5 80.5 8.5Z" fill="currentColor" stroke="none" />
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
              <rect x="4.5" y="5" width="15" height="11.2" rx="1.8" />
              <path d="M2.5 19.8h19l-1.7-3.6H4.2z" />
              <path d="M8.6 10.8l2.2 2.2 4-4.4" />
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
    width: 18px;
    height: 18px;
    flex: 0 0 18px;
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
    width: min(100%, 150px);
    height: 30px;
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
    width: 56px;
    height: 34px;
  }
}
</style>
