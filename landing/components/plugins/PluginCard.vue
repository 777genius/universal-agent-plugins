<script setup lang="ts">
import { computed, resolveComponent } from 'vue';
import { mdiArrowRight, mdiOpenInNew } from '@mdi/js';
import type { PluginCard } from '~/types/content';

const props = withDefaults(
  defineProps<{
    plugin: PluginCard;
    accent?: string;
    supportsLabel: string;
    openLabel: string;
    to?: string;
    href?: string;
    external?: boolean;
    highlightQuery?: string;
  }>(),
  {
    accent: '#00f0ff',
    to: undefined,
    href: undefined,
    external: false,
    highlightQuery: '',
  },
);

const linkTag = computed(() => (props.to ? resolveComponent('NuxtLink') : 'a'));
const { t } = useI18n();
const linkAttrs = computed(() =>
  props.to
    ? { to: props.to }
    : {
        href: props.href ?? props.plugin.href,
        target: '_blank',
        rel: 'noreferrer noopener',
      },
);
const linkIcon = computed(() => (props.external ? mdiOpenInNew : mdiArrowRight));
</script>

<template>
  <component :is="linkTag" class="plugin-card" :style="{ '--accent': accent }" v-bind="linkAttrs">
    <div class="plugin-card__card-top">
      <div class="plugin-card__brand">
        <span
          :class="[
            'plugin-card__logo-wrap',
            `plugin-card__logo-wrap--${plugin.logoSurface ?? 'default'}`,
          ]"
        >
          <img
            :src="plugin.logoSrc"
            :alt="plugin.logoAlt"
            class="plugin-card__logo"
            loading="lazy"
            decoding="async"
          >
        </span>
        <h3 class="plugin-card__card-title">
          <SearchHighlight :text="plugin.title" :query="highlightQuery" />
        </h3>
      </div>
      <div class="plugin-card__meta">
        <span class="plugin-card__type">{{ t(`plugins.types.${plugin.pluginType}`) }}</span>
      </div>
    </div>

    <p class="plugin-card__desc">
      <SearchHighlight :text="plugin.tagline" :query="highlightQuery" />
    </p>

    <p class="plugin-card__subdesc">
      <SearchHighlight :text="plugin.description" :query="highlightQuery" />
    </p>

    <div class="plugin-card__badges-label">{{ supportsLabel }}</div>
    <div class="plugin-card__badges">
      <AgentBadge
        v-for="badge in plugin.badges"
        :key="badge"
        :label="badge"
        :highlight-query="highlightQuery"
        tone="card"
      />
    </div>

    <div class="plugin-card__link">
      <span>{{ openLabel }}</span>
      <v-icon :icon="linkIcon" size="18" />
    </div>
  </component>
</template>

<style scoped>
.plugin-card {
  position: relative;
  height: 100%;
  min-height: 340px;
  border-radius: 22px;
  overflow: hidden;
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--accent) 14%, transparent) 0%,
      transparent 34%
    ),
    rgba(10, 10, 15, 0.84);
  border: 1px solid color-mix(in srgb, var(--accent) 20%, rgba(255, 255, 255, 0.08));
  transition:
    transform 0.35s ease,
    box-shadow 0.35s ease,
    border-color 0.35s ease;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
  padding: 24px;
  display: flex;
  flex-direction: column;
  text-decoration: none;
  cursor: pointer;
}

.plugin-card:hover {
  transform: translateY(-6px);
  border-color: color-mix(in srgb, var(--accent) 42%, transparent);
  box-shadow:
    0 22px 64px color-mix(in srgb, var(--accent) 12%, transparent),
    0 12px 32px rgba(0, 0, 0, 0.35);
}

.plugin-card__card-top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 16px;
}

.plugin-card__meta {
  display: flex;
  align-items: flex-end;
}

.plugin-card__brand {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.plugin-card__logo-wrap {
  width: 54px;
  height: 54px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid color-mix(in srgb, var(--accent) 24%, rgba(255, 255, 255, 0.08));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  flex-shrink: 0;
}

.plugin-card__logo-wrap--light {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(243, 246, 255, 0.96));
  border-color: rgba(255, 255, 255, 0.6);
  box-shadow:
    0 10px 24px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.85);
}

.plugin-card__logo {
  width: 28px;
  height: 28px;
  object-fit: contain;
}

.plugin-card__card-title {
  margin: 0;
  font-size: 1.22rem;
  line-height: 1.15;
  color: #e0e6ff;
}

.plugin-card__type {
  flex-shrink: 0;
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 700;
  background: rgba(0, 240, 255, 0.08);
  color: #7dd3fc;
  border: 1px solid rgba(125, 211, 252, 0.16);
}

.plugin-card__desc {
  margin: 0 0 10px;
  color: #d9e2ff;
  line-height: 1.55;
  font-size: 0.95rem;
  font-weight: 600;
}

.plugin-card__subdesc {
  margin: 0 0 28px;
  color: #8892b0;
  line-height: 1.65;
  font-size: 0.94rem;
}

.plugin-card__badges-label {
  margin-top: auto;
  margin-bottom: 10px;
  color: #8892b0;
  font-size: 0.72rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.plugin-card__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.plugin-card__link {
  margin-top: 18px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--accent);
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.v-theme--light .plugin-card {
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--accent) 10%, transparent) 0%,
      transparent 34%
    ),
    rgba(255, 255, 255, 0.92);
}

.v-theme--light .plugin-card__logo-wrap {
  background: rgba(15, 23, 42, 0.04);
}

.v-theme--light .plugin-card__logo-wrap--light {
  background: #ffffff;
  border-color: rgba(148, 163, 184, 0.3);
  box-shadow:
    0 8px 20px rgba(15, 23, 42, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.v-theme--light .plugin-card__card-title,
.v-theme--light :deep(.agent-badge) {
  color: #0f172a;
}

.v-theme--light .plugin-card__desc,
.v-theme--light .plugin-card__badges-label {
  color: #64748b;
}

@media (max-width: 600px) {
  .plugin-card {
    min-height: 320px;
    padding: 20px;
  }

  .plugin-card__brand {
    gap: 12px;
  }

  .plugin-card__logo-wrap {
    width: 48px;
    height: 48px;
    border-radius: 14px;
  }

  .plugin-card__logo {
    width: 24px;
    height: 24px;
  }

}
</style>
