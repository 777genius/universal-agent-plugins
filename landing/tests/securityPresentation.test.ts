import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import type { RegistryPlugin } from '../types/registry.ts';
import type { SecurityFinding } from '../types/security.ts';
import {
  groupSecurityFindings,
  securityAssessmentHeading,
  securityAssessmentLabel,
  securityAssessmentTooltip,
  securityFindingAudience,
} from '../utils/securityPresentation.ts';

const finding = (code: string, message = 'Review this pattern'): SecurityFinding => ({
  code,
  disposition: 'warning',
  severity: 'warn',
  confidence: 'high',
  category: 'security',
  path: '.github/workflows/test.yml',
  line: 12,
  message,
});

const plugin = (findings: SecurityFinding[], total = findings.length): RegistryPlugin =>
  ({
    display_name: 'Example',
    source: { revision: 'abcdef1234567890abcdef1234567890abcdef12' },
    security: {
      generated_at: '2026-09-05T00:00:00Z',
      scanner: { id: 'lintai', version: '0.1.2' },
      policy: { id: 'agent-plugin-install', version: 1, digest: `sha256:${'1'.repeat(64)}` },
      outcome: total ? 'warnings' : 'no_blocking_findings',
      counts: { blocking: 0, warnings: total, total },
      scanned_files: 4,
      findings,
    },
  }) as RegistryPlugin;

describe('security assessment presentation', () => {
  it('separates repository automation notes from install-relevant findings', () => {
    for (const code of ['SEC324', 'SEC325', 'SEC326', 'SEC327', 'SEC328']) {
      assert.equal(securityFindingAudience(finding(code)), 'maintainer');
    }
    assert.equal(securityFindingAudience(finding('SEC329')), 'installer');
    assert.equal(securityFindingAudience(finding('SEC999')), 'installer');

    const assessment = plugin([finding('SEC324'), finding('SEC329')]).security!;
    const groups = groupSecurityFindings(assessment);
    assert.deepEqual(
      groups.maintainer.map(({ code }) => code),
      ['SEC324'],
    );
    assert.deepEqual(
      groups.installer.map(({ code }) => code),
      ['SEC329'],
    );
  });

  it('uses calm note language without weakening signed blocking outcomes', () => {
    const warning = plugin([finding('SEC324')]).security!;
    assert.equal(securityAssessmentLabel(warning), 'Automated review: 1 note');
    assert.equal(securityAssessmentHeading(warning), 'Repository maintenance notes');

    const blocked = plugin([finding('SEC102')]);
    blocked.security!.outcome = 'blocking_findings';
    blocked.security!.counts = { blocking: 1, warnings: 0, total: 1 };
    assert.equal(
      securityAssessmentLabel(blocked.security!),
      'Automated review: 1 blocking finding',
    );
    assert.equal(securityAssessmentHeading(blocked.security!), 'Review before installing');
  });

  it('binds the tooltip to the exact revision and previews real findings', () => {
    const subject = plugin(
      [finding('SEC329', 'MCP config launches a mutable package at runtime')],
      3,
    );
    const tooltip = securityAssessmentTooltip(subject);
    assert.match(tooltip.scope, /exact indexed revision abcdef123456/);
    assert.equal(tooltip.findings[0]?.code, 'SEC329');
    assert.match(tooltip.findings[0]?.message ?? '', /MCP config launches a mutable package/);
    assert.equal(tooltip.remaining, 2);
    assert.match(tooltip.disclaimer, /not run the plugin or guarantee safety/);
    assert.equal(groupSecurityFindings(subject.security!).hidden, 2);
  });
});
