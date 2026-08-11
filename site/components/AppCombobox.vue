<script setup lang="ts">
import {
  ComboboxAnchor,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxItemIndicator,
  ComboboxPortal,
  ComboboxRoot,
  ComboboxTrigger,
  ComboboxViewport,
} from 'reka-ui'

type ComboboxOption = {
  value: string
  label: string
}

const props = defineProps<{
  modelValue: string
  options: readonly ComboboxOption[]
  label: string
  searchPlaceholder: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

watchEffect(() => {
  if (!props.options.length) throw new Error(`AppCombobox "${props.label}" requires at least one option`)
  if (props.options.some(option => option.value === '')) throw new Error(`AppCombobox "${props.label}" options must use non-empty values`)
  if (!props.options.some(option => option.value === props.modelValue)) throw new Error(`AppCombobox "${props.label}" received an unknown value`)
})

function displayValue(value: unknown) {
  return props.options.find(option => option.value === value)?.label ?? ''
}

function updateValue(value: unknown) {
  if (typeof value === 'string' && props.options.some(option => option.value === value)) {
    emit('update:modelValue', value)
  }
}

function selectCurrentText(event: FocusEvent | MouseEvent) {
  if (event.currentTarget instanceof HTMLInputElement) event.currentTarget.select()
}
</script>

<template>
  <ComboboxRoot :model-value="modelValue" :open-on-click="true" @update:model-value="updateValue">
    <ComboboxAnchor class="app-combobox__anchor">
      <svg class="app-combobox__search-icon" aria-hidden="true" viewBox="0 0 24 24" fill="none">
        <circle cx="11" cy="11" r="6.5" />
        <path d="m16 16 4 4" />
      </svg>
      <ComboboxInput
        class="app-combobox__input"
        :aria-label="label"
        :display-value="displayValue"
        :placeholder="searchPlaceholder"
        @focus="selectCurrentText"
        @click="selectCurrentText"
      />
      <ComboboxTrigger class="app-combobox__trigger" :aria-label="`Open ${label.toLowerCase()}`">
        <span aria-hidden="true">⌄</span>
      </ComboboxTrigger>
    </ComboboxAnchor>
    <ComboboxPortal>
      <ComboboxContent class="app-combobox__content" position="popper" align="start" :side-offset="7">
        <ComboboxViewport class="app-combobox__viewport">
          <ComboboxEmpty class="app-combobox__empty">No matching categories</ComboboxEmpty>
          <ComboboxItem
            v-for="option in options"
            :key="option.value"
            class="app-combobox__item"
            :value="option.value"
            :text-value="option.label"
          >
            <span>{{ option.label }}</span>
            <ComboboxItemIndicator class="app-combobox__indicator">✓</ComboboxItemIndicator>
          </ComboboxItem>
        </ComboboxViewport>
      </ComboboxContent>
    </ComboboxPortal>
  </ComboboxRoot>
</template>
