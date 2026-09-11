<script setup lang="ts">
const { t } = useI18n();
</script>

<template>
  <section id="authoring-hero" class="authoring-hero-section section anchor-offset">
    <v-container class="authoring-hero-section__container">
      <p class="authoring-hero-section__eyebrow">
        {{ t('pluginAuthoring.eyebrow') }}
      </p>
      <h1 class="authoring-hero-section__title">
        {{ t('pluginAuthoring.title') }}
      </h1>
      <p class="authoring-hero-section__subtitle">
        {{ t('pluginAuthoring.subtitle') }}
      </p>
    </v-container>
  </section>
</template>

<style scoped>
.authoring-hero-section__container {
  max-width: 860px;
  text-align: center;
  margin: 0 auto;
}

.authoring-hero-section__eyebrow {
  margin: 0 0 14px;
  color: #7257ff;
  text-transform: uppercase;
  letter-spacing: 0.16em;
  font-size: 0.74rem;
  font-family: 'JetBrains Mono', monospace;
}

.authoring-hero-section__title {
  margin: 0 0 18px;
  font-size: clamp(2.4rem, 5vw, 3.6rem);
  line-height: 1.05;
  letter-spacing: -0.04em;
  color: #eff6ff;
}

.authoring-hero-section__subtitle {
  margin: 0 auto;
  color: #91a0bf;
  font-size: 1.08rem;
  line-height: 1.72;
  max-width: 700px;
}
</style>
