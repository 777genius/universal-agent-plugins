<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

const props = withDefaults(defineProps<{
  lines: readonly string[];
  delay?: number;
  millisecondsPerCharacter?: number;
}>(), { delay: 150, millisecondsPerCharacter: 39 });

// Fragments are visual line-breaking hints, not separate shell commands.
// The terminal displays one executable command; the invisible full string
// reserves its final line-height so typing never shifts nearby elements.
const command = computed(() => props.lines.join(' '));
const characters = ref(0);
const started = ref(false);
const complete = ref(false);
const reducedMotion = ref(false);
const visibleCommand = computed(() =>
  !started.value || reducedMotion.value ? command.value : command.value.slice(0, characters.value),
);
const host = ref<HTMLElement | null>(null);
let observer: IntersectionObserver | undefined;
let delayTimer: ReturnType<typeof setTimeout> | undefined;
let typingTimer: ReturnType<typeof setInterval> | undefined;

function finish() {
  characters.value = command.value.length;
  complete.value = true;
  if (typingTimer) {
    clearInterval(typingTimer);
    typingTimer = undefined;
  }
}
function begin() {
  if (started.value) return;
  started.value = true;
  observer?.disconnect();
  delayTimer = setTimeout(() => {
    if (!command.value.length) {
      finish();
      return;
    }
    typingTimer = setInterval(() => {
      characters.value += 1;
      if (characters.value >= command.value.length) finish();
    }, Math.max(18, props.millisecondsPerCharacter));
  }, props.delay);
}
onMounted(() => {
  reducedMotion.value = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reducedMotion.value || !('IntersectionObserver' in window)) {
    finish();
    return;
  }
  observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) begin();
  }, { rootMargin: '0px 0px 80px 0px', threshold: 0.15 });
  if (host.value) observer.observe(host.value);
});
onBeforeUnmount(() => {
  observer?.disconnect();
  if (delayTimer) clearTimeout(delayTimer);
  if (typingTimer) clearInterval(typingTimer);
});
</script>

<template>
  <span
    ref="host"
    class="typed-command"
    :data-complete="complete ? 'true' : 'false'"
    :data-command="command"
  >
    <span class="typed-command__measure" aria-hidden="true">{{ command }}</span>
    <span class="typed-command__visible" aria-hidden="true">
      {{ visibleCommand }}<span
        v-if="started && !complete && !reducedMotion"
        class="typed-command__caret"
      />
    </span>
    <span class="sr-only">{{ command }}</span>
  </span>
</template>

<style scoped>
.typed-command {
  display: block;
  position: relative;
  min-width: 0;
  direction: ltr;
  text-align: left;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: inherit;
}
.typed-command__measure {
  display: block;
  visibility: hidden;
  font: inherit;
}
.typed-command__visible {
  display: block;
  position: absolute;
  inset: 0;
  white-space: inherit;
  overflow-wrap: inherit;
  font: inherit;
}
.typed-command__caret {
  display: inline-block;
  height: 1.05em;
  border-inline-end: 2px solid #00f0ff;
  vertical-align: -.13em;
  animation: typed-caret-blink .65s steps(1, end) infinite;
}
@keyframes typed-caret-blink {
  50% { opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .typed-command__caret { display: none; animation: none; }
}
</style>
