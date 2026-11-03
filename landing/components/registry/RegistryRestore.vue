<script setup lang="ts">
import monitorImage from '~/assets/images/restore-monitor.png';
import { marketingKitName, marketingKitVersion, isReviewedMarketingKit } from '~/data/marketingKitDemo';
import { restoreDemoCommands, type RestoreDemoMode } from '~/data/restoreDemo';
import { projectCatalogAutomaticSelection } from '~/utils/projectCatalogSource';
import type { RegistryPlugin } from '~/types/registry';

const props = defineProps<{ plugin?: RegistryPlugin }>();
const composer = useI18n({ useScope: 'local' });
composer.missingWarn = false;
composer.fallbackWarn = false;
const { t } = composer;
const { asset } = useSite();
const { current } = useDirectoryStatus();
const { userRestoreGuideUrl, restoreGuideEnglishFallback, docsLabel } = useDocsLinks();
const mode = ref<RestoreDemoMode>('project');
const commands = computed(() => restoreDemoCommands[mode.value]);
const projectQualified = computed(() => Boolean(current.value && props.plugin &&
  isReviewedMarketingKit(props.plugin) && projectCatalogAutomaticSelection(props.plugin)));
const projectPreviewOnly = computed(() => mode.value === 'project' && !projectQualified.value);
const copied = ref<1 | 2 | 3 | null>(null);
const copyFailed = ref<1 | 2 | 3 | null>(null);
let reset: ReturnType<typeof setTimeout> | undefined;
let copyRequest = 0;
let disposed = false;
function resetCopy() {
  copyRequest += 1;
  if (reset !== undefined) clearTimeout(reset);
  reset = undefined;
  copied.value = null;
  copyFailed.value = null;
}
watch(mode, resetCopy, { flush: 'sync' });
onBeforeUnmount(() => { disposed = true; resetCopy(); });
function stageLabel(which: 1 | 2 | 3) {
  return t(which === 1 ? 'shell.restore.from' : which === 2 ? 'shell.restore.to' : 'shell.restoreDemo.exportLabel');
}
function copyLabel(which: 1 | 2 | 3) {
  return t(copyFailed.value === which ? 'registryUi.command.copyFailed'
    : copied.value === which ? 'registryUi.command.copiedLabel' : 'registryUi.command.copyLabel') + ': ' + stageLabel(which);
}
function copyText(which: 1 | 2 | 3) {
  return t(copyFailed.value === which ? 'registryUi.command.copyFailed'
    : copied.value === which ? 'registryUi.command.copied' : 'registryUi.command.copy');
}
async function copyCommand(which: 1 | 2 | 3) {
  // Previewing a workflow never authorizes a currently unqualified package install.
  if (projectPreviewOnly.value) return;
  const command = which === 1 ? commands.value.install : which === 2 ? commands.value.restore : commands.value.export;
  if (!command) return;
  // An earlier feedback timer must not invalidate a new pending clipboard write.
  resetCopy();
  const request = copyRequest;
  const selectedMode = mode.value;
  let success = false;
  try {
    await navigator.clipboard.writeText(command);
    success = true;
  } catch {
    // Do not start a stale fallback or steal focus after switching modes/unmounting.
    if (disposed || request !== copyRequest || mode.value !== selectedMode) return;
    const previouslyFocused = document.activeElement;
    const field = document.createElement('textarea');
    field.value = command;
    field.style.cssText = 'position:fixed;opacity:0';
    try {
      document.body.appendChild(field);
      field.select();
      success = document.execCommand('copy');
    } catch {
      success = false;
    } finally {
      field.remove();
      if (previouslyFocused instanceof HTMLElement && previouslyFocused.isConnected) previouslyFocused.focus();
    }
  }
  if (disposed || request !== copyRequest || mode.value !== selectedMode) return;
  copied.value = success ? which : null;
  copyFailed.value = success ? null : which;
  if (reset) clearTimeout(reset);
  reset = setTimeout(resetCopy, 1600);
}
</script>

<template>
  <section id="restore-environment" class="registry-restore container" aria-labelledby="restore-title"
    dir="auto" :data-mode="mode" :data-project-qualified="String(projectQualified)">
    <div class="registry-restore__panel">
      <header class="registry-restore__header">
        <h2 id="restore-title">{{ t('shell.restore.title') }}</h2>
        <a class="registry-restore__guide" :href="userRestoreGuideUrl" target="_blank" rel="noreferrer">
          {{ docsLabel(t('shell.restore.guide'), restoreGuideEnglishFallback) }} <span aria-hidden="true">↗</span>
        </a>
      </header>
      <p class="registry-restore__subtitle">{{ t(`shell.restoreDemo.${mode}Subtitle`) }}</p>
      <div class="registry-restore__modes" role="group" :aria-label="t('shell.restoreDemo.scopeLabel')" :data-selected="mode">
        <span class="registry-restore__mode-thumb" aria-hidden="true" />
        <button v-for="value in (['project', 'global'] as const)" :key="value" type="button"
          :aria-pressed="mode === value" :data-mode-option="value" @click="mode = value">
          {{ t(`shell.restoreDemo.${value}Label`) }}
        </button>
      </div>
      <p v-if="projectPreviewOnly" class="registry-restore__qualification" role="note">
        {{ t('shell.restoreDemo.projectUnavailable') }}
      </p>

      <div class="registry-restore__flow">
        <div class="registry-restore__machine">
          <img class="registry-restore__monitor" :src="monitorImage" width="1577" height="997"
            alt="" loading="lazy" decoding="async" aria-hidden="true" />
          <div class="registry-restore__screen">
            <div class="registry-restore__stepbar">
              <p class="registry-restore__stage-label"><span class="registry-restore__step" aria-hidden="true">01</span>{{ t('shell.restore.from') }}</p>
              <button type="button" class="registry-restore__copy" :aria-label="copyLabel(1)"
                :disabled="projectPreviewOnly" @click="copyCommand(1)">{{ copyText(1) }}</button>
            </div>
            <div class="registry-restore__terminal" dir="ltr" :data-command="commands.install">
              <span class="registry-restore__prompt" aria-hidden="true">$</span>
              <TypedCommand :key="mode + '-install'" :lines="commands.install.split(' ')" :delay="180" />
            </div>
            <p class="registry-restore__package" dir="ltr"><span>{{ marketingKitName }}</span><strong>v{{ marketingKitVersion }}</strong></p>
          </div>
        </div>

        <div class="registry-restore__handoff">
          <div class="registry-restore__files">
            <MarketingKitManifestPreview />
            <span class="registry-restore__file">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z" /><path d="M14 3v5h5" />
              </svg>
              <code dir="ltr">uap.lock</code>
            </span>
          </div>
          <div v-if="commands.export" class="registry-restore__export">
            <code dir="ltr">{{ commands.export }}</code>
            <button type="button" class="registry-restore__copy" :aria-label="copyLabel(3)" @click="copyCommand(3)">{{ copyText(3) }}</button>
          </div>
          <p class="registry-restore__handoff-note">{{ t(`shell.restoreDemo.${mode}Handoff`) }}</p>
          <svg class="registry-restore__arrow" viewBox="0 0 160 36" fill="none" aria-hidden="true">
            <path d="M4 26C36 26 46 12 84 12h52" stroke-dasharray="3.5 7" />
            <path d="M138 5 157 12 138 19 141.5 12Z" fill="currentColor" stroke="none" />
          </svg>
          <i18n-t keypath="shell.restore.transfer" tag="p" class="sr-only">
            <template #manifest><code dir="ltr">uap.toml</code></template>
            <template #lockfile><code dir="ltr">uap.lock</code></template>
          </i18n-t>
        </div>

        <div class="registry-restore__machine registry-restore__machine--teammate">
          <img class="registry-restore__laptop" :src="asset('showcase/restore-teammate-laptop.svg')" width="1577" height="1083"
            alt="" loading="lazy" decoding="async" aria-hidden="true" />
          <div class="registry-restore__screen">
            <div class="registry-restore__stepbar">
              <p class="registry-restore__stage-label"><span class="registry-restore__step" aria-hidden="true">02</span>{{ t('shell.restore.to') }}</p>
              <button type="button" class="registry-restore__copy" :aria-label="copyLabel(2)"
                :disabled="projectPreviewOnly" @click="copyCommand(2)">{{ copyText(2) }}</button>
            </div>
            <div class="registry-restore__terminal" dir="ltr" :data-command="commands.restore">
              <span class="registry-restore__prompt" aria-hidden="true">$</span>
              <TypedCommand :key="mode + '-restore'" :lines="commands.restore.split(' ')" :delay="330" />
            </div>
            <p class="registry-restore__package" dir="ltr"><span>{{ marketingKitName }}</span><strong>v{{ marketingKitVersion }}</strong></p>
          </div>
        </div>
      </div>
      <span class="sr-only registry-restore__copy-status" role="status" aria-live="polite">
        {{ copied === null && copyFailed === null ? '' : t(copyFailed === null ? 'registryUi.command.copied' : 'registryUi.command.copyFailed') + ': ' + stageLabel((copyFailed ?? copied)!) }}
      </span>
    </div>
  </section>
</template>

<i18n src="../../features/restore/locales/en.json" lang="json" locale="en" />
<i18n src="../../features/restore/locales/ru.json" lang="json" locale="ru" />
<i18n src="../../features/restore/locales/uk.json" lang="json" locale="uk" />
<i18n src="../../features/restore/locales/zh.json" lang="json" locale="zh" />
<i18n src="../../features/restore/locales/es.json" lang="json" locale="es" />
<i18n src="../../features/restore/locales/hi.json" lang="json" locale="hi" />
<i18n src="../../features/restore/locales/ar.json" lang="json" locale="ar" />
<i18n src="../../features/restore/locales/pt.json" lang="json" locale="pt" />
<i18n src="../../features/restore/locales/fr.json" lang="json" locale="fr" />

<style scoped lang="scss">
.registry-restore {
  padding-block: 32px 48px;
  &__panel { position: relative; padding: clamp(16px, 3vw, 36px); border: 1px solid var(--line-strong); border-radius: 16px; background: var(--surface); }
  &__header { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 14px 24px; }
  h2 {
    margin: 0; font-size: clamp(1.5rem, 3vw, 2.5rem); line-height: 1.2; letter-spacing: -.035em; text-wrap: balance;
    background: linear-gradient(100deg, var(--text) 42%, var(--primary-bright) 85%); background-clip: text; color: transparent;
  }
  &__guide { color: var(--primary-bright); font-size: .9rem; text-decoration: underline; text-underline-offset: 4px; }
  &__subtitle { margin: 12px 0 16px; color: var(--muted); font-size: clamp(.9rem, 1.5vw, 1.1rem); line-height: 1.6; text-wrap: pretty; }
  &__modes {
    position: relative; display: inline-grid; grid-template-columns: repeat(2, minmax(0, 1fr));
    max-width: 100%; min-width: min(232px, 100%); padding: 4px; margin-block-end: 16px;
    border: 1px solid color-mix(in srgb, var(--primary-bright) 26%, var(--line)); border-radius: 999px;
    background: var(--surface);
    button { position: relative; z-index: 1; min-width: 0; padding: 8px 22px; border: 0; border-radius: 999px;
      background: transparent; color: var(--muted); font: inherit; font-size: .88rem; cursor: pointer; }
    button[aria-pressed='true'] { color: #fff; }
    button:focus-visible { outline: 2px solid var(--cyan); outline-offset: 3px; }
  }
  &__mode-thumb {
    position: absolute; inset-block: 4px; inset-inline-start: 4px; width: calc(50% - 4px); border-radius: 999px;
    background: linear-gradient(120deg, #7654ff, #5522ef); transition: transform .22s ease;
  }
  &__modes[data-selected='global'] &__mode-thumb { transform: translateX(100%); }
  :dir(rtl) &__modes[data-selected='global'] &__mode-thumb { transform: translateX(-100%); }
  &__qualification { margin: -4px 0 16px; color: var(--muted); font-size: .75rem; line-height: 1.5; }
  &__flow { display: grid; grid-template-columns: minmax(0, 1fr) minmax(100px, .36fr) minmax(0, 1fr); align-items: center; gap: 16px; }
  &__machine { position: relative; min-width: 0; container-type: inline-size; aspect-ratio: 1577 / 1083; direction: ltr; animation: restore-arrive .85s ease both; }
  &__machine--teammate { animation-delay: .35s; }
  &__monitor, &__laptop { display: block; width: 100%; height: 100%; pointer-events: none; user-select: none; }
  &__screen { position: absolute; inset: 20% 10% 29%; color: #e0e6ff; text-align: start; direction: ltr; }
  &__machine--teammate &__screen { inset: 19% 14% 31%; }
  &__stepbar { display: flex; align-items: flex-start; justify-content: space-between; gap: .6cqw; min-width: 0; }
  &__stage-label { display: flex; align-items: center; gap: 2cqw; min-width: 0; margin: 0; color: #acb9ed;
    font-size: clamp(10px, 2.5cqw, 17px); font-weight: 650; line-height: 1.25; text-transform: uppercase; overflow-wrap: anywhere; }
  &__step { flex: none; color: #7885ba; font-size: 5.5cqw; font-weight: 400; line-height: 1; }
  &__copy { display: inline-grid; place-items: center; flex: none; padding: 3px 4px; min-width: 24px; min-height: 24px;
    border: 0; border-radius: 4px; background: transparent; color: #b8b8f1; cursor: pointer;
    font-size: clamp(8px, 1.75cqw, 12px); font-weight: 700; text-transform: uppercase; }
  &__copy:hover, &__copy:focus-visible { color: white; outline-color: #00f0ff; }
  &__copy:disabled { opacity: .55; cursor: not-allowed; }
  &__package { display: flex; flex-wrap: wrap; gap: 1.4cqw; align-items: baseline; margin: 2cqw 0 0; color: #e0e6ff;
    font-family: 'JetBrains Mono', monospace; font-size: clamp(9px, 2.4cqw, 15px); line-height: 1.35; overflow-wrap: anywhere;
    strong { color: #00f0ff; font-weight: 700; } }
  &__screen:has(.typed-command[data-typing='true']) &__package { visibility: hidden; }
  &__terminal { display: flex; gap: 1.5cqw; min-width: 0; margin-block-start: 3cqw; font-family: 'JetBrains Mono', monospace;
    font-size: clamp(9px, 2.35cqw, 16px); line-height: 1.4; color: #e0e6ff; letter-spacing: -.025em; }
  &__prompt { color: #00f0ff; flex: none; }
  &__handoff { position: relative; display: grid; justify-items: center; min-width: 0; gap: 12px; padding-block-end: 14%; animation: restore-arrive .8s .2s ease both; }
  &__files { display: flex; align-items: end; justify-content: center; gap: clamp(6px, 1.2vw, 24px); }
  &__file { display: grid; justify-items: center; gap: 7px; font-size: clamp(11px, 1vw, 16px);
    svg { width: clamp(25px, 2.7vw, 42px); height: clamp(30px, 3.3vw, 50px); color: var(--primary-bright);
      stroke: currentColor; stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round; }
    code { color: var(--text); } }
  &__export { display: grid; justify-items: center; gap: 4px; width: 100%; min-width: 0; max-width: 260px;
    code { max-width: 100%; color: var(--text); font-size: clamp(10px, .85vw, 12px); overflow-wrap: anywhere; text-align: center; }
    button { font-size: 10px; color: var(--primary-bright); } }
  &__handoff-note { max-width: 230px; margin: 0; color: var(--muted); font-size: clamp(10px, .85vw, 12px); font-weight: 400; line-height: 1.5; text-align: center; text-wrap: pretty; }
  &__arrow { width: 100%; height: 36px; color: var(--cyan); stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  :dir(rtl) &__arrow { transform: scaleX(-1); }
}
@keyframes restore-arrive { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 1100px) {
  .registry-restore {
    &__flow { grid-template-columns: minmax(0, 1fr); gap: 12px; }
    &__machine { width: 100%; max-width: 500px; margin-inline: auto; }
    &__handoff { padding: 0; gap: 4px; }
    &__files { gap: 24px; }
    &__file { font-size: 14px; }
    &__file svg { width: 30px; height: 36px; }
    &__arrow, :dir(rtl) &__arrow { transform: rotate(90deg); width: 70px; height: 36px; margin-block: 17px; }
  }
}
@media (max-width: 480px) {
  .registry-restore {
    &__panel { padding-inline: 8px; }
    &__header, &__subtitle, &__modes, &__qualification { margin-inline: 8px; }
    &__modes { max-width: calc(100% - 16px); min-width: 0; }
    &__screen { inset: 19% 10% 27%; }
    &__stage-label { font-size: clamp(9px, 2.3cqw, 11px); }
    &__terminal { margin-block-start: 1.8cqw; }
  }
}
@media (max-width: 300px) {
  .registry-restore {
    &__stage-label { gap: .8cqw; font-size: 9px; }
    &__step { font-size: 4.6cqw; }
    &__package { margin-block-start: 1cqw; font-size: 9px; }
    &__terminal { font-size: 9px; margin-block-start: 1cqw; }
    &__copy { font-size: 8px; padding: 1px; }
  }
}
@media (prefers-reduced-motion: reduce) {
  .registry-restore__machine, .registry-restore__handoff { animation: none; }
  .registry-restore__mode-thumb { transition: none; }
}
</style>
