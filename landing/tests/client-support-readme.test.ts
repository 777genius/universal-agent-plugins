import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';
import { clientSupportRows, clientSupportCapabilityKeys } from '../data/clientSupport.ts';
import { clientLandingById } from '../data/clients.ts';

// SYNC: README.md "Supported clients" and landing/data/clientSupport.ts.
// Component: landing/components/registry/RegistryClientSupport.vue (before FAQ).
const readme = readFileSync(new URL('../../README.md', import.meta.url), 'utf8');
const dark = new Set(['opencode', 'cline', 'windsurf', 'grok', 'kimi']);
function image(id: string, icon: string) {
  const img = `<img src="landing/public/client-icons/${icon}" width="24" height="24" alt="" align="middle">`;
  return dark.has(id)
    ? `<picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/${id}-dark.svg">${img}</picture>`
    : img;
}
test('README and landing support rows, setup actions and logos are in sync', () => {
  const first = '<!-- AGENT_SUPPORT_MATRIX_START -->';
  const last = '<!-- AGENT_SUPPORT_MATRIX_END -->';
  assert.equal(readme.split(first).length, 2);
  assert.equal(readme.split(last).length, 2);
  assert.equal(clientSupportRows.length, 16);
  assert.equal(new Set(clientSupportRows.map((r) => r.id)).size, 16);
  assert.ok(!clientSupportRows.some((r) => r.id === 'vscode'));
  assert.ok(!clientLandingById.has('vscode'));

  const table = readme.split(first)[1]!.split(last)[0]!;
  const expected = clientSupportRows.map((r) => {
    for (const capability of [r.user, r.project])
      assert.ok(capability in clientSupportCapabilityKeys, `Missing translation mapping: ${capability}`);
    const client = clientLandingById.get(r.id);
    assert.ok(client, `Unknown client ${r.id}`);
    assert.ok(existsSync(new URL(`../public/client-icons/${client.icon}`, import.meta.url)));
    if (dark.has(r.id))
      assert.ok(existsSync(new URL(`../../assets/client-icons/${r.id}-dark.svg`, import.meta.url)));
    return `| ${image(r.id, client.icon)} **${r.name}** | ${r.user} | ${r.project} | ${r.setup} | ${r.nextStep} |`;
  });
  const actual = table.split('\n').filter((row) => row.startsWith('| <img') || row.startsWith('| <picture'));
  assert.deepEqual(actual, expected);
  assert.match(readme, /landing\/data\/clientSupport\.ts/);
});

test('English support translations preserve source-matrix capabilities and next steps', () => {
  const messages = JSON.parse(
    readFileSync(new URL('../i18n/locales/en.json', import.meta.url), 'utf8'),
  ).shell.clientSupport;
  assert.deepEqual(Object.keys(messages.nextSteps).filter((id) => new Set<string>(clientLandingById.keys()).has(id)).sort(), clientSupportRows.map((row) => row.id).sort());
  for (const row of clientSupportRows) {
    assert.equal(messages.nextSteps[row.id], row.nextStep, row.id);
    for (const original of [row.user, row.project]) {
      const key = clientSupportCapabilityKeys[original as keyof typeof clientSupportCapabilityKeys];
      assert.equal(messages.capabilities[key], original, original);
    }
    assert.equal(messages.setup[row.setup.toLowerCase()], row.setup, row.setup);
  }
});

// Delivery and activation are separate; preserve scope refusals in public copy.
test('new client rows retain guided trust and prepared activation boundaries', () => {
  const hermes = clientSupportRows.find((row) => row.id === 'hermes')!;
  assert.equal(hermes.user, 'Skills, MCP');
  assert.equal(hermes.project, 'Skills only');
  assert.equal(hermes.setup, 'Guided');
  assert.match(hermes.nextStep, /trust.*Project MCP is unsupported/);
  const antigravity = clientSupportRows.find((row) => row.id === 'antigravity')!;
  assert.equal(antigravity.user, 'Plugins, Skills, MCP');
  assert.equal(antigravity.project, 'Plugins, Skills, MCP');
  assert.equal(antigravity.setup, 'Prepared');
  assert.equal(clientLandingById.get('antigravity')!.status, 'Prepared by CLI');
  assert.match(antigravity.nextStep, /verify.*discovered and enabled/);
});

// Guard the single landing/README icon mapping against accidental SVG regressions.
test('Hermes logo is a valid WebP and README uses it consistently', () => {
  const icon = clientLandingById.get('hermes')!.icon;
  assert.equal(icon, 'hermes.webp');
  const bytes = readFileSync(new URL(`../public/client-icons/${icon}`, import.meta.url));
  assert.equal(bytes.toString('ascii', 0, 4), 'RIFF');
  assert.equal(bytes.toString('ascii', 8, 12), 'WEBP');
  assert.equal(readme.split('landing/public/client-icons/hermes.webp').length - 1, 2);
  assert.ok(!readme.includes('hermes.svg'));
  assert.ok(!readme.includes('The static website initially shows'));
});
