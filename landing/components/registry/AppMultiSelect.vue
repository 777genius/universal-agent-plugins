<script setup lang="ts">
import { PopoverArrow, PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui';

type MultiSelectOption = {
  value: string;
  label: string;
  icon?: string;
  disabled?: boolean;
  description?: string;
};

type AutoDetectOption = {
  label: string;
  summary?: string;
  description: string;
};

const props = defineProps<{
  modelValue: readonly string[];
  options: readonly MultiSelectOption[];
  label: string;
  autoOption?: AutoDetectOption;
  autoSelected?: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string[]];
  'update:autoSelected': [value: boolean];
}>();
const hydrated = ref(false);
onMounted(() => {
  hydrated.value = true;
});

const selected = computed(() =>
  props.options.filter((option) => props.modelValue.includes(option.value)),
);
const summary = computed(() =>
  props.autoSelected && props.autoOption
    ? (props.autoOption.summary ?? props.autoOption.label)
    : selected.value.length === 1
      ? selected.value[0]!.label
      : `${selected.value.length} agents`,
);

watchEffect(() => {
  if (!props.options.length)
    throw new Error(`AppMultiSelect "${props.label}" requires at least one option`);
  if (!props.modelValue.length && !props.autoSelected)
    throw new Error(`AppMultiSelect "${props.label}" requires at least one selected value`);
  if (props.modelValue.some((value) => !props.options.some((option) => option.value === value))) {
    throw new Error(`AppMultiSelect "${props.label}" received an unknown value`);
  }
});

function selectAuto() {
  if (!props.autoOption) return;
  emit('update:autoSelected', true);
}

function toggle(value: string) {
  if (props.options.find((option) => option.value === value)?.disabled) return;
  if (props.autoSelected) {
    emit('update:autoSelected', false);
    emit('update:modelValue', [value]);
    return;
  }
  if (props.modelValue.includes(value)) {
    if (props.modelValue.length === 1) return;
    emit(
      'update:modelValue',
      props.modelValue.filter((item) => item !== value),
    );
    return;
  }
  const selectedValues = new Set([...props.modelValue, value]);
  emit(
    'update:modelValue',
    props.options
      .filter((option) => selectedValues.has(option.value))
      .map((option) => option.value),
  );
}
</script>

<template>
  <PopoverRoot>
    <PopoverTrigger
      class="app-multiselect__trigger"
      :aria-label="`${label}: ${summary}`"
      :data-hydrated="hydrated ? 'true' : 'false'"
    >
      <span class="app-multiselect__value">
        <span v-if="!autoSelected" class="app-multiselect__icons" aria-hidden="true">
          <span v-for="option in selected.slice(0, 3)" :key="option.value"
            ><img v-if="option.icon" :src="option.icon" alt="" width="19" height="19"
          ></span>
        </span>
        <span v-else class="app-multiselect__auto-icon" aria-hidden="true"
          ><svg viewBox="0 0 24 24" fill="none">
            <path d="m12 3 .65 2.35L15 6l-2.35.65L12 9l-.65-2.35L9 6l2.35-.65L12 3Z" />
            <path
              d="m17.5 10 .85 3.15L21.5 14l-3.15.85L17.5 18l-.85-3.15L13.5 14l3.15-.85L17.5 10Z"
            />
            <path
              d="m6.5 11 .65 2.35L9.5 14l-2.35.65L6.5 17l-.65-2.35L3.5 14l2.35-.65L6.5 11Z"
            /></svg
        ></span>
        <span>{{ summary }}</span>
      </span>
      <span class="app-multiselect__chevron" aria-hidden="true"
        ><svg viewBox="0 0 16 16" fill="none"><path d="m3.5 6 4.5 4 4.5-4" /></svg
      ></span>
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent
        class="app-multiselect__content"
        align="start"
        :side-offset="7"
        :collision-padding="14"
      >
        <div class="app-multiselect__heading">
          <strong>{{ label }}</strong
          ><span>{{ autoSelected ? 'Auto-detect' : `${selected.length} selected` }}</span>
        </div>
        <button
          v-if="autoOption"
          type="button"
          class="app-multiselect__auto"
          :class="{ 'app-multiselect__auto--selected': autoSelected }"
          :aria-pressed="autoSelected"
          @click="selectAuto"
        >
          <span class="app-multiselect__auto-symbol" aria-hidden="true"
            ><svg viewBox="0 0 24 24" fill="none">
              <path d="m12 3 .65 2.35L15 6l-2.35.65L12 9l-.65-2.35L9 6l2.35-.65L12 3Z" />
              <path
                d="m17.5 10 .85 3.15L21.5 14l-3.15.85L17.5 18l-.85-3.15L13.5 14l3.15-.85L17.5 10Z"
              />
              <path
                d="m6.5 11 .65 2.35L9.5 14l-2.35.65L6.5 17l-.65-2.35L3.5 14l2.35-.65L6.5 11Z"
              /></svg
          ></span>
          <span
            ><strong>{{ autoOption.label }}</strong
            ><small>{{ autoOption.description }}</small></span
          >
          <span class="app-multiselect__check" aria-hidden="true">✓</span>
        </button>
        <div v-if="autoOption" class="app-multiselect__separator">
          <span>Or choose specific agents</span>
        </div>
        <div class="app-multiselect__options" role="group" :aria-label="label">
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            class="app-multiselect__item"
            :class="{
              'app-multiselect__item--selected': !autoSelected && modelValue.includes(option.value),
            }"
            role="checkbox"
            :aria-checked="!autoSelected && modelValue.includes(option.value)"
            :disabled="
              option.disabled ||
              (!autoSelected && modelValue.length === 1 && modelValue.includes(option.value))
            "
            @click="toggle(option.value)"
          >
            <span class="app-multiselect__item-icon"
              ><img v-if="option.icon" :src="option.icon" alt="" width="20" height="20"
            ></span>
            <span
              ><span>{{ option.label }}</span
              ><small v-if="option.description">{{ option.description }}</small></span
            >
            <span class="app-multiselect__check" aria-hidden="true">✓</span>
          </button>
        </div>
        <p>
          {{
            autoSelected
              ? 'The CLI checks this plugin against installed agents, skips incompatible ones, and lets you confirm the targets. ChatGPT is included only when the plugin provides a verified connection.'
              : 'Select every agent that should receive this plugin. At least one stays selected.'
          }}
        </p>
        <PopoverArrow class="app-multiselect__arrow" />
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
