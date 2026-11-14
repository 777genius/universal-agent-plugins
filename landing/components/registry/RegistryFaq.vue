<script setup lang="ts">
defineProps<{ items: Array<{ question: string; answer: string }> }>();
const { t } = useI18n();
const expanded = ref<number[]>([]);

function toggle(index: number) {
  expanded.value = expanded.value.includes(index)
    ? expanded.value.filter((item) => item !== index)
    : [...expanded.value, index];
}
</script>

<template>
  <section id="faq" class="registry-faq container anchor-offset" aria-labelledby="faq-title" data-scroll-reveal>
    <div class="section-heading section-heading--center">
      <p class="eyebrow">{{ t('shell.faq.eyebrow') }}</p>
      <h2 id="faq-title">{{ t('shell.faq.title') }}</h2>
    </div>
    <div class="registry-faq__list">
      <div
        v-for="(item, index) in items"
        :key="item.question"
        class="registry-faq__item"
        :class="{ 'is-open': expanded.includes(index) }"
      >
        <button
          :id="`faq-question-${index}`"
          type="button"
          class="registry-faq__trigger"
          :aria-controls="`faq-answer-${index}`"
          :aria-expanded="expanded.includes(index)"
          @click="toggle(index)"
        >
          <span class="registry-faq__mark" aria-hidden="true">?</span>
          <span class="registry-faq__question">{{ item.question }}</span>
          <svg
            class="registry-faq__chevron"
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="m6 9 6 6 6-6" />
          </svg>
        </button>
        <div
          :id="`faq-answer-${index}`"
          class="registry-faq__panel"
          role="region"
          :aria-labelledby="`faq-question-${index}`"
          :aria-hidden="!expanded.includes(index)"
          :inert="!expanded.includes(index)"
        >
          <div class="registry-faq__panel-inner">
            <p>{{ item.answer }}</p>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
