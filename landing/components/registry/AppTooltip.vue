<script setup lang="ts">
import { ref } from 'vue';
import {
  TooltipArrow,
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from 'reka-ui';

defineProps<{ aboveHeader?: boolean }>();

// Closed content must release its global open-event listener before refocus.
const open = ref(false);
</script>

<template>
  <TooltipProvider :delay-duration="180" :skip-delay-duration="80">
    <TooltipRoot v-model:open="open">
      <TooltipTrigger as-child>
        <slot name="trigger" />
      </TooltipTrigger>
      <TooltipPortal>
        <TooltipContent
          v-if="open"
          class="app-tooltip"
          :side-offset="8"
          :style="aboveHeader ? { zIndex: 1120 } : undefined"
        >
          <slot />
          <TooltipArrow class="app-tooltip__arrow" :width="12" :height="6" />
        </TooltipContent>
      </TooltipPortal>
    </TooltipRoot>
  </TooltipProvider>
</template>
