import assert from 'node:assert/strict';
import { test } from 'node:test';
import { requireReleasedCapabilities, stableVersion } from '../scripts/check-published-capabilities.mjs';
import source from '../data/clientCapabilities.generated.json' with { type: 'json' };

test('published Skills-only Grok cannot admit source Project MCP; matching or superset can', () => {
  const released = structuredClone(source);
  const grok = released.find(row => row.id === 'grok')!;
  grok.project!.mcp_transports.stdio = 'unsupported';
  grok.project!.mcp_transports['streamable-http'] = 'unsupported';
  assert.throws(() => requireReleasedCapabilities(source, released), /grok\/project\/mcp/);
  requireReleasedCapabilities(source, structuredClone(source));
  const smaller = structuredClone(source);
  smaller.find(row => row.id === 'grok')!.project!.mcp_transports.stdio = 'unsupported';
  requireReleasedCapabilities(smaller, source);
});

test('closed support schema refuses malformed or missing release fields; prose is irrelevant', () => {
  for (const broken of [null, [], [{ id: 'grok' }], [...source, source[0]]])
    assert.throws(() => requireReleasedCapabilities(source, broken));
  const malformed = structuredClone(source);
  malformed[0]!.skill_support = 'invented';
  assert.throws(() => requireReleasedCapabilities(source, malformed));
  requireReleasedCapabilities(source, source.map(row => ({ ...row, display_name: 'Different prose' })));
  for (const version of ['0.1.102', '1.0.0']) assert.equal(stableVersion(version), version);
  for (const version of ['v0.1.102', '01.1.2', '1.2.3-beta', '1.2.3+build', '../main', null])
    assert.throws(() => stableVersion(version));
});
