<script setup lang="ts">
import { mdiOpenSourceInitiative, mdiRobotOutline, mdiViewDashboardOutline } from "@mdi/js";

const { content } = useLandingContent();
const { t, locale } = useI18n();
const config = useRuntimeConfig();
const { asset } = useSite();
const githubUrl = `https://github.com/${config.public.githubRepo}`;
const { docsUrl } = useDocsLinks();
const { data: releaseData } = useReleaseDownloads();

const releaseVersion = computed(() => releaseData.value?.version || null);
const releaseDate = computed(() => {
  const raw = releaseData.value?.pubDate;
  if (!raw) return null;
  return formatReleaseDate(raw, locale.value);
});
</script>

<template>
  <section id="hero" class="hero-section section anchor-offset">
    <v-container class="hero-section__container">
      <v-row align="start" justify="space-between">
        <v-col cols="12" md="6" class="hero-section__content">
          <h1 class="hero-section__title">
            <img
              :src="asset('icon.svg')"
              alt=""
              class="hero-section__logo"
              width="56"
              height="56"
            >
            {{ content.hero.title }}
          </h1>

          <p class="hero-section__subtitle">
            {{ content.hero.subtitle }}
          </p>

          <div class="hero-section__actions">
            <v-btn
              variant="flat"
              size="large"
              :href="githubUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="hero-section__btn-primary"
            >
              {{ t("hero.primaryCta") }}
            </v-btn>
            <v-btn
              variant="outlined"
              size="large"
              href="#plugins"
              class="hero-section__btn-secondary"
            >
              {{ t("hero.secondaryCta") }}
            </v-btn>
            <v-btn
              variant="tonal"
              size="large"
              :href="docsUrl"
              class="hero-section__btn-tertiary"
            >
              {{ t("hero.docsCta") }}
            </v-btn>
          </div>

          <div class="hero-section__meta-row">
            <div v-if="releaseVersion" class="hero-section__release-badge">
              {{ t("hero.latestRelease") }} · v{{ releaseVersion }}<template v-if="releaseDate"> · {{ releaseDate }}</template>
            </div>
          </div>

          <div class="hero-section__trust">
            <div class="hero-section__trust-item">
              <v-icon size="16" class="hero-section__trust-icon" :icon="mdiRobotOutline" />
              <span>{{ t("hero.trust.oneRepo") }}</span>
            </div>
            <div class="hero-section__trust-divider" />
            <div class="hero-section__trust-item">
              <v-icon size="16" class="hero-section__trust-icon" :icon="mdiViewDashboardOutline" />
              <span>{{ t("hero.trust.validated") }}</span>
            </div>
            <div class="hero-section__trust-divider" />
            <div class="hero-section__trust-item">
              <v-icon size="16" class="hero-section__trust-icon" :icon="mdiOpenSourceInitiative" />
              <span>{{ t("hero.trust.openSource") }}</span>
            </div>
          </div>
        </v-col>

        <v-col cols="12" md="5" class="hero-section__demo-col">
          <div class="hero-section__preview">
            <div class="hero-section__preview-glow" />
            <LazyHeroDemo />
          </div>
        </v-col>
      </v-row>
    </v-container>
  </section>
</template>

<style scoped>
.hero-section {
  position: relative;
  min-height: 85vh;
  display: flex;
  align-items: center;
}

.hero-section__container {
  position: relative;
  z-index: 1;
}

.hero-section__content {
  animation: heroFadeIn 0.8s ease both;
  align-self: center;
}

.hero-section__title {
  font-size: 3rem;
  font-weight: 800;
  letter-spacing: -0.04em;
  line-height: 1.1;
  margin-bottom: 20px;
  background: linear-gradient(135deg, #e0e6ff 0%, #00f0ff 50%, #ff00ff 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: heroFadeIn 0.8s ease both;
  animation-delay: 0.2s;
  display: flex;
  align-items: center;
  gap: 16px;
  white-space: nowrap;
}

.hero-section__logo {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  flex-shrink: 0;
  display: block;
  box-shadow: 0 10px 30px rgba(0, 240, 255, 0.2);
  object-fit: cover;
}

.hero-section__subtitle {
  font-size: 1.2rem;
  line-height: 1.7;
  color: #8892b0;
  opacity: 0.9;
  max-width: 560px;
  margin-bottom: 24px;
  animation: heroFadeIn 0.8s ease both;
  animation-delay: 0.3s;
}

.hero-section__actions {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 16px;
  animation: heroFadeIn 0.8s ease both;
  animation-delay: 0.4s;
}

.hero-section__meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  margin-bottom: 24px;
}

.hero-section__release-badge {
  font-size: 0.78rem;
  font-weight: 500;
  color: #8892b0;
  font-family: "JetBrains Mono", monospace;
  animation: heroFadeIn 0.8s ease both;
  animation-delay: 0.45s;
}

.hero-section__btn-primary {
  background: linear-gradient(135deg, #00f0ff, #ff00ff) !important;
  color: #0a0a0f !important;
  font-weight: 700 !important;
  letter-spacing: 0.02em !important;
  box-shadow: 0 4px 20px rgba(0, 240, 255, 0.3) !important;
  transition: all 0.3s ease !important;
}

.hero-section__btn-primary:hover {
  box-shadow: 0 6px 30px rgba(0, 240, 255, 0.5) !important;
  transform: translateY(-1px) !important;
}

.hero-section__btn-secondary {
  border-color: rgba(0, 240, 255, 0.3) !important;
  color: #00f0ff !important;
  font-weight: 600 !important;
  transition: all 0.3s ease !important;
}

.hero-section__btn-secondary:hover {
  border-color: rgba(0, 240, 255, 0.5) !important;
  background: rgba(0, 240, 255, 0.06) !important;
}

.hero-section__btn-tertiary {
  background: rgba(255, 255, 255, 0.04) !important;
  color: #d9e2ff !important;
  font-weight: 600 !important;
  border: 1px solid rgba(217, 226, 255, 0.12) !important;
  transition: all 0.3s ease !important;
}

.hero-section__btn-tertiary:hover {
  background: rgba(255, 255, 255, 0.08) !important;
  border-color: rgba(217, 226, 255, 0.24) !important;
  transform: translateY(-1px) !important;
}

.hero-section__trust {
  display: flex;
  align-items: center;
  gap: 16px;
  animation: heroFadeIn 0.8s ease both;
  animation-delay: 0.5s;
}

.hero-section__trust-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.82rem;
  font-weight: 500;
  color: #8892b0;
}

.hero-section__trust-icon {
  color: #00f0ff;
  opacity: 0.8;
}

.hero-section__trust-divider {
  width: 1px;
  height: 16px;
  background: rgba(0, 240, 255, 0.2);
}

.hero-section__preview {
  position: relative;
  width: 100%;
  animation: heroSlideUp 0.9s ease both;
  animation-delay: 0.3s;
  align-self: flex-start;
}

.hero-section__preview-glow {
  position: absolute;
  inset: -20px;
  background: radial-gradient(circle, rgba(0, 240, 255, 0.16), transparent 65%);
  filter: blur(30px);
  pointer-events: none;
}

.hero-demo-fallback {
  aspect-ratio: 16 / 10;
  border-radius: 18px;
  background: rgba(10, 10, 15, 0.75);
  border: 1px solid rgba(0, 240, 255, 0.12);
}

@keyframes heroFadeIn {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes heroSlideUp {
  from {
    opacity: 0;
    transform: translateY(28px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.v-theme--light .hero-section__title {
  background: linear-gradient(135deg, #1e293b 0%, #0891b2 55%, #7c3aed 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.v-theme--light .hero-section__subtitle,
.v-theme--light .hero-section__trust-item,
.v-theme--light .hero-section__release-badge {
  color: #64748b;
}

.v-theme--light .hero-section__path-card {
  background: rgba(255, 255, 255, 0.86);
  border-color: rgba(14, 165, 233, 0.14);
}

.v-theme--light .hero-section__path-title {
  color: #0f172a;
}

.v-theme--light .hero-section__path-copy {
  color: #475569;
}

.v-theme--light .hero-section__path-command {
  background: rgba(241, 245, 249, 0.92);
  color: #0369a1;
}

@media (max-width: 960px) {
  .hero-section {
    min-height: auto;
    padding-top: 28px;
  }

  .hero-section__content {
    align-self: flex-start;
  }

  .hero-section__title {
    font-size: 2.4rem;
    white-space: normal;
  }

  .hero-section__subtitle {
    font-size: 1.05rem;
    margin-bottom: 28px;
  }

  .hero-section__paths {
    grid-template-columns: 1fr;
  }

  .hero-section__trust {
    flex-wrap: wrap;
    gap: 12px;
  }
}

@media (max-width: 600px) {
  .hero-section__title {
    font-size: 2rem;
    gap: 12px;
  }

  .hero-section__logo {
    width: 48px;
    height: 48px;
    font-size: 1.2rem;
  }
}
</style>
