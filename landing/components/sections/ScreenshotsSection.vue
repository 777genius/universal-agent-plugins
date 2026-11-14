<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue';
import { register } from 'swiper/element/bundle';
import { mdiChevronLeft, mdiChevronRight } from '@mdi/js';

type SwiperContainerElement = HTMLElement & {
  initialize: () => void;
  swiper?: {
    slidePrev: () => void;
    slideNext: () => void;
  };
};

const { content } = useLandingContent();
const { t } = useI18n();
const localePath = useLocalePath();

register();

const swiperRef = ref<SwiperContainerElement | null>(null);
const swiperReady = ref(false);
const selectedPluginType = ref('all');
const accents = ['#00f0ff', '#ff00ff', '#39ff14', '#ffd700', '#00f0ff', '#ff00ff'];
const catalogPath = computed(() => localePath('/plugins'));
const pluginDetailPath = (slug: string) => localePath(`/plugins/${slug}`);

const pluginTypeOptions = computed(() => {
  const types = new Set<string>();
  for (const plugin of content.value.plugins) {
    types.add(plugin.pluginType);
  }
  return ['all', ...types];
});

const visiblePlugins = computed(() =>
  content.value.plugins.filter((plugin) =>
    selectedPluginType.value === 'all' ? true : plugin.pluginType === selectedPluginType.value,
  ),
);

function initializeSwiper() {
  if (!swiperRef.value) return;
  Object.assign(swiperRef.value, {
    slidesPerView: 1.08,
    spaceBetween: 16,
    loop: true,
    grabCursor: false,
    centeredSlides: false,
    pagination: {
      clickable: true,
    },
    injectStyles: [
      `
      .swiper-pagination {
        position: static !important;
        bottom: auto !important;
        inset: auto !important;
        display: flex;
        justify-content: center;
        align-items: center;
        gap: 10px;
        margin-top: 40px;
        padding-top: 4px;
        padding-bottom: 12px;
      }
      .swiper-pagination-bullet {
        width: 10px;
        height: 10px;
        background: rgba(0, 240, 255, 0.4);
        opacity: 1;
        margin: 0 !important;
      }
      .swiper-pagination-bullet-active {
        background: #00f0ff;
        width: 28px;
        border-radius: 5px;
      }
    `,
    ],
    breakpoints: {
      640: {
        slidesPerView: 1.35,
        spaceBetween: 18,
      },
      700: {
        slidesPerView: 1.7,
        spaceBetween: 20,
      },
      960: {
        slidesPerView: 2,
        spaceBetween: 24,
      },
      1264: {
        slidesPerView: 3,
        spaceBetween: 24,
      },
      1440: {
        slidesPerView: 3,
        spaceBetween: 24,
      },
    },
  });
  swiperRef.value.initialize();
  swiperReady.value = true;
}

onMounted(() => {
  initializeSwiper();
});

watch(selectedPluginType, async () => {
  swiperReady.value = false;
  await nextTick();
  initializeSwiper();
});

function slidePrev() {
  swiperRef.value?.swiper?.slidePrev();
}

function slideNext() {
  swiperRef.value?.swiper?.slideNext();
}
</script>

<template>
  <section id="plugins" class="screenshots-section section anchor-offset">
    <v-container>
      <div class="screenshots-section__header">
        <div>
          <h2 class="screenshots-section__title">
            {{ t('plugins.sectionTitle') }}
          </h2>
          <p class="screenshots-section__subtitle">
            {{ t('plugins.sectionSubtitle') }}
          </p>
          <div class="screenshots-section__type-chips">
            <button
              v-for="pluginType in pluginTypeOptions"
              :key="pluginType"
              type="button"
              class="screenshots-section__type-chip"
              :class="{ 'is-active': selectedPluginType === pluginType }"
              @click="selectedPluginType = pluginType"
            >
              {{
                pluginType === 'all'
                  ? t('plugins.allTypes')
                  : t(`plugins.types.${pluginType}`)
              }}
            </button>
          </div>
        </div>
        <v-btn
          variant="outlined"
          size="large"
          :to="catalogPath"
          class="screenshots-section__view-all"
        >
          {{ t('plugins.viewAll') }}
        </v-btn>
      </div>
    </v-container>

    <div class="screenshots-section__carousel-wrap" :class="{ 'is-ready': swiperReady }">
      <swiper-container
        :key="selectedPluginType"
        ref="swiperRef"
        init="false"
        class="screenshots-section__swiper"
      >
        <swiper-slide
          v-for="(plugin, idx) in visiblePlugins"
          :key="plugin.id"
          class="screenshots-section__slide"
        >
          <PluginCard
            :plugin="plugin"
            :accent="accents[idx % accents.length]"
            :supports-label="t('plugins.supports')"
            :open-label="t('plugins.viewDetails')"
            :to="pluginDetailPath(plugin.slug)"
          />
        </swiper-slide>
      </swiper-container>

      <button
        class="screenshots-section__nav screenshots-section__nav--prev"
        :aria-label="t('common.previous')"
        @click="slidePrev"
      >
        <v-icon :icon="mdiChevronLeft" size="28" />
      </button>
      <button
        class="screenshots-section__nav screenshots-section__nav--next"
        :aria-label="t('common.next')"
        @click="slideNext"
      >
        <v-icon :icon="mdiChevronRight" size="28" />
      </button>
    </div>
  </section>
</template>

<style scoped>
.screenshots-section {
  position: relative;
}

.screenshots-section__header {
  max-width: 1080px;
  margin: 0 auto 48px;
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}

.screenshots-section__title {
  font-size: 2.4rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #e0e6ff 0%, #00f0ff 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.screenshots-section__subtitle {
  font-size: 1.1rem;
  color: #8892b0;
  line-height: 1.6;
  margin: 0;
  max-width: 640px;
}

.screenshots-section__type-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}

.screenshots-section__type-chip {
  border-radius: 999px;
  border: 1px solid rgba(0, 240, 255, 0.16);
  background: rgba(10, 10, 15, 0.7);
  color: #dbe7ff;
  padding: 9px 14px;
  font-size: 0.74rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.2s ease,
    background 0.2s ease,
    transform 0.2s ease;
}

.screenshots-section__type-chip:hover {
  transform: translateY(-1px);
  border-color: rgba(0, 240, 255, 0.32);
}

.screenshots-section__type-chip.is-active {
  background: rgba(0, 240, 255, 0.12);
  color: #7dd3fc;
  border-color: rgba(125, 211, 252, 0.28);
}

.screenshots-section__carousel-wrap {
  position: relative;
  overflow: hidden;
}

.screenshots-section__swiper {
  display: block;
  padding: 18px 40px 136px;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.screenshots-section__carousel-wrap.is-ready .screenshots-section__swiper {
  opacity: 1;
}

.screenshots-section__slide {
  height: auto;
  padding-bottom: 52px;
}

.screenshots-section__view-all {
  border-color: rgba(0, 240, 255, 0.22) !important;
  color: #00f0ff !important;
  font-weight: 700 !important;
  letter-spacing: 0.03em !important;
  flex-shrink: 0;
}

.screenshots-section__view-all:hover {
  border-color: rgba(0, 240, 255, 0.42) !important;
  background: rgba(0, 240, 255, 0.06) !important;
}

.screenshots-section__nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 3;
  width: 48px;
  height: 48px;
  border-radius: 999px;
  border: 1px solid rgba(0, 240, 255, 0.14);
  background: rgba(10, 10, 15, 0.8);
  color: #e0e6ff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition:
    transform 0.2s ease,
    background 0.2s ease,
    border-color 0.2s ease;
}

.screenshots-section__nav:hover {
  transform: translateY(-50%) scale(1.04);
  border-color: rgba(0, 240, 255, 0.28);
}

.screenshots-section__nav--prev {
  left: 16px;
}

.screenshots-section__nav--next {
  right: 16px;
}

.v-theme--light .screenshots-section__title {
  background: linear-gradient(135deg, #1e293b 0%, #0891b2 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.v-theme--light .screenshots-section__type-chip {
  background: rgba(255, 255, 255, 0.9);
  color: #0f172a;
  border-color: rgba(14, 165, 233, 0.16);
}

.v-theme--light .screenshots-section__subtitle {
  color: #475569;
}

@media (max-width: 959px) {
  .screenshots-section__nav {
    display: none;
  }

  .screenshots-section__header {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 600px) {
  .screenshots-section__swiper {
    padding: 14px 20px 108px;
  }

  .screenshots-section__header {
    margin-bottom: 36px;
  }

  .screenshots-section__title {
    font-size: 1.9rem;
  }

  .screenshots-section__card {
    min-height: 320px;
    padding: 20px;
  }
}
</style>
