<script setup lang="ts">
import type { SkillListing, SkillsView } from '~/types/skillsCatalog';
import { skillClients } from '~/data/skillClients';
import {
  externalListingUrl,
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
  const raw = route.query.id;
  return typeof raw === 'string' && raw.startsWith('skills-sh:') && raw.length <= 1024
    ? raw
    : '';
});
function searchTerm() {
  const explicit = route.query.q;
  if (typeof explicit === 'string' && explicit.trim().length >= 2 && explicit.length <= 200)
    return explicit.trim();
  return listingId.value.split(':').at(-1)?.slice(0, 200) ?? '';
}
const search = ref(searchTerm());
const view = ref<SkillsView>('popular');
watch(() => route.query, () => { search.value = searchTerm(); });
const { items, loading, failed, retry } = useSkillsCatalog(search, view);
const skill = computed(() =>
  items.value.find((item) => item.id === listingId.value) ??
  (selectedSkill.value?.id === listingId.value ? selectedSkill.value : undefined),
);
const repositoryUrl = computed(() => skill.value ? skillRepositoryUrl(skill.value) : undefined);
const publisherUrl = computed(() => skill.value ? skillPublisherUrl(skill.value) : undefined);
const listingUrl = computed(() => skill.value ? externalListingUrl(skill.value.providerUrl) : undefined);
const sourceUrl = computed(() => repositoryUrl.value ?? publisherUrl.value);
const sourceName = computed(() =>
  skill.value?.source.repository ?? publisherUrl.value?.replace('https://', '') ?? t('registryUi.skills.externalSource'),
);

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

      <div v-if="!skill && loading" class="skills-detail__panel" role="status">
        {{ t('registryUi.skills.loading') }}
      </div>
      <div v-else-if="!skill" class="skills-detail__panel skills-detail__empty">
        <h1>{{ t('registryUi.skills.detail.notFound') }}</h1>
        <p>{{ t(failed ? 'registryUi.skills.unavailableHelp' : 'registryUi.skills.detail.notFoundHelp') }}</p>
        <div class="skills-detail__actions">
          <button v-if="failed" type="button" class="button button--secondary" @click="retry">
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
              <div class="skills-detail__tabs" role="tablist" :aria-label="t('registryUi.skills.detail.installation')">
                <button type="button" role="tab" :aria-selected="activeTab === 'command'" :class="{ 'skills-detail__tab--active': activeTab === 'command' }" @click="activeTab = 'command'">
                  {{ t('registryUi.skills.detail.command') }}
                </button>
                <button type="button" role="tab" :aria-selected="activeTab === 'prompt'" :class="{ 'skills-detail__tab--active': activeTab === 'prompt' }" @click="activeTab = 'prompt'">
                  {{ t('registryUi.skills.detail.prompt') }}
                </button>
              </div>
              <div class="skills-detail__install-body">
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
            <div v-else class="skills-detail__install-body">
              <p>{{ t(sourceUrl ? 'registryUi.skills.detail.exactNameUnavailable' : 'registryUi.skills.unsupportedSource') }}</p>
              <a v-if="listingUrl" :href="listingUrl" target="_blank" rel="noopener noreferrer">
                {{ t('registryUi.skills.viewListing') }} <span aria-hidden="true">↗</span>
              </a>
            </div>
          </section>

          <section class="skills-detail__panel skills-detail__document">
            <h2>SKILL.md</h2>
            <p>{{ t('registryUi.skills.detail.documentNotice') }}</p>
            <a v-if="listingUrl" :href="listingUrl" target="_blank" rel="noopener noreferrer">
              {{ t('registryUi.skills.detail.readInstructions') }} <span aria-hidden="true">↗</span>
            </a>
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
              <div>
                <dt>{{ t('registryUi.skills.detail.installs') }}</dt>
                <dd>{{ n(skill.metrics.installs) }}</dd>
                <p class="skills-detail__hint">{{ t('registryUi.skills.metric.' + skill.metrics.window, { count: n(skill.metrics.installs) }) }}</p>
              </div>
              <div>
                <dt>{{ t('registryUi.skills.detail.source') }}</dt>
                <dd>
                  <a v-if="sourceUrl" :href="sourceUrl" target="_blank" rel="noopener noreferrer">{{ sourceName }} ↗</a>
                  <span v-else>{{ sourceName }}</span>
                </dd>
              </div>
              <div>
                <dt>{{ t('registryUi.skills.detail.listing') }}</dt>
                <dd>
                  <a v-if="listingUrl" :href="listingUrl" target="_blank" rel="noopener noreferrer">
                    skills.sh ↗
                  </a>
                </dd>
              </div>
            </dl>
            <p class="skills-detail__hint">{{ t('registryUi.skills.detail.metricsNotice') }}</p>
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
