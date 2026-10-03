<script setup lang="ts">
const { t } = useI18n();
const { asset } = useSite();
</script>

<template>
  <section
    id="built-with-uap"
    class="built-with container"
    aria-labelledby="built-with-title"
    data-scroll-reveal
  >
    <div class="section-heading section-heading--center">
      <h2 id="built-with-title">{{ t('shell.builtWith.title') }}</h2>
    </div>
    <a
      class="built-with__project"
      href="https://github.com/777genius/agent-notifications"
      target="_blank"
      rel="noopener noreferrer"
    >
      <img
        class="built-with__logo"
        :src="asset('showcase/agent-notifications.png')"
        alt=""
        width="96"
        height="96"
        loading="lazy"
        decoding="async"
      />
      <div class="built-with__copy">
        <h3>Agent Notifications</h3>
        <p>{{ t('shell.builtWith.agentNotificationsDescription') }}</p>
      </div>
      <span class="built-with__link">
        {{ t('shell.builtWith.viewRepository') }}
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M7 17 17 7M7 7h10v10" />
        </svg>
      </span>
    </a>
  </section>
</template>

<style scoped>
.built-with {
  padding-block: 60px;
}

.built-with__project {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr) auto;
  align-items: center;
  gap: 28px;
  max-width: 900px;
  margin-inline: auto;
  padding: 32px;
  border: 1px solid var(--line-strong);
  border-radius: 24px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
  transition:
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.built-with__project:hover,
.built-with__project:focus-visible {
  border-color: var(--cyan);
  background: var(--surface-raised);
}

.built-with__logo {
  width: 96px;
  height: 96px;
  object-fit: contain;
}

.built-with__copy h3 {
  margin-bottom: 10px;
  font-size: 1.5rem;
}

.built-with__copy p {
  margin-bottom: 0;
  color: var(--muted);
}

.built-with__link {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--cyan);
  font-size: 0.875rem;
  font-weight: 700;
}

.built-with__link svg {
  flex-shrink: 0;
}

@media (max-width: 700px) {
  .built-with__project {
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 20px;
    padding: 24px;
  }

  .built-with__logo {
    width: 64px;
    height: 64px;
  }

  .built-with__copy {
    display: contents;
  }

  .built-with__copy h3 {
    align-self: center;
    margin-bottom: 0;
    font-size: 1.25rem;
  }

  .built-with__copy p,
  .built-with__link {
    grid-column: 1 / -1;
  }
}
</style>
