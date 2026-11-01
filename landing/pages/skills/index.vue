<script setup lang="ts">
import type { SkillListing, SkillsView } from '~/types/skillsCatalog';
import { skillClients } from '~/data/skillClients';
import {
  skillIdentityFromId,
  skillInstallCommand,
  skillNeedsManualSelection,
  skillPublisherUrl,
  skillRepositoryUrl,
} from '~/utils/skillsCatalog';

const { t, n } = useI18n({ useScope: 'local' });
const localePath = useLocalePath();
const route = useRoute();
const { asset } = useSite();
const selectedSkill = useState<SkillListing | null>('skills-catalog:selected-skill', () => null);

const listingId = computed(() => {
  const value = route.query.id;
  return typeof value === 'string' && skillIdentityFromId(value) ? value : '';
});
const sourceIdentity = computed(() => skillIdentityFromId(listingId.value));
function queryName(): string {
  const value = route.query.q;
  if (typeof value === 'string' && value.trim() && value.length <= 512) return value.trim();
  return listingId.value.split(':').at(-1) ?? '';
}
const name = ref(queryName());
watch(() => route.query, () => { name.value = queryName(); });
const search = ref(name.value);
watch(name, (value) => { search.value = value; });
const view = ref<SkillsView>('popular');
const { items, loading: catalogLoading } = useSkillsCatalog(search, view);
const {
  document: sourceDocument,
  loading: documentLoading,
  failed: documentFailed,
  retry: retryDocument,
} = useSkillDocument(listingId, name);

const listedSkill = computed(() =>
  items.value.find((item) => item.id === listingId.value) ??
  (selectedSkill.value?.id === listingId.value ? selectedSkill.value : undefined),
);
const resolvedDocument = computed(() => {
  const result = sourceDocument.value;
  const source = sourceIdentity.value;
  if (!result || !source || result.source.toLowerCase() !== (source.repository ?? new URL(source.url).hostname).toLowerCase())
    return undefined;
  return result;
});
// The catalog search may omit an existing Skill. A successful source read
// still supplies a valid install identity, without fabricating install metrics.
const fallbackSkill = computed<SkillListing | undefined>(() => {
  if (resolvedDocument.value?.status !== 'resolved' || !sourceIdentity.value) return undefined;
  return {
    id: listingId.value,
    providerId: 'skills-sh',
    providerUrl: '',
    title: name.value,
    skillNameHint: name.value,
    source: sourceIdentity.value,
    metrics: { installs: 0, window: 'all-time' },
    availability: 'unverified',
  };
});
const skill = computed(() => listedSkill.value ?? fallbackSkill.value);
const repositoryUrl = computed(() => skill.value ? skillRepositoryUrl(skill.value) : undefined);
const publisherUrl = computed(() => skill.value ? skillPublisherUrl(skill.value) : undefined);
const sourceUrl = computed(() => repositoryUrl.value ?? publisherUrl.value);
const sourceName = computed(() =>
  skill.value?.source.repository ??
  publisherUrl.value?.replace('https://', '') ??
  t('registryUi.skills.externalSource'),
);
const isBusy = computed(() => !skill.value && (catalogLoading.value || documentLoading.value));
const chooseInTerminal = ref(true);
const targets = ref<string[]>(['codex']);
const activeTab = ref<'command' | 'prompt'>('command');
const command = computed(() =>
  skill.value ? skillInstallCommand(skill.value, chooseInTerminal.value ? undefined : targets.value) : undefined,
);
const prompt = computed(() => t('registryUi.skills.detail.promptText', { command: command.value ?? '' }));
const options = computed(() =>
  skillClients.map((client) => ({
    value: client.id,
    label: client.name,
    icon: asset('client-icons/' + client.icon),
    description: t('registryUi.skills.checkedBeforeInstall'),
  })),
);
const terminalOption = computed(() => ({
  label: t('registryUi.skills.chooseInTerminal'),
  summary: t('registryUi.skills.chooseInTerminal'),
  description: t('registryUi.skills.chooseInTerminalHelp'),
}));
function updateTargets(values: string[]) {
  const supported = new Set(skillClients.map((client) => client.id));
  const next = values.filter((value) => supported.has(value));
  if (next.length) targets.value = next;
}
function selectTab(tab: 'command' | 'prompt', focus = false) {
  activeTab.value = tab;
  if (focus) void nextTick(() => document.getElementById('skills-' + tab + '-tab')?.focus());
}
function onTabKeydown(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
  event.preventDefault();
  const next = event.key === 'Home' ? 'command' :
    event.key === 'End' ? 'prompt' :
      activeTab.value === 'command' ? 'prompt' : 'command';
  selectTab(next, true);
}

useHead({
  title: computed(() =>
    skill.value
      ? skill.value.title + ' | Universal Agent Plugins'
      : 'Agent Skills | Universal Agent Plugins',
  ),
  // A query-specific detail page is not prerendered with individual content.
  meta: [{ name: 'robots', content: 'noindex,follow' }],
});
</script>

<template>
  <div class="registry-surface skills-detail">
    <PageBackground />
    <div class="container skills-detail__container">
      <nav class="skills-detail__breadcrumbs" :aria-label="t('registryUi.skills.detail.breadcrumb')">
        <NuxtLink :to="localePath({ path: '/plugins/', query: { tab: 'skills' } })">
          {{ t('registryUi.skills.skills') }}
        </NuxtLink>
        <span aria-hidden="true">/</span>
        <span>{{ skill?.title ?? t('registryUi.skills.detail.skill') }}</span>
      </nav>

      <div v-if="isBusy" class="skills-detail__panel" role="status">
        {{ t('registryUi.skills.loading') }}
      </div>
      <div v-else-if="!skill" class="skills-detail__panel skills-detail__empty">
        <h1>{{ t('registryUi.skills.detail.notFound') }}</h1>
        <p>{{ t(documentFailed ? 'registryUi.skills.detail.documentUnavailable' : 'registryUi.skills.detail.notFoundHelp') }}</p>
        <div class="skills-detail__actions">
          <button v-if="documentFailed" type="button" class="button button--secondary" @click="retryDocument">
            {{ t('registryUi.skills.retry') }}
          </button>
          <NuxtLink class="button button--secondary" :to="localePath({ path: '/plugins/', query: { tab: 'skills' } })">
            {{ t('registryUi.skills.detail.backToSkills') }}
          </NuxtLink>
        </div>
      </div>
      <div v-else class="skills-detail__columns">
        <main class="skills-detail__main">
          <header class="skills-detail__header">
            <p class="skills-detail__eyebrow">{{ t('registryUi.skills.detail.skill') }}</p>
            <h1>{{ skill.title }}</h1>
            <p class="skills-detail__origin">
              <a v-if="sourceUrl" :href="sourceUrl" target="_blank" rel="noopener noreferrer">
                {{ sourceName }} <span aria-hidden="true">↗</span>
              </a>
              <span v-else>{{ sourceName }}</span>
            </p>
          </header>

          <section id="skill-install" class="skills-detail__panel">
            <h2>{{ t('registryUi.skills.detail.installation') }}</h2>
            <template v-if="command">
              <div class="skills-detail__tabs" role="tablist" :aria-label="t('registryUi.skills.detail.installation')" @keydown="onTabKeydown">
                <button
                  id="skills-command-tab"
                  type="button"
                  role="tab"
                  aria-controls="skills-install-panel"
                  :aria-selected="activeTab === 'command'"
                  :tabindex="activeTab === 'command' ? 0 : -1"
                  :class="{ 'skills-detail__tab--active': activeTab === 'command' }"
                  @click="selectTab('command')"
                >{{ t('registryUi.skills.detail.command') }}</button>
                <button
                  id="skills-prompt-tab"
                  type="button"
                  role="tab"
                  aria-controls="skills-install-panel"
                  :aria-selected="activeTab === 'prompt'"
                  :tabindex="activeTab === 'prompt' ? 0 : -1"
                  :class="{ 'skills-detail__tab--active': activeTab === 'prompt' }"
                  @click="selectTab('prompt')"
                >{{ t('registryUi.skills.detail.prompt') }}</button>
              </div>
              <div
                id="skills-install-panel"
                role="tabpanel"
                tabindex="0"
                :aria-labelledby="'skills-' + activeTab + '-tab'"
                class="skills-detail__install-body"
              >
                <AppMultiSelect
                  :model-value="targets"
                  :auto-selected="chooseInTerminal"
                  :auto-option="terminalOption"
                  :label="t('registryUi.skills.chooseAgents')"
                  :options="options"
                  :automatic-help="t('registryUi.skills.chooseInTerminalHelp')"
                  @update:model-value="updateTargets"
                  @update:auto-selected="chooseInTerminal = $event"
                />
                <CommandSnippet
                  :command="activeTab === 'command' ? command : prompt"
                  :label="activeTab === 'command' ? t('registryUi.skills.detail.command') : t('registryUi.skills.detail.prompt')"
                  kind="add"
                />
                <p v-if="skillNeedsManualSelection(skill)" class="skills-detail__hint">
                  {{ t('registryUi.skills.manualSkillSelection') }}
                </p>
                <p class="skills-detail__hint">{{ t('registryUi.skills.checkedBeforeInstall') }}</p>
              </div>
            </template>
            <p v-else class="skills-detail__hint">
              {{ t('registryUi.skills.detail.exactNameUnavailable') }}
            </p>
          </section>

          <section class="skills-detail__panel skills-detail__document">
            <h2>SKILL.md</h2>
            <p v-if="documentLoading" role="status" class="skills-detail__hint">{{ t('registryUi.skills.detail.loadingDocument') }}</p>
            <template v-else-if="resolvedDocument?.status === 'resolved' && resolvedDocument.markdown">
              <p class="skills-detail__hint skills-detail__document-caption">{{ t('registryUi.skills.detail.sourceDocumentNotice') }}</p>
              <SkillMarkdown :markdown="resolvedDocument.markdown" />
            </template>
            <div v-else class="skills-detail__document-empty">
              <p>{{ t('registryUi.skills.detail.documentUnavailable') }}</p>
              <button v-if="documentFailed" type="button" class="button button--secondary" @click="retryDocument">
                {{ t('registryUi.skills.retry') }}
              </button>
              <a v-if="sourceUrl" :href="sourceUrl" target="_blank" rel="noopener noreferrer">
                {{ t('registryUi.skills.detail.viewPublisher') }} <span aria-hidden="true">↗</span>
              </a>
            </div>
          </section>

          <section class="skills-detail__panel">
            <h2>{{ t('registryUi.skills.detail.security') }}</h2>
            <SkillSpectorAssessment
              :subject="skill.source.kind === 'github' && skill.source.repository
                ? { kind: 'skill', repository: skill.source.repository, name: skill.skillNameHint }
                : undefined"
              :applicable="skill.source.kind === 'github' && Boolean(skill.source.repository)"
              :name="skill.title"
              scope="skill"
            />
          </section>
        </main>

        <aside class="skills-detail__sidebar">
          <div class="skills-detail__panel skills-detail__facts">
            <dl>
              <div v-if="listedSkill">
                <dt>{{ t('registryUi.skills.detail.installs') }}</dt>
                <dd>{{ n(listedSkill.metrics.installs) }}</dd>
                <p class="skills-detail__hint">{{ t('registryUi.skills.metric.' + listedSkill.metrics.window, { count: n(listedSkill.metrics.installs) }) }}</p>
              </div>
              <div>
                <dt>{{ t('registryUi.skills.detail.source') }}</dt>
                <dd>
                  <a v-if="sourceUrl" :href="sourceUrl" target="_blank" rel="noopener noreferrer">{{ sourceName }} ↗</a>
                  <span v-else>{{ sourceName }}</span>
                </dd>
              </div>
              <div v-if="resolvedDocument?.commit">
                <dt>{{ t('registryUi.skills.detail.revision') }}</dt>
                <dd><code>{{ resolvedDocument.commit.slice(0, 12) }}</code></dd>
              </div>
            </dl>
            <p v-if="listedSkill" class="skills-detail__hint">{{ t('registryUi.skills.detail.metricsNotice') }}</p>
          </div>
        </aside>
      </div>
    </div>
  </div>
</template>

<i18n locale="en" lang="json" src="../../features/skills-catalog/locales/en.json" />
<i18n locale="ru" lang="json" src="../../features/skills-catalog/locales/ru.json" />
<i18n locale="uk" lang="json" src="../../features/skills-catalog/locales/uk.json" />
<i18n locale="zh" lang="json" src="../../features/skills-catalog/locales/zh.json" />
<i18n locale="es" lang="json" src="../../features/skills-catalog/locales/es.json" />
<i18n locale="hi" lang="json" src="../../features/skills-catalog/locales/hi.json" />
<i18n locale="ar" lang="json" src="../../features/skills-catalog/locales/ar.json" />
<i18n locale="pt" lang="json" src="../../features/skills-catalog/locales/pt.json" />
<i18n locale="fr" lang="json" src="../../features/skills-catalog/locales/fr.json" />

<style lang="scss" src="../../features/skills-catalog/detail.scss"></style>
