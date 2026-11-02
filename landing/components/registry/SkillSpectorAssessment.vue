<script setup lang="ts">
import type {
  SkillSpectorRecord,
  SkillSpectorSubject,
  SkillSpectorFinding,
  SkillSpectorOccurrence,
} from '~/types/skillSpector';
import {
  skillSpectorAssessment,
  hasIncompleteSkillSpectorFindings,
  groupSkillSpectorReasons,
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
  { applicable: true, subject: undefined },
);
const detailsOpen = ref(false);
const reportRoot = ref<HTMLDetailsElement | null>(null);
const reportTrigger = ref<HTMLElement | null>(null);
const reportDialog = ref<HTMLDialogElement | null>(null);
const reasonContextsOpen = ref(new Set<string>());
const onToggle = async (event: Event) => {
  if (event.target !== event.currentTarget) return;
  const open = (event.currentTarget as HTMLDetailsElement).open;
  detailsOpen.value = open;
  if (!open) {
    reasonContextsOpen.value.clear();
    if (reportDialog.value?.open) reportDialog.value.close();
    return;
  }
  await nextTick();
  if (reportDialog.value && !reportDialog.value.open) reportDialog.value.showModal();
};
const closeReport = () => reportDialog.value?.close();
const onDialogClose = () => {
  detailsOpen.value = false;
  reasonContextsOpen.value.clear();
  if (reportRoot.value) reportRoot.value.open = false;
  nextTick(() => reportTrigger.value?.focus());
};
const onBackdropClick = (event: MouseEvent) => {
  if (event.target === reportDialog.value) closeReport();
};
const { t, n, locale } = useI18n({ useScope: 'local' });
const { report, status: loadedStatus, recoverable, retry: retryReport } = useSkillSpector(() =>
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
const label = computed(() => {
  let key: string = displayStatus.value;
  if (reportStatus.value === 'ready' && key === 'unavailable') key = 'error';
  if (!legacy.value && key === 'noStaticFindings')
    key =
      props.scope === 'plugin' && !assessment.value.inventoryKnown
        ? 'selectionScanned'
        : 'scanComplete';
  // Current records include complete and partial scans, never pending or historical records.
  const outcomes = assessment.value.outcomes;
  const scanned = (outcomes?.complete ?? 0) + (outcomes?.limited ?? 0);
  const waiting = (outcomes?.unscanned ?? 0) + (outcomes?.historical ?? 0);
  if (!legacy.value && key === 'incomplete' && outcomes && !outcomes.failed) {
    if (
      props.scope === 'plugin' &&
      assessment.value.inventoryKnown &&
      (assessment.value.selected ?? 0) > 1
    ) {
      if (waiting)
        return t(`registryUi.skillSpector.${outcomes.limited ? 'waitingWithLimitsProgress' : 'waitingProgress'}`, {
          checked: n(scanned),
          total: n(assessment.value.selected ?? 0),
          waiting: n(waiting),
          limited: n(outcomes.limited),
        });
      if (outcomes.limited)
        return t('registryUi.skillSpector.coverageProgress', {
          checked: n(scanned),
          total: n(assessment.value.selected ?? 0),
          limited: n(outcomes.limited),
        });
    }
    if (outcomes.limited && !waiting) return t('registryUi.skillSpector.outcomelimited');
  }
  return t(`registryUi.skillSpector.${key}`);
});
const scannedSkills = computed(() =>
  assessment.value.selected
    ? assessment.value.outcomes
      ? assessment.value.outcomes.complete + assessment.value.outcomes.limited
      : assessment.value.fullyChecked
    : undefined,
);
const findingsIncomplete = computed(() =>
  assessment.value.records.some(hasIncompleteSkillSpectorFindings),
);
const coverageRows = computed(() =>
  Object.entries(assessment.value.outcomes ?? {}).filter(([, count]) => count > 0),
);
const retryRows = computed(() =>
  Object.entries(assessment.value.retries ?? {}).filter(([, count]) => count > 0),
);
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
const limitationExplanations = computed(() => {
  if (!assessment.value.outcomes?.limited) return [];
  const codes = new Set(reasons.value.map((reason) => reason.code));
  return [
    ['reference_missing', 'referenceMissing'],
    ['referenced_uninspected', 'referenceUninspectedArtifact'],
    ['static_parse_limit', 'staticParseLimit'],
    ['analyzer_incomplete', 'analyzerIncomplete'],
    ['acquisition_transfer_limit', 'acquisitionTransferLimit'],
  ]
    .filter(([code]) => codes.has(code!))
    .map(([, key]) => key!);
});
const reasonGroups = computed(() => groupSkillSpectorReasons(reasons.value));
const onReasonToggle = (key: string, event: Event) => {
  if (event.target !== event.currentTarget) return;
  if ((event.currentTarget as HTMLDetailsElement).open) reasonContextsOpen.value.add(key);
  else reasonContextsOpen.value.delete(key);
};
const contextRecords = computed(() =>
  assessment.value.records.filter(
    (record) =>
      record.coverage?.references?.length ||
      record.coverage?.ledger_exceptions?.length ||
      record.coverage?.reference_evidence_truncated,
  ),
);
const reasonPosition = (reason: { path?: unknown; start_line?: unknown; end_line?: unknown }) => {
  const path = typeof reason.path === 'string' ? reason.path : '';
  const start = typeof reason.start_line === 'number' ? reason.start_line : null;
  const end = typeof reason.end_line === 'number' ? reason.end_line : null;
  return `${path}${start ? `:${start}${end && end !== start ? `–${end}` : ''}` : ''}`;
};
const groups = (record: SkillSpectorRecord) => record.findings;
const recordStatus = (record: SkillSpectorRecord) =>
  record.freshness === 'historical' ||
  (props.subject?.kind === 'plugin' && record.source.revision !== props.subject.revision)
    ? 'stale'
    : record.status === 'error'
      ? 'error'
      : record.coverage?.status === 'complete' || (!record.coverage && record.status === 'complete')
        ? hasIncompleteSkillSpectorFindings(record) ||
          record.findings.some(
            (finding) => !finding.review_status || finding.review_status === 'unreviewed',
          )
          ? 'unreviewedFindings'
          : record.finding_count
            ? 'reviewedFindings'
            : 'noStaticFindings'
        : 'incomplete';
const occurrence = (
  finding: SkillSpectorFinding,
  value: SkillSpectorOccurrence,
): SkillSpectorFinding => ({ ...finding, ...value });
const reasonMessage = (reason: { code: string; message: string }) =>
  reason.code === 'skill_removed'
    ? t('registryUi.skillSpector.skillRemoved')
    : reason.code === 'acquisition_transfer_limit'
      ? t('registryUi.skillSpector.acquisitionTransferLimit')
      : reason.message;
const json = (value: unknown) => JSON.stringify(value, null, 2);
</script>

<template>
  <section
    v-if="scope === 'plugin' && applicable === false"
    class="skillspector skillspector--noSkills"
  >
    <strong
      >{{ t('registryUi.skillSpector.title') }} ·
      {{ t('registryUi.skillSpector.noSkills') }}</strong
    >
    <p>{{ t('registryUi.skillSpector.noSkillsDetails') }}</p>
  </section>
  <p
    v-else-if="expanded && reportStatus === 'loading'"
    class="skillspector skillspector--loading"
    role="status"
    aria-busy="true"
  >
    {{ t('registryUi.skillSpector.loadingAdvice') }}
  </p>
  <details
    v-else
    ref="reportRoot"
    class="skillspector"
    :class="`skillspector--${displayStatus}`"
    @toggle="onToggle"
  >
    <summary
      ref="reportTrigger"
      aria-haspopup="dialog"
      :aria-label="`${name}: ${t('registryUi.skillSpector.title')} · ${label}. ${t('registryUi.skillSpector.openReport')}`"
    >
      <span class="skillspector__icon" aria-hidden="true">i</span>
      <span>{{ t('registryUi.skillSpector.title') }} · {{ label }}</span>
      <span class="skillspector__arrow" aria-hidden="true">⌄</span>
    </summary>
    <dialog
      v-if="detailsOpen"
      ref="reportDialog"
      class="skillspector__dialog"
      :aria-label="t('registryUi.skillSpector.detailsFor', { name })"
      @close="onDialogClose"
      @click="onBackdropClick"
    >
      <div class="skillspector__dialog-frame">
        <header class="skillspector__dialog-header">
          <div>
            <h2>{{ t('registryUi.skillSpector.detailsFor', { name }) }}</h2>
            <p>{{ t('registryUi.skillSpector.statusExplanation') }}</p>
          </div>
          <button
            type="button"
            class="skillspector__dialog-close"
            @click="closeReport"
          >{{ t('registryUi.skillSpector.closeReport') }}</button>
        </header>
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
              `registryUi.skillSpector.${displayStatus === 'unsupported' ? (scope === 'skill' ? 'unsupportedSourceDetails' : 'unsupportedDetails') : displayStatus === 'pending' ? 'pendingDetails' : displayStatus === 'unavailable' ? 'checkUnavailableDetails' : 'absent'}`,
            )
          }}
        </p>
        <button
          v-if="reportStatus === 'unavailable' && recoverable"
          type="button"
          class="skillspector__retry"
          @click="retryReport"
        >
          {{ t('registryUi.skillSpector.retryLoad') }}
        </button>
        <p v-if="scannedSkills !== undefined">
          {{
            t(
              `registryUi.skillSpector.${assessment.inventoryKnown ? 'fullyChecked' : 'selectedFullyChecked'}`,
              {
                checked: n(scannedSkills),
                total: n(assessment.selected ?? 0),
              },
            )
          }}
        </p>
        <section v-if="assessment.pendingIds?.length" class="skillspector__waiting">
          <h3>{{ t('registryUi.skillSpector.awaitingList') }}</h3>
          <p>{{ t('registryUi.skillSpector.awaitingExplanation') }}</p>
          <ul>
            <li v-for="identifier in assessment.pendingIds" :key="identifier">
              <code :title="identifier">{{ identifier.split('/').at(-1) }}</code>
            </li>
          </ul>
        </section>
        <dl v-if="coverageRows.length" class="skillspector__outcomes">
          <div v-for="[outcome, count] in coverageRows" :key="outcome">
            <dt>{{
              t(`registryUi.skillSpector.${outcome === 'complete' ? 'scanComplete' : `outcome${outcome}`}`)
            }}</dt>
            <dd>{{ n(count) }}</dd>
          </div>
        </dl>
        <p v-for="explanation in limitationExplanations" :key="explanation">
          {{ t(`registryUi.skillSpector.${explanation}`) }}
        </p>
        <p v-for="[retry, count] in retryRows" :key="retry">
          {{ t(`registryUi.skillSpector.retry${retry}`, { count: n(count) }) }}
        </p>
        <p>{{ t('registryUi.skillSpector.engine') }}</p>
        <p v-if="legacy">{{ t('registryUi.skillSpector.legacyCoverage') }}</p>
        <p v-if="assessment.outOfDate">{{ t('registryUi.skillSpector.older') }}</p>
        <p v-if="scope === 'skill' && assessment.records.length">
          {{ t('registryUi.skillSpector.unverified') }}
        </p>
      </section>
      <section v-if="assessment.records.length" class="skillspector__signals">
        <strong>{{ t('registryUi.skillSpector.findingsAndConfidence') }}</strong>
        <p>{{ t('registryUi.skillSpector.signalExplanation') }}</p>
        <p v-if="assessment.groupCount !== undefined && !findingsIncomplete">
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
            !findingsIncomplete &&
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
                    <code v-if="'path' in reason && reason.path">
                      · {{ reasonPosition(reason) }}</code
                    >
                  </li>
                </ul>
              </div>
            </component>
          </li>
        </ul>
      </section>
      <section
        v-if="
          reasons.length ||
          contextRecords.length ||
          assessment.status === 'incomplete' ||
          (legacy && assessment.coverage?.pending)
        "
        class="skillspector__coverage"
      >
        <strong>{{ t('registryUi.skillSpector.coverageContext') }}</strong>
        <ul class="skillspector__reason-groups">
          <li v-for="group in reasonGroups" :key="group.key">
            <code>{{ group.code }}</code
            >: {{ reasonMessage(group) }}
            <details
              class="skillspector__reason-contexts"
              @toggle="onReasonToggle(group.key, $event)"
            >
              <summary>
                {{
                  t('registryUi.skillSpector.reasonContexts', { count: n(group.contexts.length) })
                }}
              </summary>
              <pre v-if="reasonContextsOpen.has(group.key)">{{ json(group.contexts) }}</pre>
            </details>
          </li>
        </ul>
        <template v-if="contextRecords.length">
          <p>{{ t('registryUi.skillSpector.referenceScope') }}</p>
          <details
            v-for="record in contextRecords"
            :key="record.id"
            class="skillspector__context-record"
          >
            <summary>
              {{ record.skill_name || record.source.path }} ·
              {{ t('registryUi.skillSpector.referenceContext') }}
            </summary>
            <p v-if="record.coverage?.reference_evidence_truncated">
              {{
                t('registryUi.skillSpector.contextTruncated', {
                  references: record.coverage.references?.length,
                  referencesTotal: record.coverage.references_total,
                  diagnostics: record.coverage.ledger_exceptions?.length,
                  diagnosticsTotal: record.coverage.ledger_exceptions_total,
                })
              }}
            </p>
            <ul class="skillspector__references">
              <li
                v-for="(reference, referenceIndex) in record.coverage?.references"
                :key="referenceIndex"
              >
                <a
                  :href="
                    skillSpectorFindingUrl(record, {
                      path: reference.source_path,
                      line: reference.line,
                    })
                  "
                  target="_blank"
                  rel="noreferrer"
                  ><code
                    >{{ reference.source_path }}:{{ reference.line }}:{{ reference.column }}</code
                  >
                  ↗</a
                >
                <p>
                  {{ t('registryUi.skillSpector.referenceResolution') }}:
                  <code
                    >{{
                      reference.status === 'uninspected'
                        ? t('registryUi.skillSpector.referenceDirectoryStatus')
                        : reference.status
                    }} · {{ reference.disposition }} ·
                    {{ reference.reference_kind }}</code
                  >
                </p>
                <p>
                  {{ t('registryUi.skillSpector.referenceTarget') }}:
                  <code>{{
                    reference.target_path ?? t('registryUi.skillSpector.referenceUnresolved')
                  }}</code>
                </p>
                <pre>{{ reference.evidence }}</pre>
              </li>
            </ul>
            <details
              v-if="record.coverage?.ledger_exceptions?.length"
              class="skillspector__diagnostics"
            >
              <summary>
                {{
                  t('registryUi.skillSpector.coverageDiagnostics', {
                    count: record.coverage.ledger_exceptions.length,
                  })
                }}
              </summary>
              <ul>
                <li
                  v-for="(diagnostic, diagnosticIndex) in record.coverage.ledger_exceptions"
                  :key="diagnosticIndex"
                >
                  <code
                    >{{ diagnostic.reason_code
                    }}<template v-if="diagnostic.path">
                      · {{ reasonPosition(diagnostic) }}</template
                    ></code
                  >
                  <pre>{{ diagnostic.message }}</pre>
                  <details>
                    <summary>{{ t('registryUi.skillSpector.technicalReport') }}</summary>
                    <pre>{{ json(diagnostic) }}</pre>
                  </details>
                </li>
              </ul>
            </details>
          </details>
        </template>
        <p v-if="legacy && assessment.coverage?.pending">
          {{ t('registryUi.skillSpector.pendingDetails') }}
        </p>
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
      </div>
    </dialog>
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
.skillspector__dialog {
  position: fixed;
  inset: 0;
  width: min(940px, calc(100vw - 28px));
  max-width: none;
  max-height: min(88dvh, 920px);
  margin: auto;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--line-strong);
  border-radius: 14px;
  background: var(--surface-solid);
  color: var(--text);
  box-shadow: 0 25px 80px rgba(0, 0, 0, 0.46);
}
.skillspector__dialog::backdrop {
  background: rgba(2, 5, 13, 0.76);
  backdrop-filter: blur(3px);
}
.skillspector__dialog-frame {
  display: flex;
  flex-direction: column;
  max-height: min(88dvh, 920px);
}
.skillspector__dialog-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 22px;
  border-bottom: 1px solid var(--line);
}
.skillspector__dialog-header h2 {
  margin: 0;
  font-size: 1.25rem;
  color: var(--text);
}
.skillspector__dialog-header p {
  margin: 6px 0 0;
  color: var(--subtle);
  max-width: 65ch;
  font-size: 0.85rem;
}
.skillspector__dialog-close {
  flex-shrink: 0;
  min-height: 38px;
  padding: 6px 12px;
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  font: inherit;
  font-weight: 700;
}
.skillspector__dialog-close:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 2px;
}
.skillspector__details {
  min-height: 0;
  padding: 20px 24px 28px;
  overflow-y: auto;
  overscroll-behavior: contain;
  color: var(--text);
  font-size: 0.875rem;
  overflow-wrap: anywhere;
}
.skillspector__details p {
  margin: 6px 0;
}
.skillspector__retry {
  min-height: 36px;
  margin-top: 8px;
  padding: 6px 12px;
  border: 1px solid var(--cyan);
  border-radius: 6px;
  background: transparent;
  color: var(--cyan);
  cursor: pointer;
  font: inherit;
  font-weight: 700;
}
.skillspector__retry:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 3px;
}
.skillspector__waiting h3 {
  margin: 0 0 4px;
  font-size: 1rem;
}
.skillspector__waiting ul {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  list-style: none;
  padding: 0;
  margin: 12px 0;
}
.skillspector__waiting li {
  padding: 5px 9px;
  border-radius: 7px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
}
.skillspector__outcomes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
  margin: 14px 0;
}
.skillspector__outcomes > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 9px;
  background: var(--surface);
}
.skillspector__outcomes dd {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 750;
  font-variant-numeric: tabular-nums;
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
  margin-top: 18px;
  padding-top: 18px;
}
.skillspector__record,
.skillspector__context-record,
.skillspector__reason-contexts,
.skillspector__diagnostics,
.skillspector__technical {
  margin-block: 8px;
  padding: 10px 14px;
  border: 1px solid var(--line-strong);
  border-radius: 9px;
  background: var(--surface);
}
.skillspector__record > summary,
.skillspector__context-record > summary,
.skillspector__reason-contexts > summary,
.skillspector__diagnostics > summary,
.skillspector__technical > summary {
  cursor: pointer;
  font-weight: 700;
  padding: 4px 0;
}
.skillspector__record[open],
.skillspector__context-record[open],
.skillspector__reason-contexts[open],
.skillspector__diagnostics[open],
.skillspector__technical[open] {
  border-color: var(--cyan);
}
@media (max-width: 600px) {
  .skillspector__dialog {
    width: calc(100vw - 12px);
    max-height: calc(100dvh - 12px);
  }
  .skillspector__dialog-frame { max-height: calc(100dvh - 12px); }
  .skillspector__dialog-header { padding: 14px; }
  .skillspector__details { padding: 14px; }
  .skillspector__dialog-header h2 { font-size: 1.05rem; }
}
.skillspector__disclaimer {
  color: var(--subtle);
}
@media (max-width: 600px), (pointer: coarse) {
  .skillspector > summary,
  .skillspector__record-summary,
  .skillspector__retry {
    min-height: 44px;
  }
}
</style>
