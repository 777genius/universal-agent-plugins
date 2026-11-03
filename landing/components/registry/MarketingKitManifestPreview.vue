<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { marketingKitUserManifest } from '~/data/marketingKitDemo';
import { tomlExampleTokens } from '~/utils/tomlExampleTokens';

const props = withDefaults(defineProps<{ autoCollapse?: boolean }>(), { autoCollapse: true });
const { t } = useI18n();
const tokens = tomlExampleTokens(marketingKitUserManifest);
const root = ref<HTMLElement | null>(null);
const trigger = ref<HTMLButtonElement | null>(null);
const open = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
let observer: IntersectionObserver | undefined;
let introStarted = false;

function show() {
  if (timer) clearTimeout(timer);
  open.value = true;
}
function collapse() {
  if (timer) clearTimeout(timer);
  open.value = false;
}
// Hover may already have opened the file before the click fires. Click always
// opens it; Escape, outside click, mouse leave and the close button dismiss.
function toggle() { show(); }
function closeAndFocus() {
  collapse();
  trigger.value?.focus();
}
function onEscape(event: KeyboardEvent) {
  if (!open.value) return;
  event.stopPropagation();
  event.preventDefault();
  closeAndFocus();
}
function outside(event: PointerEvent) {
  if (root.value && !root.value.contains(event.target as Node)) collapse();
}
function onFocusOut(event: FocusEvent) {
  if (!root.value?.contains(event.relatedTarget as Node | null)) collapse();
}
function beginIntro() {
  if (introStarted) return;
  introStarted = true;
  observer?.disconnect();
  if (!props.autoCollapse) {
    open.value = true;
    return;
  }
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  if (root.value?.matches(':hover') || root.value?.contains(document.activeElement)) return;
  open.value = true;
  timer = setTimeout(() => {
    if (!root.value?.matches(':hover') && !root.value?.contains(document.activeElement)) open.value = false;
  }, 3200);
}
onMounted(() => {
  if ('IntersectionObserver' in window) {
    observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) beginIntro();
    }, { threshold: 0.15, rootMargin: '0px 0px 40px 0px' });
    if (root.value) observer.observe(root.value);
  } else {
    beginIntro();
  }
  document.addEventListener('pointerdown', outside);
});
onBeforeUnmount(() => {
  observer?.disconnect();
  if (timer) clearTimeout(timer);
  document.removeEventListener('pointerdown', outside);
});
</script>

<template>
  <div
    ref="root" class="marketing-manifest" :class="{ 'marketing-manifest--open': open }"
    @mouseenter="show" @mouseleave="autoCollapse && collapse()" @focusout="onFocusOut" @keydown.esc="onEscape"
  >
    <button ref="trigger" type="button" class="marketing-manifest__file" :aria-expanded="open"
      aria-controls="marketing-kit-manifest-preview" @click="toggle">
      <svg viewBox="0 0 24 28" fill="none" aria-hidden="true">
        <path d="M14 2H5a2 2 0 0 0-2 2v20a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V9Z" />
        <path d="M14 2v7h7" /><path d="M7 16h10M7 20h7" />
      </svg>
      <span>uap.toml</span>
      <span class="sr-only"> — {{ t('shell.restore.manifestPreview') }}</span>
    </button>
    <Transition name="marketing-manifest-morph">
      <div v-if="open" id="marketing-kit-manifest-preview" class="marketing-manifest__popover"
        role="region" :aria-label="t('shell.restore.manifestPreview')">
        <div class="marketing-manifest__heading">
          <strong>{{ t('shell.restore.manifestPreview') }}</strong>
          <button type="button" :aria-label="t('shell.restore.closePreview')" @click="closeAndFocus">×</button>
        </div>
        <pre dir="ltr"><code><span v-for="(token, index) in tokens" :key="index" :class="`toml-token--${token.kind}`">{{ token.text }}</span></code></pre>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.marketing-manifest { position: relative; display: inline-grid; justify-items: center; z-index: 3; min-width: 0; }
.marketing-manifest:focus-within { z-index: 9; }
.marketing-manifest__file {
  display: grid; justify-items: center; gap: 7px; padding: 6px 9px; color: var(--text);
  border: 1px solid transparent; border-radius: 10px; background: transparent; font: inherit;
  cursor: pointer; font-size: clamp(11px, 1vw, 16px);
}
.marketing-manifest__file:focus-visible { outline: 2px solid var(--cyan); outline-offset: 2px; }
.marketing-manifest__file svg {
  width: clamp(25px, 2.7vw, 42px); height: clamp(30px, 3.3vw, 50px); stroke: var(--primary-bright);
  stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round;
}
.marketing-manifest__popover {
  position: absolute; z-index: 5; bottom: calc(100% + 8px); left: 50%;
  width: min(355px, calc(100vw - 42px)); max-height: min(420px, 70vh); overflow: auto;
  transform: translateX(-50%); transform-origin: 50% 100%;
  border: 1px solid color-mix(in srgb, var(--primary-bright) 65%, transparent);
  border-radius: 13px; background: #0b1327; color: #dfe5ff;
  box-shadow: 0 15px 45px #030511c9, 0 0 28px #cb5be325; text-align: left; direction: ltr;
}
.marketing-manifest__heading {
  display: flex; justify-content: space-between; align-items: center; gap: 8px;
  padding: 9px 13px; border-bottom: 1px solid #aaa8ce33; font-size: 12px;
}
.marketing-manifest__heading button {
  background: transparent; color: #dfe5ff; border: 0; border-radius: 6px; cursor: pointer;
  font-size: 19px; min-width: 28px; min-height: 28px;
}
.marketing-manifest__heading button:focus-visible { outline: 2px solid #00f0ff; }
.marketing-manifest__popover pre {
  overflow: auto; margin: 0; padding: 11px 13px 14px; font-family: 'JetBrains Mono', monospace;
  font-size: clamp(9px, 1.15vw, 11px); line-height: 1.65; white-space: pre; tab-size: 2;
}
.toml-token--plain { color: #dfe5ff; }
.toml-token--section { color: #d7afff; }
.toml-token--key { color: #5cdcff; }
.toml-token--string { color: #a4ec83; }
.toml-token--number { color: #ffe878; }
.toml-token--comment { color: #9babc5; }
.marketing-manifest-morph-enter-active, .marketing-manifest-morph-leave-active {
  transition: opacity .42s ease, transform .42s cubic-bezier(.2,.85,.25,1);
}
.marketing-manifest-morph-enter-from, .marketing-manifest-morph-leave-to {
  opacity: 0; transform: translateX(-50%) translateY(12px) scale(.18);
}
@media (prefers-reduced-motion: reduce) {
  .marketing-manifest-morph-enter-active, .marketing-manifest-morph-leave-active { transition: none; }
}
</style>
