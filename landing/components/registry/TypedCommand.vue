<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

const props = withDefaults(defineProps<{
  lines: readonly string[];
  delay?: number;
  millisecondsPerCharacter?: number;
}>(), { delay: 150, millisecondsPerCharacter: 39 });

const host = ref<HTMLElement | null>(null);
const started = ref(false);
const complete = ref(false);
const reducedMotion = ref(false);
let observer: IntersectionObserver | undefined;
let completionTimer: ReturnType<typeof setTimeout> | undefined;

const sequence = computed(() => {
  let offset = props.delay;
  return props.lines.map((text) => {
    const line = { text, delay: offset, duration: Math.max(180, text.length * props.millisecondsPerCharacter) };
    offset += line.duration + 120;
    return line;
  });
});
const totalDuration = computed(() =>
  sequence.value.reduce((total, line) => Math.max(total, line.delay + line.duration), 0),
);
function begin() {
  if (started.value) return;
  started.value = true;
  observer?.disconnect();
  completionTimer = setTimeout(() => { complete.value = true; }, totalDuration.value + 120);
}
onMounted(() => {
  reducedMotion.value = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reducedMotion.value || !('IntersectionObserver' in window)) {
    complete.value = true;
    return;
  }
  observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) begin();
  }, { rootMargin: '0px 0px 80px 0px', threshold: 0.15 });
  if (host.value) observer.observe(host.value);
});
onBeforeUnmount(() => {
  observer?.disconnect();
  if (completionTimer) clearTimeout(completionTimer);
});
</script>

<template>
  <span
    ref="host"
    class="typed-command"
    :class="{ 'typed-command--started': started && !reducedMotion }"
    :data-complete="complete || reducedMotion ? 'true' : 'false'"
  >
    <span
      v-for="(line, index) in sequence"
      :key="index"
      class="typed-command__line"
      :style="{
        '--characters': String(line.text.length),
        '--duration': `${line.duration}ms`,
        '--delay': `${line.delay}ms`,
      }"
    >{{ line.text }}</span>
  </span>
</template>

<style scoped>
.typed-command {
  display: block;
  min-width: 0;
  direction: ltr;
  text-align: left;
  font: inherit;
}
.typed-command__line {
  display: block;
  position: relative;
  width: max-content;
  max-width: 100%;
  min-height: 1.4em;
  overflow: hidden;
  white-space: pre;
  font: inherit;
}
.typed-command--started .typed-command__line {
  width: 0;
  border-inline-end: 1px solid transparent;
  animation: typed-reveal var(--duration) steps(var(--characters), end) var(--delay) forwards;
}
.typed-command--started .typed-command__line::after {
  content: '';
  position: absolute;
  inset-inline-end: 0;
  top: .15em;
  width: 1px;
  height: 1.05em;
  background: #00f0ff;
  animation: typed-caret .65s steps(1, end) infinite;
}
@keyframes typed-reveal {
  to { width: calc(var(--characters) * 1ch); }
}
@keyframes typed-caret {
  50% { opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .typed-command--started .typed-command__line {
    width: max-content;
    animation: none;
    border: 0;
  }
  .typed-command--started .typed-command__line::after { display: none; }
}
</style>
