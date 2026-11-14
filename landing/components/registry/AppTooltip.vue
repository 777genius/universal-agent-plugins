<script setup lang="ts">
import {
  TooltipArrow,
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from 'reka-ui';

defineProps<{ aboveHeader?: boolean }>();
</script>

<template>
  <TooltipProvider :delay-duration="180" :skip-delay-duration="80">
    <TooltipRoot>
      <TooltipTrigger as-child>
        <slot name="trigger" />
      </TooltipTrigger>
      <TooltipPortal>
        <TooltipContent
          class="app-tooltip"
          :side-offset="8"
          :style="aboveHeader ? { zIndex: 1100 } : undefined"
        >
          <slot />
          <TooltipArrow class="app-tooltip__arrow" :width="12" :height="6" />
        </TooltipContent>
      </TooltipPortal>
    </TooltipRoot>
  </TooltipProvider>
</template>
