<!--
  Adapted from plugin-kit-ai landing/components/shared/CommandSnippetCard.vue (MIT).
  See site/NOTICE.md for the complete attribution.
-->
<script setup lang="ts">
type CommandKind = 'terminal' | 'add' | 'update' | 'repair' | 'remove';

const props = withDefaults(
  defineProps<{
    command: string;
    label?: string;
    kind?: CommandKind;
    compact?: boolean;
    inline?: boolean;
  }>(),
  {
    label: 'Terminal',
    kind: 'terminal',
    compact: false,
    inline: false,
  },
);
const copied = ref(false);
const visibleCommand = computed(() => {
  if (!props.compact) return props.command;
  if (props.command.includes(' --target '))
    return props.command.replace(' --target ', '\n--target ');
  return props.command.replace(/^(npx universal-agent-plugins\s+\S+)\s+/, '$1\n');
});
let timer: ReturnType<typeof setTimeout> | undefined;

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer);
});

async function copyCommand() {
  try {
    await navigator.clipboard.writeText(props.command);
  } catch {
    const field = document.createElement('textarea');
    field.value = props.command;
    field.style.position = 'fixed';
    field.style.opacity = '0';
    document.body.appendChild(field);
    field.select();
    document.execCommand('copy');
    field.remove();
  }
  copied.value = true;
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => {
    copied.value = false;
  }, 1600);
}
</script>

<template>
  <div
    class="command-snippet"
    :class="[
      `command-snippet--${kind}`,
      { 'command-snippet--compact': compact, 'command-snippet--inline': inline },
    ]"
  >
    <div class="command-snippet__header">
      <span v-if="!inline" class="command-snippet__label">
        <svg class="command-snippet__label-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <template v-if="kind === 'add'">
            <path d="M12 5v14M5 12h14" />
          </template>
          <template v-else-if="kind === 'update'">
            <path d="M20 7v5h-5M4 17v-5h5" />
            <path d="M18.5 12a6.5 6.5 0 0 0-11.2-4.5L4 12M5.5 12a6.5 6.5 0 0 0 11.2 4.5L20 12" />
          </template>
          <template v-else-if="kind === 'repair'">
            <path
              d="M14.7 6.3a4 4 0 0 0-5 5L4 17l3 3 5.7-5.7a4 4 0 0 0 5-5l-2.5 2.5-3-3 2.5-2.5Z"
            />
            <path d="m5.5 17.5 1 1" />
          </template>
          <template v-else-if="kind === 'remove'">
            <path d="M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13M10 11v5M14 11v5" />
          </template>
          <template v-else>
            <path d="m5 7 4 5-4 5M11 17h8" />
          </template>
        </svg>
        {{ label }}
      </span>
      <button
        type="button"
        :aria-label="copied ? 'Command copied' : 'Copy command'"
        @click="copyCommand"
      >
        {{ copied ? 'Copied' : 'Copy' }}
      </button>
    </div>
    <pre><code>{{ visibleCommand }}</code></pre>
    <span class="sr-only" role="status" aria-live="polite">{{
      copied ? 'Command copied to clipboard' : ''
    }}</span>
  </div>
</template>
