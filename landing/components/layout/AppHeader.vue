<script setup lang="ts">
import { mdiClose, mdiGithub, mdiMenu } from '@mdi/js';
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
} from 'reka-ui';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const localePath = useLocalePath();
const config = useRuntimeConfig();
const menuOpen = ref(false);
const interactiveReady = ref(false);
const githubUrl = `https://github.com/${config.public.githubRepo}`;
const homePath = computed(() => localePath('/'));
const homeHref = computed(() => router.resolve(homePath.value).href);

const navItems = [
  { id: 'plugins', label: 'Plugins' },
  { id: 'why', label: 'Why it works' },
  { id: 'faq', label: 'FAQ' },
];

const normalizePath = (value: string) => (value !== '/' ? value.replace(/\/+$/, '') : '/');

const isHomePage = computed(() => normalizePath(route.path) === normalizePath(homePath.value));

const sectionHref = (sectionId: string) =>
  isHomePage.value ? `#${sectionId}` : `${homeHref.value}#${sectionId}`;

onMounted(() => {
  interactiveReady.value = true;
});
</script>

<template>
  <header class="app-header">
    <v-container class="app-header__inner">
      <AppLogo />
      <nav class="app-header__nav">
        <v-btn v-for="item in navItems" :key="item.id" variant="text" :href="sectionHref(item.id)">
          {{ item.label }}
        </v-btn>
      </nav>
      <div class="app-header__spacer" />
      <div class="app-header__desktop-actions">
        <v-btn
          variant="outlined"
          size="small"
          :href="githubUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="app-header__github-btn"
          :prepend-icon="mdiGithub"
        >
          {{ t('nav.viewOnGithub') }}
        </v-btn>
        <template v-if="interactiveReady">
          <ThemeToggle />
        </template>
        <div v-else class="app-header__control-fallback" aria-hidden="true" />
      </div>
      <div class="app-header__mobile-actions">
        <DialogRoot v-model:open="menuOpen">
          <DialogTrigger as-child>
            <v-btn :icon="mdiMenu" variant="text" aria-label="Open navigation menu" />
          </DialogTrigger>
          <DialogPortal>
            <DialogOverlay class="mobile-menu-overlay" />
            <DialogContent class="mobile-menu">
              <DialogTitle class="sr-only">Navigation menu</DialogTitle>
              <DialogDescription class="sr-only">
                Jump to plugins, product details, frequently asked questions, or GitHub.
              </DialogDescription>
              <div class="mobile-menu__header">
                <div @click="menuOpen = false">
                  <AppLogo />
                </div>
                <div style="flex: 1" />
                <DialogClose as-child>
                  <v-btn :icon="mdiClose" variant="text" aria-label="Close navigation menu" />
                </DialogClose>
              </div>
              <hr class="mobile-menu__divider" >
              <nav class="mobile-menu__list">
                <a
                  v-for="item in navItems"
                  :key="item.id"
                  :href="sectionHref(item.id)"
                  class="mobile-menu__link"
                  @click="menuOpen = false"
                >
                  {{ item.label }}
                </a>
                <a
                  :href="githubUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="mobile-menu__link"
                  @click="menuOpen = false"
                >
                  {{ t('nav.viewOnGithub') }}
                </a>
              </nav>
              <hr class="mobile-menu__divider" >
              <div class="mobile-menu__actions">
                <span>Appearance</span>
                <template v-if="interactiveReady">
                  <ThemeToggle />
                </template>
                <template v-else>
                  <div class="app-header__control-fallback" aria-hidden="true" />
                </template>
              </div>
            </DialogContent>
          </DialogPortal>
        </DialogRoot>
      </div>
    </v-container>
  </header>
</template>

<style scoped>
.app-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  height: 64px;
  display: flex;
  align-items: center;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(0, 240, 255, 0.08);
}

.v-theme--light .app-header {
  background: rgba(255, 255, 255, 0.9);
  border-bottom-color: rgba(0, 0, 0, 0.06);
}

.v-theme--dark .app-header {
  background: rgba(10, 10, 15, 0.9);
}

.app-header__inner {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
}

.app-header__nav {
  display: flex;
  align-self: stretch;
  align-items: stretch;
  margin-left: 48px;
}

.app-header__nav :deep(.v-btn) {
  height: 100% !important;
  border-radius: 0;
}

.app-header__spacer {
  flex: 1;
}

.app-header__desktop-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.app-header__control-fallback {
  width: 40px;
  height: 36px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.v-theme--light .app-header__control-fallback {
  background: rgba(15, 23, 42, 0.04);
  border-color: rgba(15, 23, 42, 0.08);
}

.app-header__github-btn {
  border-color: rgba(0, 240, 255, 0.25) !important;
  color: #00f0ff !important;
  font-weight: 600 !important;
  font-size: 12px !important;
  letter-spacing: 0.02em !important;
}

.app-header__github-btn:hover {
  border-color: rgba(0, 240, 255, 0.5) !important;
  background: rgba(0, 240, 255, 0.06) !important;
}

.app-header__mobile-actions {
  display: none;
}

@media (max-width: 959px) {
  .app-header__nav {
    display: none;
  }

  .app-header__desktop-actions {
    display: none;
  }

  .app-header__mobile-actions {
    display: flex;
  }
}

.mobile-menu-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: color-mix(in srgb, var(--surface-solid) 94%, transparent);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
}

.mobile-menu {
  position: fixed;
  inset: 0;
  z-index: 10000;
  padding: 16px 16px 24px;
  height: 100%;
  overflow-y: auto;
  color: var(--text);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--cyan) 3%, transparent), transparent 32%),
    var(--surface-solid);
}

.mobile-menu__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 12px;
}

.mobile-menu__divider {
  border: none;
  border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.mobile-menu__list {
  display: flex;
  flex-direction: column;
  padding: 8px 0;
}

.mobile-menu__link {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  font-size: 1rem;
  color: rgb(var(--v-theme-on-surface));
  text-decoration: none;
  border-radius: 8px;
  transition: background-color 0.15s;
}

.mobile-menu__link:hover {
  background: rgba(var(--v-theme-on-surface), 0.06);
}

.mobile-menu__actions {
  display: flex;
  flex-direction: row;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px 0;
  color: var(--muted);
  font-size: 0.9rem;
}

.mobile-menu-overlay[data-state='open'] {
  animation: mobile-menu-overlay-in 0.18s ease-out;
}

.mobile-menu[data-state='open'] {
  animation: mobile-menu-content-in 0.2s ease-out;
}

@keyframes mobile-menu-overlay-in {
  from {
    opacity: 0;
  }
}

@keyframes mobile-menu-content-in {
  from {
    opacity: 0;
    transform: translateY(-8px);
  }
}
</style>
