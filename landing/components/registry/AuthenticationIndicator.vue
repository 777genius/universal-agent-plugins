<script setup lang="ts">
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverAnchor } from 'reka-ui';
import type { AuthenticationIndicator } from '~/utils/authentication';

defineProps<{ indicator: AuthenticationIndicator }>();
const instanceId = useId();
const contentId = ref<string>();
const trigger = ref<HTMLButtonElement>();
const open = ref(false);
const pinned = ref(false);
const pointerOpening = ref(true);
let leaveTimer: ReturnType<typeof setTimeout> | undefined;
function enter() {
  if (!open.value) pointerOpening.value = true;
  clearTimeout(leaveTimer);
  open.value = true;
}
function hover(event: PointerEvent) {
  if (event.pointerType === 'mouse') enter();
}
function leave() {
  leaveTimer = setTimeout(() => {
    if (!pinned.value) open.value = false;
  }, 120);
}
function pin(event: MouseEvent) {
  pointerOpening.value = event.detail !== 0;
  clearTimeout(leaveTimer);
  pinned.value = true;
  open.value = true;
  nextTick(() => {
    const content = document.querySelector<HTMLElement>(`[data-auth-content="${instanceId}"]`);
    contentId.value = content?.id;
    if (!pointerOpening.value) content?.focus();
  });
}
function closeFocus(event: Event) {
  event.preventDefault();
  if (!pointerOpening.value) trigger.value?.focus();
}
function autoFocus(event: Event) {
  if (pointerOpening.value) event.preventDefault();
}
watch(open, (value) => {
  if (!value) pinned.value = false;
});
onBeforeUnmount(() => clearTimeout(leaveTimer));
</script>

<template>
  <PopoverRoot v-model:open="open">
    <PopoverAnchor as-child>
      <button
        ref="trigger"
        aria-haspopup="dialog"
        :aria-expanded="open"
        :aria-controls="contentId"
        class="authentication-indicator"
        type="button"
        :aria-label="indicator.label"
        :data-authentication="indicator.state"
        @pointerenter="hover"
        @pointerleave="leave"
        @click="pin"
      >
        <svg
          aria-hidden="true"
          width="17"
          height="17"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <template v-if="indicator.state === 'required'">
            <circle cx="8" cy="8" r="5" />
            <path d="m11.5 11.5 9 9M17 17l3-3M14 14l3-3" />
          </template>
          <template v-else-if="indicator.state === 'not_required'">
            <rect x="4" y="10" width="16" height="11" rx="2" />
            <path d="M8 10V6a4 4 0 0 1 7.5-2M12 15v2" />
          </template>
          <template v-else>
            <circle cx="12" cy="12" r="9" />
            <path d="M9.5 8.5a2.5 2.5 0 1 1 4 2c-1 .7-1.5 1-1.5 2.5M12 17h.01" />
          </template>
        </svg>
        <span class="sr-only">{{ indicator.label }}</span>
      </button>
    </PopoverAnchor>
    <PopoverPortal>
      <PopoverContent
        as-child
        :data-auth-content="instanceId"
        :aria-label="indicator.label"
        class="authentication-indicator__content"
        :side-offset="6"
        :collision-padding="14"
        align="start"
        @open-auto-focus="autoFocus"
        @close-auto-focus="closeFocus"
        @pointerenter="enter"
        @pointerleave="leave"
      >
        <div>
          <strong>{{ indicator.label }}</strong>
          <p>{{ indicator.detail }}</p>
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>

<style scoped lang="scss">
.authentication-indicator {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 38px;
  height: 38px;
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--muted);
  background: transparent;
  cursor: pointer;
}
.authentication-indicator:hover,
.authentication-indicator[aria-expanded='true'] {
  color: var(--primary-bright);
  border-color: var(--primary);
}
.authentication-indicator:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}
.authentication-indicator__content {
  z-index: 130;
  width: min(360px, calc(100vw - 28px));
  padding: 14px;
  border: 1px solid var(--line);
  border-radius: 11px;
  color: var(--text);
  background: var(--surface-solid);
  box-shadow: 0 18px 52px rgba(0, 0, 0, 0.38);
  font-size: 0.76rem;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.authentication-indicator__content p {
  margin: 6px 0 0;
  color: var(--muted);
}
</style>
