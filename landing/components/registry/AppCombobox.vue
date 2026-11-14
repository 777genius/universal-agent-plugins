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
} from 'reka-ui';
const { t } = useI18n();

type ComboboxOption = {
  value: string;
  label: string;
};

const props = defineProps<{
  modelValue: string;
  options: readonly ComboboxOption[];
  label: string;
  searchPlaceholder: string;
  leadingIcon?:
    'category' | 'component' | 'source' | 'trust' | 'agent' | 'authentication' | 'owner';
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();
const hydrated = ref(false);
onMounted(() => {
  hydrated.value = true;
});

watchEffect(() => {
  if (!props.options.length)
    throw new Error(
      t('registryUi.controls.optionsRequired', { component: 'AppCombobox', label: props.label }),
    );
  if (props.options.some((option) => option.value === ''))
    throw new Error(
      t('registryUi.controls.nonEmptyOptions', { component: 'AppCombobox', label: props.label }),
    );
  if (!props.options.some((option) => option.value === props.modelValue))
    throw new Error(
      t('registryUi.controls.unknownValue', { component: 'AppCombobox', label: props.label }),
    );
});

function displayValue(value: unknown) {
  return props.options.find((option) => option.value === value)?.label ?? '';
}

function updateValue(value: unknown) {
  if (typeof value === 'string' && props.options.some((option) => option.value === value)) {
    emit('update:modelValue', value);
  }
}

function selectCurrentText(event: FocusEvent | MouseEvent) {
  if (event.currentTarget instanceof HTMLInputElement) event.currentTarget.select();
}
</script>

<template>
  <ComboboxRoot :model-value="modelValue" :open-on-click="true" @update:model-value="updateValue">
    <ComboboxAnchor class="app-combobox__anchor">
      <FilterIcon v-if="leadingIcon" :name="leadingIcon" />
      <svg
        v-else
        class="app-combobox__search-icon"
        aria-hidden="true"
        viewBox="0 0 24 24"
        fill="none"
      >
        <circle cx="11" cy="11" r="6.5" />
        <path d="m16 16 4 4" />
      </svg>
      <ComboboxInput
        class="app-combobox__input"
        :aria-label="label"
        :data-hydrated="hydrated ? 'true' : 'false'"
        :display-value="displayValue"
        :placeholder="searchPlaceholder"
        @focus="selectCurrentText"
        @click="selectCurrentText"
      />
      <ComboboxTrigger
        class="app-combobox__trigger"
        :aria-label="t('registryUi.combobox.open', { label })"
      >
        <svg aria-hidden="true" viewBox="0 0 16 16" fill="none">
          <path d="m3.5 6 4.5 4 4.5-4" />
        </svg>
      </ComboboxTrigger>
    </ComboboxAnchor>
    <ComboboxPortal>
      <ComboboxContent
        class="app-combobox__content"
        position="popper"
        align="start"
        :side-offset="7"
      >
        <ComboboxViewport class="app-combobox__viewport">
          <ComboboxEmpty class="app-combobox__empty">{{
            t('registryUi.combobox.empty')
          }}</ComboboxEmpty>
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
