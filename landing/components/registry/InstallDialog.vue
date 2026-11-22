<script setup lang="ts">
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
} from 'reka-ui';

const open = defineModel<boolean>('open', { required: true });
defineProps<{ title: string; source?: string; sourceUrl?: string; icon?: string }>();
const { t } = useI18n();
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogTrigger as-child><slot name="trigger" /></DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="install-dialog__overlay" />
      <DialogContent class="install-dialog">
        <header class="install-dialog__header">
          <img v-if="icon" :src="icon" alt="" width="36" height="36" />
          <div class="install-dialog__identity">
            <DialogTitle class="install-dialog__title">{{ title }}</DialogTitle>
            <a v-if="source && sourceUrl" :href="sourceUrl" target="_blank" rel="noreferrer">{{
              source
            }}</a>
            <span v-else-if="source">{{ source }}</span>
          </div>
          <DialogClose
            class="install-dialog__close"
            :aria-label="t('registryUi.install.closeDialog')"
          >
            <span aria-hidden="true">×</span>
          </DialogClose>
        </header>
        <div class="install-dialog__body">
          <DialogDescription class="install-dialog__description">{{
            t('registryUi.install.dialogDescription')
          }}</DialogDescription>
          <slot />
        </div>
        <footer v-if="$slots.footer" class="install-dialog__footer"><slot name="footer" /></footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.install-dialog__overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgb(0 0 0 / 65%);
  backdrop-filter: blur(5px);
}
.install-dialog {
  position: fixed;
  z-index: 1101;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: min(960px, calc(100vw - 48px));
  max-height: calc(100dvh - 48px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--line-strong);
  border-radius: 22px;
  background: var(--surface-solid);
  color: var(--text);
  box-shadow: 0 30px 100px rgb(0 0 0 / 40%);
}
/* The picker is portaled beside the dialog and must share its modal layer. */
:global(body:has(.install-dialog) .app-multiselect__content) {
  z-index: 1110;
}
.install-dialog__header {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 10px;
  padding: 10px 18px;
  border-bottom: 1px solid var(--line);
}
.install-dialog__header > img {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  object-fit: contain;
}
.install-dialog__identity {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.install-dialog__title {
  margin: 0 0 2px;
  font-size: 1rem;
  line-height: 1.3;
}
.install-dialog__identity :is(a, span) {
  color: var(--muted);
  font-size: 0.72rem;
}
.install-dialog__close {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--text);
  font-size: 1.4rem;
  cursor: pointer;
}
.install-dialog__close:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}
.install-dialog__body {
  position: relative;
  z-index: 0;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 22px 26px 62px;
}
.install-dialog__footer {
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  padding: 0 26px 12px;
  border-top: 1px solid var(--line);
  background: var(--surface-solid);
}
.install-dialog__footer :deep(.command-snippet) {
  position: relative;
  margin-top: -36px;
}
.install-dialog__footer :deep(.native-cli-checkbox) {
  margin-top: 8px;
  gap: 7px;
  font-size: 0.75rem;
  line-height: 1.3;
}
.install-dialog__footer :deep(.native-cli-checkbox input) {
  width: 15px;
  height: 15px;
}
.install-dialog__footer :deep(.command-snippet pre) {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 22dvh;
  overflow-y: auto;
}
.install-dialog__description {
  margin: 0 0 20px;
  color: var(--muted);
  line-height: 1.5;
}
.install-dialog__body :deep(.plugin-card__install-panel) {
  margin: 0;
  padding: 0;
  border: 0;
}
.install-dialog__body :deep(.plugin-card__install-panel) {
  gap: 16px;
}
.install-dialog__body :deep(.plugin-card__install) {
  gap: 14px;
}
.install-dialog__body :deep(.command-snippet pre) {
  font-size: 0.86rem;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.install-dialog__body :deep(.plugin-card__author) {
  font-size: 0.85rem;
  line-height: 1.6;
}
.install-dialog__body :deep(.app-multiselect__value) {
  font-size: 0.9rem;
}
.install-dialog__body :deep(.app-multiselect__trigger) {
  min-height: 48px;
}
@media (max-width: 600px) {
  .install-dialog {
    width: calc(100vw - 16px);
    max-height: calc(100dvh - 16px);
    border-radius: 16px;
  }
  .install-dialog__header {
    padding: 9px 12px;
    gap: 9px;
  }
  .install-dialog__footer {
    padding: 0 14px 10px;
  }
  .install-dialog__body {
    padding: 18px 14px 54px;
  }
}
</style>
