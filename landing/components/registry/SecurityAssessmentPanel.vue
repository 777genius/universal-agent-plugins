<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import type { SecurityFinding } from '~/types/security';
import {
  formatSecurityDate,
  groupSecurityFindings,
  securityAssessmentHeading,
  securityAssessmentLabel,
  securityFindingLocation,
  shortRevision,
} from '~/utils/securityPresentation';
const { t, locale, n } = useI18n();
const translate = (key: string, params: Record<string, string | number> = {}, plural?: number) =>
  plural === undefined ? t(key, params) : t(key, { ...params, count: n(plural) }, plural);

const props = defineProps<{ plugin: RegistryPlugin }>();
const assessment = computed(() => props.plugin.security!);
const groups = computed(() => groupSecurityFindings(assessment.value));
const heading = computed(() => securityAssessmentHeading(assessment.value, translate));
const label = computed(() => securityAssessmentLabel(assessment.value, translate));
const revision = computed(() =>
  shortRevision(props.plugin.source.revision, t('registryUi.security.unknown')),
);

function findingLocation(finding: SecurityFinding) {
  return securityFindingLocation(finding);
}
</script>

<template>
  <section
    id="security-review"
    class="security-review"
    :class="`security-review--${assessment.outcome}`"
    aria-labelledby="security-review-title"
  >
    <div class="security-review__header">
      <div>
        <p class="security-review__eyebrow">{{ t('registryUi.security.eyebrow') }}</p>
        <h2 id="security-review-title">{{ heading }}</h2>
      </div>
      <span class="security-review__status">{{ label }}</span>
    </div>

    <p class="security-review__scope">
      {{
        t('registryUi.security.scope', {
          version: assessment.scanner.version,
          revision,
          date: formatSecurityDate(assessment.generated_at, locale),
        })
      }}
    </p>
    <p class="security-review__freshness">
      {{ t('registryUi.security.freshness') }}
    </p>

    <div v-if="groups.installer.length" class="security-review__group">
      <h3>{{ t('registryUi.security.before') }}</h3>
      <ul class="security-review__findings">
        <li
          v-for="(finding, index) in groups.installer"
          :key="`${finding.code}:${finding.path}:${finding.line}:${index}`"
        >
          <div class="security-review__finding-heading">
            <code>{{ finding.code }}</code>
            <span v-if="findingLocation(finding)">{{ findingLocation(finding) }}</span>
          </div>
          <p>{{ finding.message }}</p>
        </li>
      </ul>
    </div>

    <details v-if="groups.maintainer.length" class="security-review__maintainer">
      <summary>
        {{ t('registryUi.security.maintenanceCount', { count: n(groups.maintainer.length) }) }}
      </summary>
      <p>
        {{ t('registryUi.security.maintenanceHelp') }}
      </p>
      <ul class="security-review__findings">
        <li
          v-for="(finding, index) in groups.maintainer"
          :key="`${finding.code}:${finding.path}:${finding.line}:${index}`"
        >
          <div class="security-review__finding-heading">
            <code>{{ finding.code }}</code>
            <span v-if="findingLocation(finding)">{{ findingLocation(finding) }}</span>
          </div>
          <p>{{ finding.message }}</p>
        </li>
      </ul>
    </details>

    <p v-if="groups.hidden" class="security-review__truncated">
      {{
        t('registryUi.security.truncated', {
          shown: n(assessment.findings.length),
          total: n(assessment.counts.total),
        })
      }}
    </p>
    <p v-if="assessment.counts.total === 0" class="security-review__empty">
      {{ t('registryUi.security.empty') }}
    </p>
    <p class="security-review__disclaimer">
      {{ t('registryUi.security.disclaimer') }}
    </p>
  </section>
</template>
