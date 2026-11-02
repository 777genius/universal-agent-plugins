<script setup lang="ts">
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverAnchor } from 'reka-ui';
import type { AuthenticationIndicator } from '~/utils/authentication';

const props = defineProps<{ indicator: AuthenticationIndicator }>();
const instanceId = useId();
const contentId = ref<string>();
const trigger = ref<HTMLButtonElement>();
const open = ref(false);
const pinned = ref(false);
const pointerOpening = ref(true);
let leaveTimer: ReturnType<typeof setTimeout> | undefined;
const icon = computed(() =>
  props.indicator.varies
    ? 'varies'
    : props.indicator.state === 'required'
      ? 'lock'
      : props.indicator.state === 'not_required'
        ? 'lock-open'
        : 'help',
);
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
        :data-varies="indicator.varies"
        :data-icon="icon"
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
          <template v-if="icon === 'lock'">
            <rect x="5" y="10.5" width="14" height="9.5" rx="2" />
            <path d="M8 10.5V7a4 4 0 0 1 8 0v3.5" />
            <circle cx="12" cy="13.4" r="0.9" />
            <path d="M12 14.3v2.3" />
          </template>
          <template v-else-if="icon === 'lock-open'">
            <rect x="4" y="10" width="16" height="11" rx="2" />
            <path d="M8 10V6a4 4 0 0 1 7.5-2M12 15v2" />
          </template>
          <template v-else-if="icon === 'varies'">
            <rect x="5" y="10.5" width="14" height="9.5" rx="2" />
            <path d="M8 10.5V7a4 4 0 0 1 8 0v3.5" />
            <path d="M8.5 15.8c.9-1.4 1.9-1.4 2.9 0 .9 1.3 1.9 1.3 2.9 0" />
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
          <p v-if="indicator.release" class="authentication-indicator__release">
            {{ indicator.release }}
          </p>
          <ul v-if="indicator.groups.length" class="authentication-indicator__groups">
            <li v-for="group in indicator.groups" :key="group.authentication">
              <svg
                aria-hidden="true"
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <template v-if="group.authentication === 'required'">
                  <rect x="5" y="10.5" width="14" height="9.5" rx="2" />
                  <path d="M8 10.5V7a4 4 0 0 1 8 0v3.5" />
                  <circle cx="12" cy="13.4" r="0.9" />
                  <path d="M12 14.3v2.3" />
                </template>
                <template v-else-if="group.authentication === 'not_required'">
                  <rect x="4" y="10" width="16" height="11" rx="2" />
                  <path d="M8 10V6a4 4 0 0 1 7.5-2M12 15v2" />
                </template>
                <template v-else>
                  <circle cx="12" cy="12" r="9" />
                  <path d="M9.5 8.5a2.5 2.5 0 1 1 4 2c-1 .7-1.5 1-1.5 2.5M12 17h.01" />
                </template>
              </svg>
              <span class="authentication-indicator__policy">{{ group.label }}</span>
              <span class="authentication-indicator__clients">{{ group.clients.join(', ') }}</span>
            </li>
          </ul>
          <p class="authentication-indicator__summary">{{ indicator.summary }}</p>
          <p v-if="indicator.note" class="authentication-indicator__note">{{ indicator.note }}</p>
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
.authentication-indicator__release {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 0.7rem;
}
.authentication-indicator__groups {
  display: grid;
  gap: 12px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}
.authentication-indicator__groups li {
  display: grid;
  grid-template-columns: 14px minmax(0, 1fr);
  align-items: start;
  gap: 2px 8px;
}
.authentication-indicator__groups svg {
  margin-top: 2px;
  color: var(--muted);
}
.authentication-indicator__policy {
  min-width: 0;
  font-weight: 600;
}
.authentication-indicator__clients {
  grid-column: 2;
  min-width: 0;
  color: var(--muted);
}
.authentication-indicator__summary,
.authentication-indicator__note {
  margin: 8px 0 0;
  color: var(--muted);
}
.authentication-indicator__note {
  margin-top: 4px;
  font-size: 0.7rem;
}
</style>
