import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { createI18n, type LocaleMessageDictionary, type VueMessageType } from 'vue-i18n';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import {
  authenticationLabel,
  authenticationState,
  resolveDistribution,
  validationLabel,
} from '../utils/registry.ts';
import type {
  DistributionView,
  RegistryPlugin,
  RegistryDiagnostic,
  ClientID,
} from '../types/registry.ts';

const messages = JSON.parse(
  readFileSync(new URL('../locales/en.json', import.meta.url), 'utf8'),
);
const { t } = createI18n<[LocaleMessageDictionary<VueMessageType>], 'en', false>({ legacy: false, locale: 'en', messages: { en: messages } }).global;
const fixture = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
).plugins[0] as RegistryPlugin;
// Execute each actual UI formatter; translations never feed resolver selection.
const formatters = ['InstallPanel', 'RegistryPluginCard'].map((name) => {
  const source = readFileSync(
    new URL(`../components/registry/${name}.vue`, import.meta.url),
    'utf8',
  );
  const fn = source.slice(
    source.indexOf('function diagnosticText('),
    source.indexOf('const resolution ='),
  );
  return runInNewContext(stripTypeScriptTypes(fn) + '\ndiagnosticText;', { t }) as (
    diagnostic: RegistryDiagnostic | undefined,
    original?: string,
  ) => string;
});
function candidate(id: string, kind: DistributionView['kind']): DistributionView {
  const result = structuredClone(fixture.distributions[0]);
  result.id = id;
  result.kind = kind;
  result.status = 'active';
  result.releases = [result.releases[0]];
  const release = result.releases[0];
  release.release_sequence = 1;
  release.release_status = 'active';
  release.meets_minimum_capabilities = true;
  release.blocking_clients = [];
  release.materialized_clients = ['codex'];
  release.targets = [
    { client: 'codex', authentication: 'not_required', delivery: 'managed', scopes: ['project'] },
  ];
  return result;
}
function plugin(...distributions: DistributionView[]): RegistryPlugin {
  return {
    ...structuredClone(fixture),
    name: 'example',
    declared_default_distribution: distributions[0].id,
    distributions,
  };
}

test('resolver adds diagnostic codes while retaining exact reasons, selection priority and complete-target policy', () => {
  const cases: [string, (distribution: DistributionView) => void][] = [
    [
      'distribution is suspended',
      (d) => {
        d.status = 'suspended';
      },
    ],
    [
      'no releases',
      (d) => {
        d.releases = [];
      },
    ],
    [
      'release 1 is revoked',
      (d) => {
        d.releases[0].release_status = 'revoked';
      },
    ],
    [
      'release 1 misses required components',
      (d) => {
        d.releases[0].meets_minimum_capabilities = false;
      },
    ],
    [
      'release 1 does not support codex',
      (d) => {
        d.releases[0].targets = [];
      },
    ],
    [
      'release 1 has blocking trusted failure for codex',
      (d) => {
        d.releases[0].blocking_clients = ['codex'];
      },
    ],
    [
      'release 1 lacks current positive package compatibility evidence (passed materialization) for codex',
      (d) => {
        d.releases[0].materialized_clients = [];
      },
    ],
  ];
  for (const [reason, mutate] of cases) {
    const upstream = candidate('default', 'upstream');
    mutate(upstream);
    const p = plugin(
      upstream,
      candidate('direct', 'direct'),
      candidate('bridge', 'community_bridge'),
    );
    const selected = resolveDistribution(p, ['codex']);
    assert.equal(selected.distribution!.id, 'bridge');
    assert.equal(selected.fallback_reason, `declared default default was ineligible: ${reason}`);
    assert.equal(selected.distribution!.fallback_reason, selected.fallback_reason);
    for (const format of formatters)
      assert.equal(format(selected.fallback_diagnostic), selected.fallback_reason);
    const unavailable = resolveDistribution(plugin(upstream), ['codex']);
    assert.equal(
      unavailable.unavailable_reason,
      `example: no eligible distribution supports the complete target set codex; default: ${reason}`,
    );
    for (const format of formatters)
      assert.equal(format(unavailable.unavailable_diagnostic), unavailable.unavailable_reason);
  }
  const multi = resolveDistribution(plugin(candidate('default', 'upstream')), ['codex', 'cursor']);
  assert.equal(multi.distribution, undefined);
  assert.ok(multi.unavailable_reason!.includes('codex,cursor'));
  const active = resolveDistribution(plugin(candidate('default', 'upstream')), ['codex']);
  assert.equal(active.distribution!.id, 'default');
  assert.equal(active.fallback_reason, undefined);
  for (const targets of [[], ['codex', 'codex']] as ClientID[][]) {
    const invalid = resolveDistribution(plugin(candidate('default', 'upstream')), targets);
    assert.equal(invalid.unavailable_reason, 'targets must be unique supported client IDs');
    for (const format of formatters)
      assert.equal(format(invalid.unavailable_diagnostic), invalid.unavailable_reason);
  }
  for (const format of formatters)
    assert.equal(format(undefined, '<original>'), 'Source-provided reason: <original>');
});

test('authentication visibility uses semantic states with preserved legacy precedence', () => {
  const d = candidate('default', 'upstream');
  d.targets = d.releases[0].targets;
  for (const [legacy, state, label] of [
    ['none', 'not_required', 'No account required'],
    ['oauth', 'oauth', 'OAuth required'],
    ['client_managed', 'client_managed', 'Client-managed authentication'],
  ] as const) {
    assert.equal(authenticationState(undefined, [], legacy), state);
    assert.equal(authenticationLabel(undefined, [], legacy), label);
  }
  for (const state of ['not_required', 'required', 'unknown'] as const) {
    d.targets[0].authentication = state;
    assert.equal(authenticationState(d, ['codex']), state);
    assert.equal(authenticationLabel(d, ['codex']), t(`registryUi.authentication.${state}`));
  }
  d.targets.push({ ...d.targets[0], client: 'cursor', authentication: 'required' });
  assert.equal(authenticationState(d, ['codex', 'cursor']), 'varies');
  assert.equal(authenticationState(d, []), 'unknown');
  assert.equal(authenticationState(d, ['claude']), 'unknown');
});

test('validation evidence retains priority and requires environment metadata', () => {
  const view = structuredClone(fixture);
  view.evidence = [
    { client: 'codex', level: 'oauth', outcome: 'passed' },
  ] as RegistryPlugin['evidence'];
  view.package_evidence = [];
  assert.equal(validationLabel(view), 'Package reviewed');
  Object.assign(view.evidence[0], {
    client_version: '1',
    os: 'linux',
    architecture: 'amd64',
    tested_at: '2026-09-08',
  });
  assert.equal(validationLabel(view), 'OAuth tested');
});
