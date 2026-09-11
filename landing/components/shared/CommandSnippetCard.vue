<script setup lang="ts">
import { renderHighlightedCommand } from '~/utils/commandHighlight';

const { t } = useI18n();

const props = withDefaults(
  defineProps<{
    label: string;
    command: string;
    copyLabel?: string;
    copiedLabel?: string;
    accent?: string;
  }>(),
  {
    accent: '#00f0ff',
    copyLabel: undefined,
    copiedLabel: undefined,
  },
);

const copied = ref(false);

let copiedTimer: ReturnType<typeof setTimeout> | null = null;

onBeforeUnmount(() => {
  if (copiedTimer) {
    clearTimeout(copiedTimer);
  }
});

const fallbackCopy = async (text: string) => {
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'absolute';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
};

const copyCommand = async () => {
  if (!import.meta.client) {
    return;
  }

  const text = props.command.trim();

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      await fallbackCopy(text);
    }
  } catch {
    await fallbackCopy(text);
  }

  copied.value = true;
  if (copiedTimer) {
    clearTimeout(copiedTimer);
  }
  copiedTimer = setTimeout(() => {
    copied.value = false;
  }, 1800);
};

const commandLines = computed(() => renderHighlightedCommand(props.command));
const copyStateLabel = computed(() =>
  copied.value ? (props.copiedLabel ?? t('download.copied')) : (props.copyLabel ?? t('download.copy')),
);
</script>

<template>
  <div class="command-snippet" :style="{ '--snippet-accent': accent }">
    <div class="command-snippet__head">
      <span class="command-snippet__label">{{ label }}</span>
      <button
        type="button"
        class="command-snippet__copy-btn"
        :aria-label="copyStateLabel"
        @click="copyCommand"
      >
        {{ copyStateLabel }}
      </button>
    </div>

    <pre class="command-snippet__body"><code><template
      v-for="(line, lineIndex) in commandLines"
      :key="`line-${lineIndex}`"
    ><span class="command-snippet__line"><span
      v-for="(token, tokenIndex) in line"
      :key="`line-${lineIndex}-token-${tokenIndex}`"
      class="command-snippet__token"
      :class="`command-snippet__token--${token.className}`"
    >{{ token.text }}</span></span></template></code></pre>
  </div>
</template>

<style scoped>
.command-snippet {
  display: grid;
  gap: 8px;
}

.command-snippet__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.command-snippet__label {
  color: #8892b0;
  font-size: 0.7rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: 'JetBrains Mono', monospace;
}

.command-snippet__copy-btn {
  appearance: none;
  border: 1px solid color-mix(in srgb, var(--snippet-accent) 20%, rgba(255, 255, 255, 0.12));
  background: color-mix(in srgb, var(--snippet-accent) 8%, rgba(255, 255, 255, 0.03));
  color: var(--snippet-accent);
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 0.68rem;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.command-snippet__copy-btn:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--snippet-accent) 36%, rgba(255, 255, 255, 0.12));
  background: color-mix(in srgb, var(--snippet-accent) 14%, rgba(255, 255, 255, 0.03));
}

.command-snippet__copy-btn:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--snippet-accent) 42%, transparent);
  outline-offset: 2px;
}

.command-snippet__body {
  display: block;
  margin: 0;
  padding: 14px 16px;
  border-radius: 14px;
  background:
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--snippet-accent) 4%, transparent),
      rgba(0, 0, 0, 0.2)
    ),
    rgba(0, 0, 0, 0.26);
  border: 1px solid rgba(255, 255, 255, 0.07);
  font-size: 0.8rem;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-x: auto;
  font-family: 'JetBrains Mono', monospace;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
}

.command-snippet__line {
  display: block;
}

.command-snippet__token {
  color: #dbeafe;
}

.command-snippet__token--command {
  color: #67e8f9;
}

.command-snippet__token--action {
  color: #f0abfc;
}

.command-snippet__token--flag {
  color: #facc15;
}

.command-snippet__token--path {
  color: #86efac;
}

.command-snippet__token--url {
  color: #38bdf8;
}

.command-snippet__token--operator {
  color: #c084fc;
}

.v-theme--light .command-snippet__label {
  color: #475569;
}

.v-theme--light .command-snippet__body {
  background: rgba(241, 245, 249, 0.92);
  border-color: rgba(15, 23, 42, 0.06);
}

.v-theme--light .command-snippet__token {
  color: #1e293b;
}

.v-theme--light .command-snippet__token--command {
  color: #0891b2;
}

.v-theme--light .command-snippet__token--action {
  color: #c026d3;
}

.v-theme--light .command-snippet__token--flag {
  color: #ca8a04;
}

.v-theme--light .command-snippet__token--path {
  color: #15803d;
}

.v-theme--light .command-snippet__token--url {
  color: #2563eb;
}

.v-theme--light .command-snippet__token--operator {
  color: #7c3aed;
}
</style>
