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

const props = defineProps<{ plugin: RegistryPlugin }>();
const assessment = computed(() => props.plugin.security!);
const groups = computed(() => groupSecurityFindings(assessment.value));
const heading = computed(() => securityAssessmentHeading(assessment.value));
const label = computed(() => securityAssessmentLabel(assessment.value));
const revision = computed(() => shortRevision(props.plugin.source.revision));

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
        <p class="security-review__eyebrow">Automated security review</p>
        <h2 id="security-review-title">{{ heading }}</h2>
      </div>
      <span class="security-review__status">{{ label }}</span>
    </div>

    <p class="security-review__scope">
      LintAI {{ assessment.scanner.version }} checked the exact indexed revision
      <code>{{ revision }}</code> on {{ formatSecurityDate(assessment.generated_at) }}. This result
      applies only to those package files.
    </p>
    <p class="security-review__freshness">
      A newer upstream revision is different code. It must be indexed and checked again; the CLI
      will not reuse this result for changed files.
    </p>

    <div v-if="groups.installer.length" class="security-review__group">
      <h3>Things to review before installing</h3>
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
      <summary>Repository maintenance notes ({{ groups.maintainer.length }})</summary>
      <p>
        These findings concern the author's repository automation. They are useful hardening advice,
        but those workflows are not run by the installer.
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
      Showing {{ assessment.findings.length }} of {{ assessment.counts.total }} findings. The signed
      public summary is size-limited; the totals include every finding from the scan.
    </p>
    <p v-if="assessment.counts.total === 0" class="security-review__empty">
      The automated rules did not detect a blocking pattern in the checked files.
    </p>
    <p class="security-review__disclaimer">
      Automated checks reduce risk; they do not prove that a plugin is safe. Review the source and
      permissions before installing software you do not trust.
    </p>
  </section>
</template>
