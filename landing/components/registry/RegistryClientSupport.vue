<script setup lang="ts">
import { clientSupportRows, clientSupportCapabilityKeys } from '~/data/clientSupport';
import { clientLandingById } from '~/data/clients';

const { asset } = useSite();
const { t } = useI18n();
const capabilityLabel = (value: string) => {
  const key = clientSupportCapabilityKeys[value as keyof typeof clientSupportCapabilityKeys];
  return key ? t(`shell.clientSupport.capabilities.${key}`) : value;
};
// SYNC: landing/data/clientSupport.ts and README.md "Supported clients" must
// match. Icons come from landing/data/clients.ts; parity is checked by
// landing/tests/client-support-readme.test.ts.
const clients = clientSupportRows.map((client) => ({
  ...client,
  icon: clientLandingById.get(client.id)!.icon,
}));
const monochromeIcons = new Set(['opencode', 'cline', 'windsurf', 'grok', 'kimi']);
</script>

<template>
  <section
    id="agent-support"
    class="registry-client-support container anchor-offset"
    aria-labelledby="agent-support-title"
    data-scroll-reveal
  >
    <header class="section-heading section-heading--center">
      <p class="eyebrow">{{ t('shell.clientSupport.eyebrow') }}</p>
      <h2 id="agent-support-title">{{ t('shell.clientSupport.title') }}</h2>
      <p>{{ t('shell.clientSupport.description') }}</p>
    </header>

    <div
      class="registry-client-support__scroll"
      role="region"
      :aria-label="t('shell.clientSupport.tableLabel')"
      tabindex="0"
    >
      <table>
        <caption class="sr-only">{{ t('shell.clientSupport.tableCaption') }}</caption>
        <thead>
          <tr>
            <th scope="col">{{ t('shell.clientSupport.columns.agent') }}</th>
            <th scope="col">{{ t('shell.clientSupport.columns.user') }}</th>
            <th scope="col">{{ t('shell.clientSupport.columns.project') }}</th>
            <th scope="col">{{ t('shell.clientSupport.columns.setup') }}</th>
            <th scope="col">{{ t('shell.clientSupport.columns.nextStep') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="client in clients" :key="client.name">
            <th scope="row">
              <span class="registry-client-support__agent">
                <img
                  :src="asset(`client-icons/${client.icon}`)"
                  :class="{ 'registry-client-support__icon--contrast': monochromeIcons.has(client.id) }"
                  class="registry-client-support__icon"
                  alt=""
                  aria-hidden="true"
                  width="24"
                  height="24"
                  loading="lazy"
                  decoding="async"
                >
                <span>{{ client.name }}</span>
              </span>
            </th>
            <td>{{ capabilityLabel(client.user) }}</td>
            <td :class="{ 'registry-client-support__unavailable': client.project === 'Not supported' }">
              {{ capabilityLabel(client.project) }}
            </td>
            <td>
              <span
                class="registry-client-support__setup"
                :class="`registry-client-support__setup--${client.setup.toLowerCase()}`"
              >
                {{ t(`shell.clientSupport.setup.${client.setup.toLowerCase()}`) }}
              </span>
            </td>
            <td>{{ t(`shell.clientSupport.nextSteps.${client.id}`) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="registry-client-support__scroll-hint">{{ t('shell.clientSupport.scrollHint') }}</p>
    <p class="registry-client-support__note">
      {{ t('shell.clientSupport.note') }}
      <a
        href="https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md"
        target="_blank"
        rel="noopener noreferrer"
      >{{ t('shell.clientSupport.details') }}</a>.
    </p>
  </section>
</template>

<style scoped lang="scss">
.registry-client-support {
  padding-block: 64px 72px;

  .section-heading {
    max-width: 760px;
  }

  h2 {
    font-size: clamp(1.8rem, 3.4vw, 2.8rem);
  }

  &__scroll {
    max-width: 100%;
    overflow-x: auto;
    border: 1px solid var(--line-strong);
    border-radius: 16px;
    background: var(--surface);
    box-shadow: 0 14px 38px color-mix(in srgb, var(--text) 4%, transparent);
    overscroll-behavior-inline: contain;
  }

  table {
    width: 100%;
    min-width: 900px;
    border-collapse: collapse;
    text-align: start;
    line-height: 1.45;
  }

  &__agent {
    display: inline-flex;
    align-items: center;
    gap: 9px;
  }
  &__icon {
    width: 24px;
    height: 24px;
    flex: none;
    object-fit: contain;
  }
  &__scroll-hint {
    display: none;
    margin: 8px 0 0;
    color: var(--muted);
    font-size: 0.76rem;
    text-align: center;
  }

  th,
  td {
    padding: 13px 15px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    font-size: 0.81rem;
  }

  thead {
    background: var(--surface-raised);
  }

  thead th {
    color: var(--muted);
    font-size: 0.73rem;
    font-weight: 700;
    white-space: nowrap;
  }

  tbody th {
    color: var(--text);
    font-weight: 700;
    white-space: nowrap;
  }

  tbody td {
    color: var(--muted);
  }

  tbody td:nth-child(2),
  tbody td:nth-child(3) {
    color: var(--text);
  }

  tbody tr:last-child th,
  tbody tr:last-child td {
    border-bottom: 0;
  }

  tbody tr:hover {
    background: color-mix(in srgb, var(--primary-bright) 5%, transparent);
  }

  &__unavailable {
    color: var(--muted) !important;
  }

  &__setup {
    display: inline-block;
    padding: 4px 9px;
    border: 1px solid var(--line-strong);
    border-radius: 7px;
    color: var(--text);
    background: var(--surface-raised);
    font-size: 0.73rem;
    font-weight: 650;
    white-space: nowrap;
  }

  &__setup--automatic {
    color: var(--green);
    border-color: color-mix(in srgb, var(--green) 35%, var(--line));
    background: color-mix(in srgb, var(--green) 8%, transparent);
  }

  &__note {
    max-width: 970px;
    margin: 14px auto 0;
    color: var(--muted);
    font-size: 0.78rem;
    line-height: 1.65;
    text-align: center;
  }

  &__note a {
    color: var(--primary-bright);
    text-underline-offset: 3px;
  }
}

:global(.v-theme--dark .registry-client-support__icon--contrast) {
  filter: brightness(0) invert(1);
}

@media (max-width: 720px) {
  .registry-client-support {
    padding-block: 48px 56px;

    table {
      min-width: 880px;
    }
    &__scroll-hint {
      display: block;
    }
  }
}
</style>
