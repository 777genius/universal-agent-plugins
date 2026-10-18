<script setup lang="ts">
const { t } = useI18n();
</script>

<template>
  <section
    id="restore-environment"
    class="registry-restore container"
    aria-labelledby="restore-title"
  >
    <div class="registry-restore__panel">
      <div class="registry-restore__copy">
        <p class="eyebrow">{{ t('shell.restore.eyebrow') }}</p>
        <h2 id="restore-title">{{ t('shell.restore.title') }}</h2>
        <p>{{ t('shell.restore.intro') }}</p>
        <p class="registry-restore__availability">
          <strong>{{ t('shell.restore.availability') }}</strong>
          <span>{{ t('shell.restore.releaseNote') }}</span>
        </p>
        <a
          class="registry-restore__guide"
          href="https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md#restore-on-another-machine"
          target="_blank"
          rel="noreferrer"
          >{{ t('shell.restore.guide') }} <span aria-hidden="true">↗</span></a
        >
      </div>
      <div class="registry-restore__steps">
        <CommandSnippet
          command="agentplugins deps export"
          :label="t('shell.restore.from')"
        />
        <i18n-t keypath="shell.restore.transfer" tag="p">
          <template #manifest><code dir="ltr">uap.toml</code></template>
          <template #lockfile><code dir="ltr">uap.lock</code></template>
        </i18n-t>
        <CommandSnippet
          command="agentplugins deps import --install"
          :label="t('shell.restore.to')"
        />
        <p>{{ t('shell.restore.directory') }}</p>
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
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 24px 40px;
    padding: clamp(20px, 4vw, 36px);
    border: 1px solid var(--line-strong);
    border-radius: 16px;
    background: var(--surface);
  }

  h2 {
    margin-block: 8px 12px;
    font-size: clamp(1.5rem, 3vw, 2rem);
    line-height: 1.2;
    letter-spacing: -0.035em;
    text-wrap: balance;
  }

  p {
    margin-block: 0 14px;
    color: var(--muted);
    font-size: 0.9rem;
    line-height: 1.65;
  }

  .eyebrow {
    color: var(--cyan);
  }

  &__availability {
    display: grid;
    gap: 6px;

    strong {
      width: fit-content;
      padding: 3px 9px;
      border: 1px solid var(--line-strong);
      border-radius: 6px;
      color: var(--text);
      background: var(--surface-raised);
      font-size: 0.8rem;
    }
  }

  &__guide {
    color: var(--primary-bright);
    font-size: 0.9rem;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  &__steps {
    min-width: 0;

    p {
      margin-block: 12px;
      font-size: 0.8rem;
    }

    code {
      color: var(--text);
    }

    :deep(pre) {
      direction: ltr;
      text-align: left;
      overflow-wrap: anywhere;
    }
  }

  .registry-restore__boundary {
    grid-column: 1 / -1;
    margin: 0;
    padding-block-start: 16px;
    border-block-start: 1px solid var(--line);
    font-size: 0.8rem;
  }
}

@media (max-width: 720px) {
  .registry-restore__panel {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
