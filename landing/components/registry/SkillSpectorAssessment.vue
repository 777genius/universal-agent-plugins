<script setup lang="ts">
import type { SkillSpectorRecord, SkillSpectorSubject } from '~/types/skillSpector';
import {
  skillSpectorAssessment,
  skillSpectorFindingLocation,
  skillSpectorFindingUrl,
  skillSpectorSourceUrl,
} from '~/utils/skillSpector';
import { formatSecurityDate } from '~/utils/securityPresentation';

const props = defineProps<{
  subject?: SkillSpectorSubject;
  name: string;
  scope: 'plugin' | 'skill';
}>();
const { t, n, locale } = useI18n({ useScope: 'local' });
const { report, status: reportStatus } = useSkillSpector(() => props.subject);
const assessment = computed(() => skillSpectorAssessment(report.value, props.subject));
const displayStatus = computed(() =>
  reportStatus.value === 'ready' ? assessment.value.status : reportStatus.value,
);
const visualStatus = computed(() =>
  props.scope === 'skill' && displayStatus.value === 'noFindings'
    ? 'historical'
    : displayStatus.value,
);
const label = computed(() => {
  let status = t(`registryUi.skillSpector.${displayStatus.value}`, {
    count: n(assessment.value.findingCount),
  });
  if (['partial', 'error'].includes(displayStatus.value) && assessment.value.findingCount) {
    status += ` · ${t('registryUi.skillSpector.findingsCount', { count: n(assessment.value.findingCount) })}`;
  }
  return props.scope === 'skill' &&
    assessment.value.records.length &&
    assessment.value.status !== 'historical'
    ? `${t('registryUi.skillSpector.historical')}: ${status}`
    : status;
});
function recordLabel(record: SkillSpectorRecord, showCount = false) {
  const status =
    record.status === 'complete'
      ? record.finding_count
        ? 'findingsCount'
        : 'noFindings'
      : record.status;
  let label = t(`registryUi.skillSpector.${status}`, { count: n(record.finding_count) });
  if (showCount && record.status !== 'complete' && record.finding_count) {
    label += ` · ${t('registryUi.skillSpector.findingsCount', { count: n(record.finding_count) })}`;
  }
  return label;
}
const coverageLimits = computed(() => {
  const messages: Record<string, string> = {
    static_parse_limit: 'staticParseLimit',
    reference_missing: 'referenceMissing',
    analyzer_incomplete: 'analyzerIncomplete',
  };
  return [
    ...new Set(
      assessment.value.records.flatMap((record) =>
        record.errors.flatMap((error) =>
          Object.hasOwn(messages, error.code) ? [messages[error.code]!] : [],
        ),
      ),
    ),
  ];
});
</script>

<template>
  <details class="skillspector" :class="`skillspector--${visualStatus}`">
    <summary :aria-label="`${t('registryUi.skillSpector.detailsFor', { name })}: ${label}`">
      <span class="skillspector__icon" aria-hidden="true">{{
        visualStatus === 'noFindings' ? '✓' : 'i'
      }}</span>
      <span
        >SkillSpector ·
        <template v-if="scope === 'plugin'"
          >{{ t('registryUi.skillSpector.selectedSkills') }}: </template
        >{{ label }}</span
      >
      <span class="skillspector__arrow" aria-hidden="true">⌄</span>
    </summary>
    <div class="skillspector__details" :aria-busy="reportStatus === 'loading'">
      <strong>{{
        t(`registryUi.skillSpector.${scope === 'plugin' ? 'pluginScope' : 'skillScope'}`)
      }}</strong>
      <p v-if="assessment.coverage?.pending">
        {{
          t('registryUi.skillSpector.pendingCoverage', {
            checked: n(assessment.coverage.reported),
            total: n(assessment.coverage.selected),
          })
        }}
      </p>
      <p v-if="reportStatus !== 'ready'">
        {{ t(`registryUi.skillSpector.${reportStatus}Details`) }}
      </p>
      <p v-else-if="!assessment.records.length">{{ t('registryUi.skillSpector.absent') }}</p>
      <template v-else>
        <p>{{ t('registryUi.skillSpector.scanner', { version: report!.scanner.version }) }}</p>
        <p v-if="assessment.status === 'partial' || assessment.status === 'error'">
          {{ t('registryUi.skillSpector.incomplete') }}
        </p>
        <ul v-if="coverageLimits.length" class="skillspector__coverage">
          <li v-for="limit in coverageLimits" :key="limit">
            {{ t(`registryUi.skillSpector.${limit}`) }}
          </li>
        </ul>
        <p v-if="assessment.outOfDate">{{ t('registryUi.skillSpector.older') }}</p>
        <p v-if="scope === 'skill'">{{ t('registryUi.skillSpector.unverified') }}</p>
        <ul class="skillspector__records">
          <li v-for="record in assessment.records" :key="record.id">
            <component
              :is="assessment.records.length > 1 ? 'details' : 'div'"
              :class="{ skillspector__record: assessment.records.length > 1 }"
            >
              <summary v-if="assessment.records.length > 1" class="skillspector__record-summary">
                <span
                  >{{
                    record.skill_name ||
                    record.source.path.split('/').at(-1) ||
                    record.source.repository
                  }}
                  · {{ recordLabel(record, true) }}</span
                >
                <span class="skillspector__arrow" aria-hidden="true">⌄</span>
              </summary>
              <div :class="{ 'skillspector__record-body': assessment.records.length > 1 }">
                <a :href="skillSpectorSourceUrl(record)" target="_blank" rel="noreferrer"
                  >{{ record.source.path || record.skill_name || record.source.repository }} ↗</a
                >
                <p v-if="record.scanned_at">
                  {{
                    t('registryUi.skillSpector.checked', {
                      date: formatSecurityDate(record.scanned_at, locale),
                    })
                  }}
                </p>
                <p>
                  {{ t('registryUi.skillSpector.revision') }}:
                  <code>{{ record.source.revision }}</code>
                </p>
                <p>{{ recordLabel(record) }}</p>
                <p v-if="record.findings_truncated">
                  {{
                    t('registryUi.skillSpector.truncated', {
                      shown: n(record.findings.length),
                      total: n(record.finding_count),
                    })
                  }}
                </p>
                <ul
                  v-if="record.findings.length"
                  class="skillspector__findings"
                  :aria-label="t('registryUi.skillSpector.findings')"
                >
                  <li v-for="(finding, index) in record.findings" :key="index">
                    <code>{{ finding.rule }} · {{ finding.severity }}</code>
                    <p>{{ finding.message }}</p>
                    <a
                      v-if="skillSpectorFindingUrl(record, finding)"
                      :href="skillSpectorFindingUrl(record, finding)"
                      target="_blank"
                      rel="noreferrer"
                      ><code>{{ skillSpectorFindingLocation(finding) }}</code> ↗</a
                    >
                    <code v-else>{{ skillSpectorFindingLocation(finding) }}</code>
                  </li>
                </ul>
                <ul v-if="record.errors.length" :aria-label="t('registryUi.skillSpector.errors')">
                  <li v-for="(error, index) in record.errors" :key="index">
                    <code>{{ error.code }}</code
                    >: {{ error.message }}
                  </li>
                </ul>
              </div>
            </component>
          </li>
        </ul>
      </template>
      <p>{{ t('registryUi.skillSpector.selected') }}</p>
      <p v-if="assessment.records.length" class="skillspector__disclaimer">
        {{ t('registryUi.skillSpector.disclaimer') }}
      </p>
    </div>
  </details>
</template>

<i18n locale="en" lang="json" src="../../features/skillspector/locales/en.json" />
<i18n locale="ru" lang="json" src="../../features/skillspector/locales/ru.json" />
<i18n locale="uk" lang="json" src="../../features/skillspector/locales/uk.json" />
<i18n locale="zh" lang="json" src="../../features/skillspector/locales/zh.json" />
<i18n locale="es" lang="json" src="../../features/skillspector/locales/es.json" />
<i18n locale="hi" lang="json" src="../../features/skillspector/locales/hi.json" />
<i18n locale="ar" lang="json" src="../../features/skillspector/locales/ar.json" />
<i18n locale="pt" lang="json" src="../../features/skillspector/locales/pt.json" />
<i18n locale="fr" lang="json" src="../../features/skillspector/locales/fr.json" />

<style scoped lang="scss">
.skillspector {
  margin-top: 8px;
  margin-bottom: 8px;
  min-width: 0;
  color: var(--subtle);
  font-size: 0.75rem;
  line-height: 1.5;
}
.skillspector > summary {
  display: flex;
  align-items: center;
  gap: 6px;
  width: fit-content;
  max-width: 100%;
  min-height: 32px;
  cursor: pointer;
  font-weight: 750;
  list-style: none;
}
.skillspector > summary::-webkit-details-marker {
  display: none;
}
.skillspector > summary:focus-visible,
.skillspector__record-summary:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 3px;
  border-radius: 4px;
}
.skillspector__icon {
  display: inline-grid;
  place-items: center;
  flex: 0 0 16px;
  height: 16px;
  border: 1px solid currentcolor;
  border-radius: 50%;
  font-size: 0.6875rem;
}
.skillspector__arrow {
  flex-shrink: 0;
  margin-inline-start: 2px;
}
.skillspector[open] > summary > .skillspector__arrow,
.skillspector__record[open] > summary > .skillspector__arrow {
  transform: rotate(180deg);
}
.skillspector--noFindings > summary {
  color: var(--cyan);
}
.skillspector--findingsCount > summary,
.skillspector--partial > summary {
  color: #f5c451;
}
.skillspector__details {
  margin-top: 8px;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
  max-height: 28rem;
  overflow-y: auto;
}
.skillspector__details p {
  margin: 6px 0;
}
.skillspector__details a {
  color: var(--cyan);
}
.skillspector__details code {
  font-size: 0.75rem;
  white-space: normal;
}
.skillspector__record-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 32px;
  cursor: pointer;
  list-style: none;
  font-weight: 600;
}
.skillspector__record-summary::-webkit-details-marker {
  display: none;
}
.skillspector__record-body {
  margin-top: 8px;
}
.skillspector__records {
  list-style: none;
  padding: 0;
  margin: 12px 0;
}
.skillspector__records > li + li {
  border-top: 1px solid var(--line);
  margin-top: 12px;
  padding-top: 12px;
}
.skillspector__coverage,
.skillspector__findings {
  padding-inline-start: 16px;
}
.skillspector__findings > li {
  margin: 8px 0;
}
.skillspector__disclaimer {
  color: var(--subtle);
}
@media (max-width: 600px), (pointer: coarse) {
  .skillspector > summary,
  .skillspector__record-summary {
    min-height: 44px;
  }
}
</style>
