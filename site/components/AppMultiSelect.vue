<script setup lang="ts">
import {
  PopoverArrow,
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
} from 'reka-ui'

type MultiSelectOption = {
  value: string
  label: string
  icon?: string
  disabled?: boolean
  description?: string
}

const props = defineProps<{
  modelValue: readonly string[]
  options: readonly MultiSelectOption[]
  label: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const selected = computed(() => props.options.filter(option => props.modelValue.includes(option.value)))
const summary = computed(() => selected.value.length === 1 ? selected.value[0]!.label : `${selected.value.length} agents`)

watchEffect(() => {
  if (!props.options.length) throw new Error(`AppMultiSelect "${props.label}" requires at least one option`)
  if (!props.modelValue.length) throw new Error(`AppMultiSelect "${props.label}" requires at least one selected value`)
  if (props.modelValue.some(value => !props.options.some(option => option.value === value))) {
    throw new Error(`AppMultiSelect "${props.label}" received an unknown value`)
  }
})

function toggle(value: string) {
  if (props.options.find(option => option.value === value)?.disabled) return
  if (props.modelValue.includes(value)) {
    if (props.modelValue.length === 1) return
    emit('update:modelValue', props.modelValue.filter(item => item !== value))
    return
  }
  const selectedValues = new Set([...props.modelValue, value])
  emit('update:modelValue', props.options.filter(option => selectedValues.has(option.value)).map(option => option.value))
}
</script>

<template>
  <PopoverRoot>
    <PopoverTrigger class="app-multiselect__trigger" :aria-label="`${label}: ${summary}`">
      <span class="app-multiselect__value">
        <span class="app-multiselect__icons" aria-hidden="true">
          <span v-for="option in selected.slice(0, 3)" :key="option.value"><img v-if="option.icon" :src="option.icon" alt="" width="19" height="19" /></span>
        </span>
        <span>{{ summary }}</span>
      </span>
      <span class="app-multiselect__chevron" aria-hidden="true">⌄</span>
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent class="app-multiselect__content" align="start" :side-offset="7">
        <div class="app-multiselect__heading"><strong>{{ label }}</strong><span>{{ selected.length }} selected</span></div>
        <div class="app-multiselect__options" role="group" :aria-label="label">
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            class="app-multiselect__item"
            :class="{ 'app-multiselect__item--selected': modelValue.includes(option.value) }"
            role="checkbox"
            :aria-checked="modelValue.includes(option.value)"
            :disabled="option.disabled || (modelValue.length === 1 && modelValue.includes(option.value))"
            @click="toggle(option.value)"
          >
            <span class="app-multiselect__item-icon"><img v-if="option.icon" :src="option.icon" alt="" width="20" height="20" /></span>
            <span><span>{{ option.label }}</span><small v-if="option.description">{{ option.description }}</small></span>
            <span class="app-multiselect__check" aria-hidden="true">✓</span>
          </button>
        </div>
        <p>Select every agent that should receive this plugin. At least one stays selected.</p>
        <PopoverArrow class="app-multiselect__arrow" />
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
