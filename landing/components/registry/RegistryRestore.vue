<script setup lang="ts">
import monitorImage from '~/assets/images/restore-monitor.png';
import teammateWallpaper from '~/assets/images/restore-teammate-wallpaper.svg';
import {
  marketingKitName,
  marketingKitVersion,
  marketingKitInstallCommand,
  marketingKitExportCommand,
  marketingKitRestoreCommand,
} from '~/data/marketingKitDemo';

const { t } = useI18n();
const { restoreGuideUrl, restoreGuideEnglishFallback, docsLabel } = useDocsLinks();
const copied = ref<1 | 2 | null>(null);
let reset: ReturnType<typeof setTimeout> | undefined;
onBeforeUnmount(() => { if (reset) clearTimeout(reset); });

async function copyCommand(which: 1 | 2) {
  const command = which === 1 ? marketingKitInstallCommand : marketingKitRestoreCommand;
  let success = false;
  try {
    await navigator.clipboard.writeText(command);
    success = true;
  } catch {
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
    }
  }
  if (!success) return;
  copied.value = which;
  if (reset) clearTimeout(reset);
  reset = setTimeout(() => { copied.value = null; }, 1600);
}
</script>

<template>
  <section
    id="restore-environment"
    class="registry-restore container"
    aria-labelledby="restore-title"
    dir="auto"
  >
    <div class="registry-restore__panel">
      <header class="registry-restore__header">
        <h2 id="restore-title">{{ t('shell.restore.title') }}</h2>
        <a class="registry-restore__guide" :href="restoreGuideUrl" target="_blank" rel="noreferrer">
          {{ docsLabel(t('shell.restore.guide'), restoreGuideEnglishFallback) }}
          <span aria-hidden="true">↗</span>
        </a>
      </header>
      <p class="registry-restore__subtitle">{{ t('shell.restore.subtitle') }}</p>

      <div class="registry-restore__flow">
        <div class="registry-restore__machine">
          <img
            class="registry-restore__monitor"
            :src="monitorImage"
            width="1577"
            height="997"
            alt=""
            loading="lazy"
            decoding="async"
            aria-hidden="true"
          />
          <div class="registry-restore__screen">
            <div class="registry-restore__stepbar">
              <p class="registry-restore__stage-label">
                <span class="registry-restore__step" aria-hidden="true">01</span>
                {{ t('shell.restore.from') }}
              </p>
              <button type="button" class="registry-restore__copy" :aria-label="t('registryUi.command.copyLabel')"
                @click="copyCommand(1)"
              >{{ copied === 1 ? t('registryUi.command.copied') : t('registryUi.command.copy') }}</button>
            </div>
            <p class="registry-restore__package" dir="ltr">
              <span>{{ marketingKitName }}</span><strong>v{{ marketingKitVersion }}</strong>
            </p>
            <div class="registry-restore__terminal" dir="ltr" :data-command="marketingKitInstallCommand">
              <span class="registry-restore__prompt" aria-hidden="true">$</span>
              <TypedCommand
                :lines="['npx -y', 'universal-agent-plugins', 'install --scope user']"
                :delay="180"
              />
            </div>
            <p class="registry-restore__commit">{{ t('shell.restore.commit') }}</p>
          </div>
        </div>

        <div class="registry-restore__handoff">
          <div class="registry-restore__files">
            <MarketingKitManifestPreview />
            <span class="registry-restore__file">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8Z" />
                <path d="M14 3v5h5" />
              </svg>
              <code dir="ltr">uap.lock</code>
            </span>
          </div>
          <code class="registry-restore__locked-version" dir="ltr">v{{ marketingKitVersion }}</code>
          <div class="registry-restore__export" dir="ltr" :data-command="marketingKitExportCommand">
            <TypedCommand :lines="['agentplugins deps export']" :delay="1700" />
          </div>
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
          <img
            class="registry-restore__monitor"
            :src="monitorImage"
            width="1577"
            height="997"
            alt=""
            loading="lazy"
            decoding="async"
            aria-hidden="true"
          />
          <img
            class="registry-restore__wallpaper"
            :src="teammateWallpaper"
            alt=""
            width="960"
            height="580"
            loading="lazy"
            aria-hidden="true"
          />
          <div class="registry-restore__screen">
            <div class="registry-restore__stepbar">
              <p class="registry-restore__stage-label">
                <span class="registry-restore__step" aria-hidden="true">02</span>
                {{ t('shell.restore.to') }}
              </p>
              <button type="button" class="registry-restore__copy" :aria-label="t('registryUi.command.copyLabel')"
                @click="copyCommand(2)"
              >{{ copied === 2 ? t('registryUi.command.copied') : t('registryUi.command.copy') }}</button>
            </div>
            <p class="registry-restore__package" dir="ltr">
              <span>{{ marketingKitName }}</span><strong>v{{ marketingKitVersion }}</strong>
            </p>
            <div class="registry-restore__terminal" dir="ltr" :data-command="marketingKitRestoreCommand">
              <span class="registry-restore__prompt" aria-hidden="true">$</span>
              <TypedCommand :lines="['agentplugins deps import', '--install']" :delay="330" />
            </div>
          </div>
        </div>
      </div>

      <p class="registry-restore__boundary">
        {{ t('shell.restore.requirements') }} {{ t('shell.restore.excludes') }}
      </p>
    </div>
  </section>
</template>

<style scoped lang="scss">
.registry-restore {
  padding-block: 32px 48px;
  &__panel {
    position: relative;
    padding: clamp(16px, 3vw, 36px);
    border: 1px solid var(--line-strong);
    border-radius: 16px;
    background: var(--surface);
  }
  &__header {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    justify-content: space-between;
    gap: 14px 24px;
  }
  h2 {
    margin: 0;
    font-size: clamp(1.5rem, 3vw, 2.5rem);
    line-height: 1.2;
    letter-spacing: -.035em;
    text-wrap: balance;
    background: linear-gradient(100deg, var(--text) 42%, var(--primary-bright) 85%);
    background-clip: text;
    color: transparent;
  }
  &__guide {
    color: var(--primary-bright);
    font-size: .9rem;
    text-decoration: underline;
    text-underline-offset: 4px;
  }
  &__subtitle {
    margin: 12px 0 20px;
    color: var(--muted);
    font-size: clamp(.9rem, 1.5vw, 1.1rem);
    line-height: 1.6;
    text-wrap: pretty;
  }
  &__flow {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(100px, .36fr) minmax(0, 1fr);
    align-items: center;
    gap: 16px;
  }
  &__machine {
    position: relative;
    min-width: 0;
    container-type: inline-size;
    aspect-ratio: 1577 / 1083;
    direction: ltr;
    animation: restore-arrive .85s ease both;
  }
  &__machine--teammate { animation-delay: .35s; }
  &__monitor {
    display: block;
    width: 100%;
    height: 100%;
    pointer-events: none;
    user-select: none;
  }
  &__machine--teammate &__monitor { filter: hue-rotate(46deg) saturate(1.18); }
  &__wallpaper {
    position: absolute;
    inset: 22% 10% 28%;
    width: 80%;
    height: 50%;
    object-fit: cover;
    border-radius: 3px;
    opacity: .36;
    pointer-events: none;
  }
  &__screen {
    position: absolute;
    inset: 20% 10% 29%;
    color: #e0e6ff;
    text-align: start;
    direction: ltr;
  }
  &__stepbar {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: .6cqw;
    min-width: 0;
  }
  &__stage-label {
    display: flex;
    align-items: center;
    gap: 2cqw;
    min-width: 0;
    margin: 0;
    color: #acb9ed;
    font-size: clamp(10px, 2.5cqw, 17px);
    font-weight: 650;
    line-height: 1.25;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }
  &__step {
    flex: none;
    color: #7885ba;
    font-size: 5.5cqw;
    font-weight: 400;
    line-height: 1;
  }
  &__copy {
    flex: none;
    padding: 3px 2px;
    min-width: 0;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: #b8b8f1;
    cursor: pointer;
    font-size: clamp(8px, 1.75cqw, 12px);
    font-weight: 700;
    text-transform: uppercase;
  }
  &__copy:hover, &__copy:focus-visible {
    color: white;
    outline-color: #00f0ff;
  }
  &__package {
    display: flex;
    flex-wrap: wrap;
    gap: 1.4cqw;
    align-items: baseline;
    margin: 2cqw 0 0;
    color: #e0e6ff;
    font-family: 'JetBrains Mono', monospace;
    font-size: clamp(9px, 2.4cqw, 15px);
    line-height: 1.35;
    overflow-wrap: anywhere;
    strong { color: #00f0ff; font-weight: 700; }
  }
  &__terminal {
    display: flex;
    gap: 1.5cqw;
    min-width: 0;
    margin-block-start: 3cqw;
    font-family: 'JetBrains Mono', monospace;
    font-size: clamp(9px, 2.35cqw, 16px);
    line-height: 1.4;
    color: #e0e6ff;
    letter-spacing: -.025em;
  }
  &__prompt { color: #00f0ff; flex: none; }
  &__commit {
    margin: 2cqw 0 0;
    color: #949cb6;
    font-family: 'JetBrains Mono', monospace;
    font-size: clamp(9px, 1.9cqw, 13px);
    line-height: 1.3;
  }
  &__handoff {
    position: relative;
    display: grid;
    justify-items: center;
    min-width: 0;
    gap: 12px;
    padding-block-end: 14%;
    animation: restore-arrive .8s .2s ease both;
  }
  &__files {
    display: flex;
    align-items: end;
    justify-content: center;
    gap: clamp(6px, 1.2vw, 24px);
  }
  &__file {
    display: grid;
    justify-items: center;
    gap: 7px;
    font-size: clamp(11px, 1vw, 16px);
    svg {
      width: clamp(25px, 2.7vw, 42px);
      height: clamp(30px, 3.3vw, 50px);
      color: var(--primary-bright);
      stroke: currentColor;
      stroke-width: 1.6;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
    code { color: var(--text); }
  }
  &__export {
    min-width: 0;
    max-width: 100%;
    font: 600 clamp(9px, .85vw, 12px)/1.35 'JetBrains Mono', monospace;
    color: var(--text);
  }
  &__locked-version {
    color: var(--primary-bright);
    font-size: clamp(11px, 1vw, 15px);
    font-weight: 700;
  }
  &__arrow {
    width: 100%;
    height: 36px;
    color: var(--cyan);
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  :dir(rtl) &__arrow { transform: scaleX(-1); }
  &__boundary {
    margin: 18px 0 0;
    padding-block-start: 20px;
    border-block-start: 1px solid var(--line);
    color: var(--muted);
    font-size: .78rem;
    line-height: 1.6;
    text-align: center;
  }
}
@keyframes restore-arrive {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}
@media (max-width: 1100px) {
  .registry-restore {
    &__flow { grid-template-columns: minmax(0, 1fr); gap: 12px; }
    &__machine { width: 100%; max-width: 500px; margin-inline: auto; }
    &__handoff { padding: 0; gap: 4px; }
    &__files { gap: 24px; }
    &__file { font-size: 14px; }
    &__file svg { width: 30px; height: 36px; }
    &__arrow, :dir(rtl) &__arrow {
      transform: rotate(90deg);
      width: 70px;
      height: 36px;
      margin-block: 17px;
    }
  }
}
@media (max-width: 480px) {
  .registry-restore {
    &__panel { padding-inline: 8px; }
    &__header, &__subtitle { margin-inline: 8px; }
    &__screen { inset: 19% 10% 27%; }
    &__stage-label { font-size: clamp(9px, 2.3cqw, 11px); }
    &__commit { display: none; }
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
}
</style>
