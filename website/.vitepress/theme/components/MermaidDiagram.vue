<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { cleanupMermaidZoom, setupMermaidZoom } from "vitepress-mermaid-zoom";

const props = defineProps<{
  chart: string;
}>();

const svg = ref("");
const error = ref("");
const rootId = nextDiagramId();

function nextDiagramId() {
  return `pkai-mermaid-${Math.random().toString(36).slice(2)}`;
}

function surfaceSelector() {
  return `#${rootId} .mermaid-diagram__surface`;
}

async function renderDiagram() {
  if (typeof window === "undefined") {
    return;
  }

  try {
    cleanupMermaidZoom({ selector: surfaceSelector() });

    const mermaid = (await import("mermaid")).default;
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "neutral",
      themeVariables: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif"
      }
    });

    const { svg: generated, bindFunctions } = await mermaid.render(nextDiagramId(), props.chart.trim());
    svg.value = generated;
    error.value = "";
    await nextTick();
    const surface = document.querySelector(surfaceSelector());
    bindFunctions?.(surface ?? document.body);
    setupMermaidZoom({ selector: surfaceSelector() });
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : String(cause);
    error.value = `Mermaid render failed: ${message}`;
    svg.value = "";
  }
}

onMounted(() => {
  void renderDiagram();
});

watch(
  () => props.chart,
  () => {
    void renderDiagram();
  }
);

onBeforeUnmount(() => {
  cleanupMermaidZoom({ selector: surfaceSelector() });
});
</script>

<template>
  <div :id="rootId" class="mermaid-diagram">
    <div v-if="svg" class="mermaid mermaid-diagram__surface" v-html="svg" />
    <pre v-else-if="error" class="mermaid-diagram__error">{{ error }}</pre>
  </div>
</template>
