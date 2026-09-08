<script setup lang="ts">
import { registryFaqItems } from '~/data/registryFaq';
</script>

<template>
  <section
    id="faq"
    class="registry-faq container"
    aria-labelledby="faq-title"
    data-scroll-reveal
  >
    <div class="section-heading section-heading--center">
      <p class="eyebrow">FAQ</p>
      <h2 id="faq-title">Common questions</h2>
    </div>
    <div class="registry-faq__list">
      <details v-for="item in registryFaqItems" :key="item.question">
        <summary>{{ item.question }}<span aria-hidden="true">+</span></summary>
        <p>{{ item.answer }}</p>
      </details>
    </div>
  </section>
</template>
