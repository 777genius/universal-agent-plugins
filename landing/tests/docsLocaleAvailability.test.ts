import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { collectDocsLocaleAvailability } from '../scripts/generate-docs-locale-availability.mjs';

const locales = ['en', 'ru', 'es', 'fr', 'zh'];
const currentRoutes = [
  '', 'build', 'build/checks', 'build/handoff', 'build/hybrid', 'build/layout',
  'build/mcp-remote', 'build/mcp-stdio', 'build/skill', 'build/skills',
  'guide/quickstart', 'reference/client-compatibility', 'use', 'use/install', 'use/manage',
];

test('committed availability covers every current public source route without rewriting historical dispositions', async () => {
  const inventoryUrl = new URL('../../website/tools/quality/locale-dispositions.json', import.meta.url);
  const preserved = await fs.readFile(inventoryUrl, 'utf8');
  const inventory = JSON.parse(preserved);
  // These immutable entries intentionally remain fallbacks after the current
  // translations have been added in source frontmatter.
  assert.equal(inventory.pages['ru/build/checks.md'].disposition, 'english-fallback');
  const generated = await collectDocsLocaleAvailability();
  for (const locale of locales) assert.deepEqual(generated.realPaths[locale], currentRoutes, locale);
  const committed = JSON.parse(await fs.readFile(new URL('../data/docsLocaleAvailability.generated.json', import.meta.url), 'utf8'));
  assert.deepEqual(committed, generated, 'regenerate docs locale availability after changing current public translations');
  assert.equal(await fs.readFile(inventoryUrl, 'utf8'), preserved);
});

test('only correctly localized current public translations become landing destinations', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'uap-docs-availability-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  for (const locale of locales) await fs.mkdir(path.join(root, locale));
  const write = async (relative: string, locale: string, disposition?: string) => {
    const file = path.join(root, relative);
    await fs.mkdir(path.dirname(file), { recursive: true });
    await fs.writeFile(file, `---\nlocale: "${locale}"\n${disposition ? `localeDisposition: "${disposition}"\n` : ''}---\n\n# Fixture\n`);
  };
  await write('en/build/index.md', 'en');
  await write('ru/build/index.md', 'ru', 'current-translation');
  await write('ru/use/index.md', 'ru', 'english-fallback');
  await write('ru/index.md', 'ru', 'historical-snapshot');
  await write('ru/guide/installation.md', 'ru', 'current-translation');
  await write('ru/build/mismatched.md', 'es', 'current-translation');

  const initial = await collectDocsLocaleAvailability(root);
  assert.deepEqual(initial.realPaths, { en: ['build'], ru: ['build'], es: [], fr: [], zh: [] });

  // A current translation changes the link map without editing or inventing
  // an entry in the historical preservation inventory.
  await write('ru/use/index.md', 'ru', 'current-translation');
  const translated = await collectDocsLocaleAvailability(root);
  assert.deepEqual(translated.realPaths.ru, ['build', 'use']);
  assert.deepEqual(await collectDocsLocaleAvailability(root), translated, 'deterministic repeated collection');
});
