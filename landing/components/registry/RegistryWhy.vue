<script setup lang="ts">
const { t } = useI18n();
</script>

<template>
  <section
    id="why"
    class="registry-benefits container"
    aria-labelledby="why-title"
    data-scroll-reveal
  >
    <div class="section-heading section-heading--center">
      <p class="eyebrow">{{ t('shell.why.eyebrow') }}</p>
      <h2 id="why-title">{{ t('shell.why.title') }}<br >{{ t('shell.why.subtitle') }}</h2>
      <p>{{ t('shell.why.intro') }}</p>
    </div>
    <div class="registry-benefits__grid">
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">01</span>
          <svg
            viewBox="0 0 32 32"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m16 3 11 6v14l-11 6-11-6V9Z M5 9l11 6 11-6 M16 15v14 M10.5 6l11 6v6" />
          </svg>
        </div>
        <h3>{{ t('shell.why.standardTitle') }}</h3>
        <i18n-t keypath="shell.why.standardDescription" tag="p">
          <template #manifest><code>plugin.json</code></template>
        </i18n-t>
      </article>
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">02</span>
          <svg
            viewBox="0 0 32 32"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="3" y="12" width="8" height="8" rx="2" />
            <rect x="22" y="3" width="7" height="7" rx="2" />
            <rect x="22" y="22" width="7" height="7" rx="2" />
            <path d="M11 16h5V6.5h6 M16 16v9.5h6 M16 16h12" />
          </svg>
        </div>
        <h3>{{ t('shell.why.clientTitle') }}</h3>
        <p>
          {{ t('shell.why.clientDescription') }}
        </p>
      </article>
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">03</span>
          <svg
            viewBox="0 0 32 32"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M26 12a11 11 0 0 0-19-4L4 11 M4 5v6h6 M6 20a11 11 0 0 0 19 4l3-3 M28 27v-6h-6 M11 16l3 3 7-7"
            />
          </svg>
        </div>
        <h3>{{ t('shell.why.lifecycleTitle') }}</h3>
        <p>{{ t('shell.why.lifecycleDescription') }}</p>
      </article>
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">04</span>
          <svg
            viewBox="0 0 32 32"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M2 16s5-9 14-9 14 9 14 9-5 9-14 9S2 16 2 16Z" />
            <circle cx="16" cy="16" r="4" />
            <path d="M16 2v2 M16 28v2" />
          </svg>
        </div>
        <h3>{{ t('shell.why.sourceTitle') }}</h3>
        <p>
          {{ t('shell.why.sourceDescription') }}
        </p>
      </article>
    </div>
  </section>
</template>

<style src="~/assets/styles/registry-benefits.scss" lang="scss" />
