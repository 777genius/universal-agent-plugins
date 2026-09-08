import assert from 'node:assert/strict';
import { test } from 'node:test';
import { assertTranslatedCopy, validateMessages } from '../scripts/check-i18n.mjs';

test('reviewed technical captions are exact and scoped; prose clones still fail', () => {
  const reviewed = {
    nav: { faq: 'FAQ' },
    registryUi: {
      components: { mcp: 'mcp' },
      detail: { title: "{name} Agent Plugin {'|'} Universal Agent Plugins", schemaName: '{name} Agent Plugin' },
      community: { title: "{name} Agent Plugin {'|'} Universal Agent Plugins" },
    },
  };
  assertTranslatedCopy(reviewed, reviewed, 'uk');
  validateMessages(reviewed, reviewed, 'uk');
  assert.throws(() => assertTranslatedCopy(reviewed, reviewed, 'ru'), /unchanged English copy/);
  for (const copy of [
    { registryUi: { components: { mcp: 'mcp protocol support' } } },
    { nav: { other: 'FAQ' } },
    { registryUi: { detail: { description: 'Install your first plugin' } } },
  ]) assert.throws(() => assertTranslatedCopy(copy, copy, 'uk'), /unchanged English copy/);
});
