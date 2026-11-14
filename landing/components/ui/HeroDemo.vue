<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { mdiCheckCircleOutline, mdiClockOutline } from '@mdi/js';

const { t } = useI18n();

const steps = computed(() => [
  {
    id: 'author',
    label: t('hero.demo.steps.author'),
    caption: t('hero.demo.captions.author'),
    accent: '#00f0ff',
  },
  {
    id: 'generate',
    label: t('hero.demo.steps.generate'),
    caption: t('hero.demo.captions.generate'),
    accent: '#ff00ff',
  },
  {
    id: 'validate',
    label: t('hero.demo.steps.validate'),
    caption: t('hero.demo.captions.validate'),
    accent: '#ffd700',
  },
  {
    id: 'ship',
    label: t('hero.demo.steps.ship'),
    caption: t('hero.demo.captions.ship'),
    accent: '#39ff14',
  },
]);

const outputs = ['Claude', 'Codex', 'Gemini', 'OpenCode', 'Cursor'];
const repoFiles = ['plugin.json', 'skills/', 'mcp.json'];

const containerRef = ref<HTMLElement | null>(null);
const activeStep = ref(0);

let observer: IntersectionObserver | null = null;
let intervalId: ReturnType<typeof setInterval> | null = null;

const start = () => {
  if (intervalId) return;
  intervalId = setInterval(() => {
    activeStep.value = (activeStep.value + 1) % steps.value.length;
  }, 1800);
};

const stop = () => {
  if (!intervalId) return;
  clearInterval(intervalId);
  intervalId = null;
};

const visible = ref(false);

watch(visible, (value) => {
  if (value) start();
  else stop();
});

onMounted(() => {
  observer = new IntersectionObserver(
    ([entry]) => {
      visible.value = entry.isIntersecting;
    },
    { threshold: 0.15 },
  );
  if (containerRef.value) observer.observe(containerRef.value);
});

onUnmounted(() => {
  observer?.disconnect();
  stop();
});
</script>

<template>
  <div ref="containerRef" class="hero-demo" role="img" :aria-label="t('hero.preview')">
    <div class="hero-demo__content">
      <div class="hero-demo__steps">
        <div
          v-for="(step, index) in steps"
          :key="step.id"
          class="hero-demo__step"
          :class="{ 'hero-demo__step--active': index === activeStep }"
          :style="{ '--accent': step.accent }"
        >
          <div class="hero-demo__step-index">
            {{ String(index + 1).padStart(2, '0') }}
          </div>
          <div class="hero-demo__step-copy">
            <div class="hero-demo__step-text">
              <span class="hero-demo__step-label">{{ step.label }}</span>
              <span class="hero-demo__step-caption">{{ step.caption }}</span>
            </div>
            <span
              class="hero-demo__step-state"
              :aria-label="index <= activeStep ? t('hero.demo.ready') : t('hero.demo.waiting')"
            >
              <v-icon
                :icon="index <= activeStep ? mdiCheckCircleOutline : mdiClockOutline"
                size="18"
              />
            </span>
          </div>
        </div>
      </div>

      <div class="hero-demo__files">
        <div class="hero-demo__file-card">
          <div class="hero-demo__file-header">
            <span>{{ t('hero.demo.repo') }}</span>
            <span>{{ t('hero.demo.sourceOfTruth') }}</span>
          </div>
          <div class="hero-demo__file-list">
            {{ repoFiles.join('\n') }}
          </div>
        </div>

        <div class="hero-demo__output-card">
          <div class="hero-demo__file-header">
            <span>{{ t('hero.demo.outputs') }}</span>
            <span>{{ t('hero.demo.supportedAgents') }}</span>
          </div>
          <div class="hero-demo__output-list">
            <span
              v-for="output in outputs"
              :key="output"
              class="hero-demo__output hero-demo__output--active"
            >
              {{ output }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hero-demo {
  position: relative;
  border-radius: 18px;
  background: rgba(10, 10, 15, 0.82);
  border: 1px solid rgba(0, 240, 255, 0.14);
  box-shadow:
    0 24px 80px rgba(0, 0, 0, 0.35),
    0 0 60px rgba(0, 240, 255, 0.05);
  overflow: hidden;
}

.hero-demo__content {
  padding: 18px;
}

.hero-demo__steps {
  display: grid;
  gap: 6px;
  margin-bottom: 14px;
}

.hero-demo__step {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0);
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    box-shadow 0.22s ease;
}

.hero-demo__step::after {
  content: '';
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: -3px;
  height: 1px;
  background: rgba(255, 255, 255, 0.06);
}

.hero-demo__step:last-child::after,
.hero-demo__step--active::after {
  display: none;
}

.hero-demo__step--active {
  border-color: color-mix(in srgb, var(--accent) 44%, rgba(255, 255, 255, 0.12));
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--accent) 12%, rgba(255, 255, 255, 0.02)),
    rgba(255, 255, 255, 0.02)
  );
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--accent) 14%, transparent),
    0 0 0 1px color-mix(in srgb, var(--accent) 8%, transparent);
}

.hero-demo__step-index {
  min-width: 30px;
  height: 30px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: color-mix(in srgb, var(--accent) 82%, #ffffff 18%);
  background: color-mix(in srgb, var(--accent) 10%, rgba(255, 255, 255, 0.03));
  border: 1px solid color-mix(in srgb, var(--accent) 28%, rgba(255, 255, 255, 0.08));
  box-shadow: inset 0 0 18px color-mix(in srgb, var(--accent) 10%, transparent);
}

.hero-demo__step-copy {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.hero-demo__step-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.hero-demo__step-label {
  color: #e0e6ff;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.76rem;
  line-height: 1.2;
}

.hero-demo__step-caption {
  color: #7b86a8;
  font-size: 0.62rem;
  line-height: 1.3;
}

.hero-demo__step-state {
  color: #8892b0;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
}

.hero-demo__step--active .hero-demo__step-state {
  color: color-mix(in srgb, var(--accent) 82%, #ffffff 18%);
}

.hero-demo__files {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  gap: 14px;
}

.hero-demo__file-card,
.hero-demo__output-card {
  border-radius: 16px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.hero-demo__file-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  color: #8892b0;
  font-size: 0.62rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.hero-demo__file-list {
  white-space: pre-line;
  color: #dbeafe;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  line-height: 1.7;
}

.hero-demo__output-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.hero-demo__output {
  padding: 7px 9px;
  border-radius: 999px;
  font-size: 0.72rem;
  color: #8892b0;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.hero-demo__output--active {
  color: #0a0a0f;
  background: linear-gradient(135deg, #00f0ff, #39ff14);
  border-color: transparent;
}

.v-theme--light .hero-demo {
  background: rgba(255, 255, 255, 0.92);
  border-color: rgba(8, 145, 178, 0.14);
}

.v-theme--light .hero-demo__title,
.v-theme--light .hero-demo__step-label {
  color: #0f172a;
}

.v-theme--light .hero-demo__file-list {
  color: #164e63;
}

.v-theme--light .hero-demo__step::after {
  background: rgba(15, 23, 42, 0.08);
}

.v-theme--light .hero-demo__step--active {
  border-color: color-mix(in srgb, var(--accent) 38%, rgba(15, 23, 42, 0.12));
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--accent) 10%, rgba(255, 255, 255, 0.86)),
    rgba(255, 255, 255, 0.92)
  );
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--accent) 10%, transparent),
    0 10px 24px rgba(15, 23, 42, 0.05);
}

@media (max-width: 700px) {
  .hero-demo__files {
    grid-template-columns: 1fr;
  }

  .hero-demo__step-copy {
    flex-direction: column;
    gap: 4px;
  }
}
</style>
