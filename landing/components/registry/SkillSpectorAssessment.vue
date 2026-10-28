<script setup lang="ts">
import type {
  SkillSpectorRecord,
  SkillSpectorSubject,
  SkillSpectorFinding,
  SkillSpectorOccurrence,
} from '~/types/skillSpector';
import {
  skillSpectorAssessment,
  skillSpectorFindingLocation,
  skillSpectorFindingUrl,
  skillSpectorSourceUrl,
} from '~/utils/skillSpector';
import { formatSecurityDate } from '~/utils/securityPresentation';

const props = withDefaults(
  defineProps<{
    subject?: SkillSpectorSubject;
    name: string;
    scope: 'plugin' | 'skill';
    expanded?: boolean;
    applicable?: boolean;
  }>(),
  { applicable: true },
);
const { t, n, locale } = useI18n({ useScope: 'local' });
const { report, status: loadedStatus } = useSkillSpector(() =>
  props.applicable === false ? undefined : props.subject,
);
const reportStatus = computed(() => (props.applicable === false ? 'ready' : loadedStatus.value));
const assessment = computed(() => skillSpectorAssessment(report.value, props.subject));
const legacy = computed(() =>
  Boolean(
    report.value &&
    (report.value.schema_version !== 3 ||
      report.value.records.some((record) => record.legacy_format)),
  ),
);
const displayStatus = computed(() => {
  if (props.applicable === false) return 'unsupported';
  if (reportStatus.value !== 'ready') return reportStatus.value;
  const status = assessment.value.status;
  if (legacy.value) {
    if (status === 'noFindings') return props.scope === 'skill' ? 'stale' : 'noStaticFindings';
    if (status === 'findingsCount') return 'unreviewedFindings';
    if (status === 'partial') return 'incomplete';
    if (status === 'error') return 'unavailable';
    if (status === 'historical') return 'stale';
  }
  return status;
});
const label = computed(() => t(`registryUi.skillSpector.${displayStatus.value}`));
const fullyChecked = computed(() => assessment.value.fullyChecked);
const advice = computed(() => {
  if (displayStatus.value === 'confirmedMaterialIssue') return 'adviceConfirmed';
  if (
    ['unreviewedFindings'].includes(displayStatus.value) ||
    (assessment.value.unreviewedCount ?? assessment.value.findingCount) > 0
  )
    return 'adviceReview';
  if (displayStatus.value === 'noStaticFindings') return 'adviceComplete';
  return 'adviceUnknown';
});
const reasons = computed(
  () =>
    assessment.value.reasons ??
    assessment.value.records.flatMap((record) => record.coverage?.reasons ?? record.errors),
);
const groups = (record: SkillSpectorRecord) => record.findings;
const recordStatus = (record: SkillSpectorRecord) =>
  record.status === 'error'
    ? 'unavailable'
    : record.coverage?.status === 'complete' || (!record.coverage && record.status === 'complete')
      ? record.finding_count
        ? record.findings.some(
            (finding) => !finding.review_status || finding.review_status === 'unreviewed',
          )
          ? 'unreviewedFindings'
          : 'reviewedFindings'
        : 'noStaticFindings'
      : 'incomplete';
const occurrence = (
  finding: SkillSpectorFinding,
  value: SkillSpectorOccurrence,
): SkillSpectorFinding => ({ ...finding, ...value });
const reasonMessage = (reason: { code: string; message: string }) =>
  reason.code === 'skill_removed' ? t('registryUi.skillSpector.skillRemoved') : reason.message;
const json = (value: unknown) => JSON.stringify(value, null, 2);
</script>

<template>
  <details
    class="skillspector"
    :class="`skillspector--${displayStatus}`"
    :open="expanded || undefined"
  >
    <summary :aria-label="`${t('registryUi.skillSpector.detailsFor', { name })}: ${label}`">
      <span class="skillspector__icon" aria-hidden="true">i</span>
      <span
        >SkillSpector · {{ label
        }}<span v-if="fullyChecked !== undefined" class="skillspector__summary-coverage">
          ·
          {{
            t(
              `registryUi.skillSpector.${assessment.inventoryKnown ? 'fullyChecked' : 'selectedFullyChecked'}`,
              {
                checked: n(fullyChecked),
                total: n(assessment.selected ?? 0),
              },
            )
          }}</span
        ></span
      >
      <span class="skillspector__arrow" aria-hidden="true">⌄</span>
    </summary>
    <div class="skillspector__details" :aria-busy="reportStatus === 'loading'">
      <section class="skillspector__advice">
        <strong>{{ t('registryUi.skillSpector.installAdvice') }}</strong>
        <p>
          {{ t(`registryUi.skillSpector.${scope === 'plugin' ? 'pluginScope' : 'skillScope'}`) }}
        </p>
        <p>{{ t(`registryUi.skillSpector.${advice}`) }}</p>
        <p v-if="reportStatus !== 'ready'">
          {{ t(`registryUi.skillSpector.${reportStatus}Details`) }}
        </p>
        <p v-else-if="!assessment.records.length">
          {{
            t(
              `registryUi.skillSpector.${displayStatus === 'unsupported' ? 'unsupportedDetails' : displayStatus === 'pending' ? 'pendingDetails' : displayStatus === 'unavailable' ? 'unavailableDetails' : 'absent'}`,
            )
          }}
        </p>
        <p v-if="fullyChecked !== undefined">
          {{
            t(
              `registryUi.skillSpector.${assessment.inventoryKnown ? 'fullyChecked' : 'selectedFullyChecked'}`,
              {
                checked: n(fullyChecked),
                total: n(assessment.selected ?? 0),
              },
            )
          }}
        </p>
        <p v-if="legacy">{{ t('registryUi.skillSpector.legacyCoverage') }}</p>
        <p v-if="assessment.outOfDate">{{ t('registryUi.skillSpector.older') }}</p>
        <p v-if="scope === 'skill' && assessment.records.length">
          {{ t('registryUi.skillSpector.unverified') }}
        </p>
      </section>
      <section v-if="assessment.records.length" class="skillspector__signals">
        <strong>{{ t('registryUi.skillSpector.findingsAndConfidence') }}</strong>
        <p>{{ t('registryUi.skillSpector.signalExplanation') }}</p>
        <p v-if="assessment.groupCount !== undefined">
          {{
            t('registryUi.skillSpector.signalCounts', {
              groups: n(assessment.groupCount),
              signals: n(assessment.findingCount),
              raw: n(assessment.rawCount ?? 0),
            })
          }}
        </p>
        <p
          v-if="
            !assessment.findingCount &&
            assessment.records.some((record) => record.status === 'complete')
          "
        >
          {{ t('registryUi.skillSpector.empty') }}
        </p>
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
                  ·
                  {{ t(`registryUi.skillSpector.${recordStatus(record)}`) }}</span
                >
                <span class="skillspector__arrow" aria-hidden="true">⌄</span>
              </summary>
              <div :class="{ 'skillspector__record-body': assessment.records.length > 1 }">
                <p v-if="record.findings_truncated">
                  {{
                    t(
                      `registryUi.skillSpector.${record.coverage?.grouping_complete === false || legacy ? 'limitedGroups' : 'truncatedGroups'}`,
                      {
                        shown: n(record.findings.length),
                        total: n(record.finding_group_count ?? record.finding_count),
                      },
                    )
                  }}
                </p>
                <ul
                  v-if="record.findings.length"
                  class="skillspector__findings"
                  :aria-label="t('registryUi.skillSpector.findings')"
                >
                  <li
                    v-for="(finding, index) in groups(record)"
                    :key="finding.id || index"
                    :data-finding-id="finding.id"
                  >
                    <code>{{ finding.rule }} · {{ finding.severity }}</code>
                    <p>{{ finding.message }}</p>
                    <p>
                      {{
                        t(
                          `registryUi.skillSpector.${finding.review_status === 'confirmed' && ['HIGH', 'CRITICAL'].includes(finding.severity) ? 'confirmedMaterialIssue' : finding.review_status === 'false_positive' ? 'registryClassification' : finding.review_status === 'accepted' ? 'acceptedSignal' : 'unreviewedSignal'}`,
                        )
                      }}<template
                        v-if="finding.confidence !== undefined && finding.confidence !== null"
                      >
                        ·
                        {{
                          t('registryUi.skillSpector.confidence', {
                            confidence: n(Math.round(finding.confidence * 100)),
                          })
                        }}</template
                      >
                    </p>
                    <p v-if="finding.exception" class="skillspector__classification">
                      {{ t('registryUi.skillSpector.classificationExplanation') }}
                      {{ finding.exception.reason }} <code>{{ finding.exception.id }}</code>
                    </p>
                    <p v-if="finding.raw_count !== undefined">
                      {{
                        t('registryUi.skillSpector.occurrenceCount', {
                          count: n(finding.raw_count),
                        })
                      }}
                    </p>
                    <template
                      v-for="(position, positionIndex) in finding.occurrences?.length
                        ? finding.occurrences
                        : [finding]"
                      :key="positionIndex"
                    >
                      <a
                        v-if="skillSpectorFindingUrl(record, occurrence(finding, position))"
                        :href="skillSpectorFindingUrl(record, occurrence(finding, position))"
                        target="_blank"
                        rel="noreferrer"
                        ><code>{{ skillSpectorFindingLocation(position) }}</code> ↗</a
                      >
                      <code v-else>{{ skillSpectorFindingLocation(position) }}</code>
                      <span
                        v-if="
                          position.end_line !== undefined || position.start_column !== undefined
                        "
                        class="skillspector__position"
                      >
                        {{
                          t('registryUi.skillSpector.position', {
                            end: position.end_line ?? position.line ?? '—',
                            startColumn: position.start_column ?? '—',
                            endColumn: position.end_column ?? '—',
                          })
                        }}</span
                      >
                      <br />
                    </template>
                    <details
                      v-if="
                        (finding.evidence && Object.keys(finding.evidence).length) ||
                        finding.code_snippet ||
                        finding.context ||
                        finding.remediation
                      "
                    >
                      <summary>{{ t('registryUi.skillSpector.evidence') }}</summary>
                      <pre v-if="finding.code_snippet">{{
                        typeof finding.code_snippet === 'string'
                          ? finding.code_snippet
                          : json(finding.code_snippet)
                      }}</pre>
                      <pre v-if="finding.context">{{ json(finding.context) }}</pre>
                      <pre v-if="finding.remediation">{{ json(finding.remediation) }}</pre>
                      <pre v-if="finding.evidence && Object.keys(finding.evidence).length">{{
                        json(finding.evidence)
                      }}</pre>
                    </details>
                    <details v-if="finding.finding_ids?.length">
                      <summary>{{ t('registryUi.skillSpector.findingIds') }}</summary>
                      <pre>{{ finding.finding_ids.join('\n') }}</pre>
                    </details>
                  </li>
                </ul>
                <ul
                  v-if="(record.coverage?.reasons ?? record.errors).length"
                  class="skillspector__record-reasons"
                >
                  <li
                    v-for="(reason, reasonIndex) in record.coverage?.reasons ?? record.errors"
                    :key="reasonIndex"
                  >
                    <code>{{ reason.code }}</code
                    >: {{ reasonMessage(reason) }}
                  </li>
                </ul>
              </div>
            </component>
          </li>
        </ul>
      </section>
      <section
        v-if="reasons.length || assessment.status === 'incomplete' || assessment.coverage?.pending"
        class="skillspector__coverage"
      >
        <strong>{{ t('registryUi.skillSpector.uninspected') }}</strong>
        <p>{{ t('registryUi.skillSpector.incomplete') }}</p>
        <ul>
          <li v-for="(reason, index) in reasons" :key="index">
            <code>{{ reason.code }}</code
            >: {{ reason.message
            }}<template v-if="'path' in reason && reason.path">
              · <code>{{ reason.path }}</code></template
            ><template v-if="'analyzer_id' in reason && reason.analyzer_id">
              · <code>{{ reason.analyzer_id }}</code></template
            >
          </li>
        </ul>
        <p v-if="assessment.coverage?.pending">{{ t('registryUi.skillSpector.pendingDetails') }}</p>
      </section>
      <details v-if="assessment.records.length" class="skillspector__technical">
        <summary>{{ t('registryUi.skillSpector.provenance') }}</summary>
        <p>{{ t('registryUi.skillSpector.scanner', { version: report!.scanner.version }) }}</p>
        <div v-for="record in assessment.records" :key="record.id">
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
            {{ t('registryUi.skillSpector.revision') }}: <code>{{ record.source.revision }}</code>
          </p>
          <p v-if="record.profile">
            {{ t('registryUi.skillSpector.profile') }}:
            <code>{{ record.profile.id }} · {{ record.profile.digest }}</code>
          </p>
          <p v-if="record.retry">
            {{ t('registryUi.skillSpector.retry') }}:
            <code>{{ record.retry.disposition }} · {{ record.retry.reason }}</code
            ><template v-if="record.retry.next_attempt_at">
              · {{ formatSecurityDate(record.retry.next_attempt_at, locale) }}</template
            >
          </p>
          <details>
            <summary>{{ t('registryUi.skillSpector.technicalReport') }}</summary>
            <pre>{{ json(record) }}</pre>
          </details>
        </div>
      </details>
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
.skillspector--noFindings > summary,
.skillspector--noStaticFindings > summary {
  color: var(--cyan);
}
.skillspector--findingsCount > summary,
.skillspector--partial > summary,
.skillspector--incomplete > summary,
.skillspector--unreviewedFindings > summary,
.skillspector--confirmedMaterialIssue > summary {
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
.skillspector__details pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-size: 0.75rem;
}
.skillspector__details section + section,
.skillspector__technical {
  border-top: 1px solid var(--line);
  margin-top: 12px;
  padding-top: 12px;
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
