<script setup lang="ts">
defineProps<{ items: Array<{ question: string; answer: string }> }>();
const { t } = useI18n();
</script>

<template>
  <section id="faq" class="registry-faq container" aria-labelledby="faq-title" data-scroll-reveal>
    <div class="section-heading section-heading--center">
      <p class="eyebrow">{{ t('shell.faq.eyebrow') }}</p>
      <h2 id="faq-title">{{ t('shell.faq.title') }}</h2>
    </div>
    <div class="registry-faq__list">
      <details v-for="item in items" :key="item.question">
        <summary>{{ item.question }}<span aria-hidden="true">+</span></summary>
        <p>{{ item.answer }}</p>
      </details>
    </div>
  </section>
</template>
