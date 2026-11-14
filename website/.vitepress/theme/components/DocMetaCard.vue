<script setup lang="ts">
import { computed } from "vue";
import { useData } from "vitepress";

const { lang } = useData();

const props = defineProps<{
  surface: string;
  stability: string;
  maturity?: string;
  sourceRef?: string;
  sourceHref?: string;
}>();

const translations: Record<string, { source: string; surface: string; stability: string; maturity: string }> = {
  en: { source: "Source", surface: "Surface", stability: "Stability", maturity: "Maturity" },
  ru: { source: "Исходник", surface: "Раздел", stability: "Стабильность", maturity: "Зрелость" },
  uk: { source: "Джерело", surface: "Розділ", stability: "Стабільність", maturity: "Зрілість" },
  zh: { source: "源代码", surface: "类别", stability: "稳定性", maturity: "成熟度" },
  es: { source: "Fuente", surface: "Sección", stability: "Estabilidad", maturity: "Madurez" },
  hi: { source: "स्रोत", surface: "अनुभाग", stability: "स्थिरता", maturity: "परिपक्वता" },
  ar: { source: "المصدر", surface: "القسم", stability: "الاستقرار", maturity: "النضج" },
  pt: { source: "Código-fonte", surface: "Seção", stability: "Estabilidade", maturity: "Maturidade" },
  fr: { source: "Source", surface: "Section", stability: "Stabilité", maturity: "Maturité" }
};
const labels = computed(() => translations[lang.value.split("-")[0]] || translations.en);

const prettySurface = computed(() => formatLabel(props.surface));
const prettyStability = computed(() => formatLabel(props.stability));
const prettyMaturity = computed(() => (props.maturity ? formatLabel(props.maturity) : ""));

function formatLabel(value: string) {
  return value
    .replaceAll("-", " ")
    .replaceAll("_", " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}
</script>

<template>
  <div class="doc-meta-card">
    <span class="doc-meta-card__chip doc-meta-card__chip--surface">{{ labels.surface }}: {{ prettySurface }}</span>
    <span class="doc-meta-card__chip" :class="`doc-meta-card__chip--${stability}`">{{ labels.stability }}: {{ prettyStability }}</span>
    <span v-if="maturity" class="doc-meta-card__chip doc-meta-card__chip--maturity">{{ labels.maturity }}: {{ prettyMaturity }}</span>
    <a v-if="sourceHref" class="doc-meta-card__source" :href="sourceHref" target="_blank" rel="noreferrer">
      {{ labels.source }}
    </a>
    <code v-else-if="sourceRef" dir="ltr" class="doc-meta-card__source">{{ sourceRef }}</code>
  </div>
</template>
