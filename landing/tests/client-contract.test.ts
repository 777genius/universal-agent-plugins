import assert from 'node:assert/strict';
import { readFileSync, statSync } from 'node:fs';
import { test } from 'node:test';
import { clientIds } from '../data/clientIds.generated.ts';
import { clientLandingPages, clients, hiddenPublicClientIds } from '../data/clients.ts';
import { clientSupportCapabilityKeys, clientSupportRows } from '../data/clientSupport.ts';
import { publishedLocales } from '../data/i18n.ts';
import { assertClientCoverage } from '../utils/clientContract.ts';
import { CLIENTS, clientIDs } from '../utils/registry-codecs/common.ts';
import { releaseTargets } from '../utils/registry-codecs/evidence.ts';

test('generated capabilities, frontend identities and strict decoder share the canonical client set', () => {
  const capabilities = JSON.parse(
    readFileSync(new URL('../data/clientCapabilities.generated.json', import.meta.url), 'utf8'),
  ) as { id: string }[];
  assert.deepEqual(capabilities.map((row) => row.id), clientIds);
  assert.deepEqual([...CLIENTS], clientIds);
  assert.deepEqual(clientIDs(clientIds, 'fixture'), clientIds);
});

test('new canonical clients require presentation; stale and duplicate rows fail clearly', () => {
  const expected = ['first-agent', 'fixture-next-agent'];
  const complete = [{ id: 'fixture-next-agent' }, { id: 'first-agent' }];
  assert.doesNotThrow(() => assertClientCoverage(expected, complete, 'Agent presentation'));
  assert.throws(
    () => assertClientCoverage(expected, [{ id: 'first-agent' }], 'Agent presentation'),
    /Agent presentation: missing client fixture-next-agent/,
  );
  assert.throws(
    () => assertClientCoverage(expected, [...complete, { id: 'first-agent' }], 'Agent support table'),
    /Agent support table: duplicate client first-agent/,
  );
  assert.throws(
    () => assertClientCoverage(expected, [...complete, { id: 'retired-agent' }], 'Agent presentation'),
    /Agent presentation: unknown client retired-agent/,
  );
});

test('every public canonical client has a page, support row and nonempty icon asset', () => {
  const expected = clientIds.filter((id) => !hiddenPublicClientIds.has(id));
  for (const id of hiddenPublicClientIds) assert.ok(clientIds.includes(id as typeof clientIds[number]));
  assert.deepEqual(clients.map((client) => client.id), expected);
  assertClientCoverage(expected, clientLandingPages, 'Public agent pages');
  assertClientCoverage(expected, clientSupportRows, 'Public agent support');
  assert.equal(new Set(clientLandingPages.map((client) => client.slug)).size, expected.length);
  for (const client of clientLandingPages) {
    const icon = statSync(new URL(`../public/client-icons/${client.icon}`, import.meta.url));
    assert.ok(icon.isFile() && icon.size > 0, `${client.id}: missing icon asset`);
  }
});

for (const locale of publishedLocales) {
  test(`${locale} has presentation and support translations for every public client`, () => {
    const messages = JSON.parse(
      readFileSync(new URL(`../i18n/locales/${locale}.json`, import.meta.url), 'utf8'),
    );
    for (const client of clientLandingPages) {
      for (const field of ['note', 'status', 'intro', 'delivery', 'activation']) {
        const value = messages.registryUi.clients[client.id]?.[field];
        assert.ok(typeof value === 'string' && value.trim(), `${locale}: ${client.id}.${field}`);
      }
    }
    const support = messages.shell.clientSupport;
    for (const row of clientSupportRows) {
      const values = [support.nextSteps[row.id], support.setup[row.setup.toLowerCase()]];
      for (const label of [row.user, row.project]) {
        const key = clientSupportCapabilityKeys[label as keyof typeof clientSupportCapabilityKeys];
        assert.ok(key, `${locale}: missing support capability mapping ${label}`);
        values.push(support.capabilities[key]);
      }
      assert.ok(values.every((value) => typeof value === 'string' && value.trim()), `${locale}: ${row.id} support`);
    }
  });
}

test('Hermes, Antigravity and Pi policy decoding preserves exact declarations', () => {
  const targets = ['hermes', 'antigravity', 'pi'].map((client) => ({
    client, authentication: 'not_required', delivery: 'managed', scopes: ['user'],
  }));
  assert.deepEqual(releaseTargets(targets, 'fixture'), targets);
  assert.deepEqual(releaseTargets(targets.slice(0, 1), 'fixture'), targets.slice(0, 1));
  for (const client of ['fixture-next-agent', 'Hermes', 'hermes--extra']) {
    assert.throws(
      () => releaseTargets([{ ...targets[0], client }], 'fixture'),
      /invalid or duplicate client/,
    );
  }
  assert.throws(() => releaseTargets([targets[0], targets[0]], 'fixture'), /duplicate client/);
});
