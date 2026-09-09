import type { RegistryPlugin } from '../types/registry';
import type { SecurityFinding } from '../types/security';

export type SecurityFindingAudience = 'installer' | 'maintainer';

export interface SecurityFindingGroups {
  installer: SecurityFinding[];
  maintainer: SecurityFinding[];
  hidden: number;
}

export interface SecurityTooltipPresentation {
  label: string;
  scope: string;
  findings: SecurityFinding[];
  remaining: number;
  disclaimer: string;
}

export type SecurityTranslator = (
  key: string,
  params?: Record<string, string | number>,
  plural?: number,
) => string;

type SecurityAssessment = NonNullable<RegistryPlugin['security']>;

// These findings describe repository automation. The workflows are not copied
// to an agent or executed during installation, so present them as maintainer
// hardening notes without hiding them from the public report.
const maintainerFindingCodes = new Set(['SEC324', 'SEC325', 'SEC326', 'SEC327', 'SEC328']);

export function securityFindingAudience(finding: SecurityFinding): SecurityFindingAudience {
  return maintainerFindingCodes.has(finding.code) ? 'maintainer' : 'installer';
}

export function groupSecurityFindings(assessment: SecurityAssessment): SecurityFindingGroups {
  const groups: SecurityFindingGroups = { installer: [], maintainer: [], hidden: 0 };
  for (const finding of assessment.findings) {
    groups[securityFindingAudience(finding)].push(finding);
  }
  groups.hidden = Math.max(0, assessment.counts.total - assessment.findings.length);
  return groups;
}

export function securityAssessmentLabel(
  assessment: SecurityAssessment,
  translate?: SecurityTranslator,
): string {
  if (assessment.counts.blocking > 0) {
    return (
      translate?.(
        'registryUi.security.blocking',
        { count: assessment.counts.blocking },
        assessment.counts.blocking,
      ) ?? `Automated review: ${formatCount(assessment.counts.blocking, 'blocking finding')}`
    );
  }
  if (assessment.counts.warnings > 0) {
    return (
      translate?.(
        'registryUi.security.notes',
        { count: assessment.counts.warnings },
        assessment.counts.warnings,
      ) ?? `Automated review: ${formatCount(assessment.counts.warnings, 'note')}`
    );
  }
  return translate?.('registryUi.security.clear') ?? 'Automated review: no blocking findings';
}

export function securityAssessmentHeading(
  assessment: SecurityAssessment,
  translate?: SecurityTranslator,
): string {
  const groups = groupSecurityFindings(assessment);
  if (assessment.counts.blocking > 0)
    return translate?.('registryUi.security.review') ?? 'Review before installing';
  if (assessment.counts.warnings === 0)
    return translate?.('registryUi.security.noBlocking') ?? 'No blocking findings detected';
  if (groups.installer.length === 0 && groups.hidden === 0)
    return translate?.('registryUi.security.maintenance') ?? 'Repository maintenance notes';
  return translate?.('registryUi.security.reviewNotes') ?? 'Automated review notes';
}

export function securityAssessmentTooltip(
  plugin: RegistryPlugin,
  translate?: SecurityTranslator,
): SecurityTooltipPresentation {
  const assessment = plugin.security;
  if (!assessment) {
    return {
      label: '',
      scope: '',
      findings: [],
      remaining: 0,
      disclaimer: '',
    };
  }
  const groups = groupSecurityFindings(assessment);
  const revision = shortRevision(
    plugin.source.revision,
    translate?.('registryUi.security.unknown'),
  );
  const preview = [...groups.installer, ...groups.maintainer].slice(0, 2);
  return {
    label: securityAssessmentLabel(assessment, translate),
    scope:
      translate?.('registryUi.security.tooltipScope', {
        version: assessment.scanner.version,
        revision,
      }) ?? `LintAI ${assessment.scanner.version} checked exact indexed revision ${revision}.`,
    findings: preview,
    remaining: Math.max(0, assessment.counts.total - preview.length),
    disclaimer:
      translate?.('registryUi.security.tooltipDisclaimer') ??
      'This static check looks for configured patterns. It does not run the plugin or guarantee safety.',
  };
}

export function shortRevision(revision: string | null, unknown = 'unknown'): string {
  return revision?.slice(0, 12) || unknown;
}

export function formatSecurityDate(value: string, locale = 'en'): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(date);
}

export function securityFindingLocation(finding: SecurityFinding): string {
  const path = finding.path.trim();
  if (!path) return '';
  return finding.line ? `${path}:${finding.line}` : path;
}

function formatCount(count: number, singular: string): string {
  return `${count} ${singular}${count === 1 ? '' : 's'}`;
}
