<script setup lang="ts">
defineProps<{ modelValue: string; label: string; placeholder: string; clearLabel: string }>();
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();
</script>

<template>
  <label class="search-field">
    <span class="sr-only">{{ label }}</span>
    <svg class="search-field__icon" aria-hidden="true" viewBox="0 0 24 24" fill="none">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
    <input
      :value="modelValue"
      type="search"
      :aria-label="label"
      :placeholder="placeholder"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <button
      v-if="modelValue"
      class="search-field__clear"
      type="button"
      :aria-label="clearLabel"
      @click="emit('update:modelValue', '')"
    >
      <span aria-hidden="true">×</span>
    </button>
  </label>
</template>
