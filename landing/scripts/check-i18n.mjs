import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import { createI18n } from 'vue-i18n';
import { slavicPluralRule } from '../utils/slavicPluralRule.ts';
import { publishedLocales } from '../data/i18n.ts';
import { productRoutes } from '../data/routes.ts';
import { expandLocalizedRoutes } from '../utils/localizedRoutes.ts';
import { clientLandingPages } from '../data/clients.ts';

const flatten = (value, prefix = '', result = {}) => {
  if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value))
      flatten(child, prefix ? `${prefix}.${key}` : key, result);
  } else result[prefix] = value;
  return result;
};
const placeholders = (text) => [...text.matchAll(/\{([\w]+)\}/g)].map((match) => match[1]).sort();
// A quoted literal pipe is message text, not a plural separator.
const branches = (text) => {
  const parts = [''];
  for (const token of text.match(/\{'[^']*'\}|[^|{]+|[\s\S]/g) || []) {
    if (token === '|') parts.push('');
    else parts[parts.length - 1] += token;
  }
  return parts;
};
function validateBranchContracts(source, target, locale, key) {
  const expected = branches(source);
  const actual = branches(target);
  const slavic = locale === 'ru' || locale === 'uk';
  const expanded = slavic && expected.length === 2 && actual.length === 3;
  assert(actual.length === expected.length || expanded, `${locale}:${key}: plural branches`);
  for (let index = 0; index < actual.length; index++) {
    const contract = expected[expanded && index > 0 ? 1 : index];
    assert.deepEqual(
      placeholders(actual[index]),
      placeholders(contract),
      `${locale}:${key}: placeholders branch ${index}`,
    );
    assert.deepEqual(
      protectedTokens(actual[index]),
      protectedTokens(contract),
      `${locale}:${key}: protected literals branch ${index}`,
    );
  }
}
const protectedTokens = (text) =>
  text.match(/https?:\/\/[^\s<>]+|(?:SEC\d{3})|(?:--[a-z][a-z-]*)/g)?.sort() || [];

// Exact brand/protocol captions may remain English; prose still requires translation.
const unchangedLabels = new Set([
  'UAP',
  'Universal Agent Plugins',
  'Agent Plugins 1.0',
  'Agent Plugins',
  'GitHub',
  'GitLab',
  'Codex',
  'ChatGPT',
  'Cursor',
  'GitHub Copilot CLI',
  'VS Code',
  'Kiro',
  'Claude Code',
  'Gemini CLI',
  'OpenCode',
  'Cline',
  'Windsurf',
  'Context7',
  'MCP',
  'OAuth',
  'OAuth 2.0',
  'API',
  'CLI',
  'JSON',
  'YAML',
  'HTTP',
  'HTTPS',
  'stdio',
  'SSE',
  'Streamable HTTP',
  'Windows',
  'macOS',
  'Linux',
  'Homebrew',
  'PowerShell',
  'Node.js',
  'npm',
  'npx',
  'English',
  'Русский',
  'Українська',
]);
// Independently reviewed technical captions/templates, scoped by locale AND key.
const reviewedCaptions = {
  ar: { 'registryUi.components.mcp': 'mcp' },
  es: { 'registryUi.components.mcp': 'mcp' },
  fr: {
    'nav.plugins': 'Plugins',
    'nav.faq': 'FAQ',
    'registryUi.components.mcp': 'mcp',
    'plugins.install.boundaryLabel': 'Attention',
    'plugins.categories.collaboration': 'Collaboration',
    'plugins.categories.commerce': 'Commerce',
    'faq.labels.default': 'Question',
    'shell.why.sourceTitle': 'Source visible',
    'shell.header.plugins': 'Plugins',
    'shell.faq.eyebrow': 'FAQ',
    'shell.header.faq': 'FAQ',
    'shell.navigation.plugins': 'Plugins',
    'shell.navigation.faq': 'FAQ',
    'registryUi.catalog.source': 'Source',
    'registryUi.catalog.agent': 'Agent',
    'registryUi.card.installable': 'installable',
    'registryUi.install.agents': 'Agents',
    'registryUi.detail.plugins': 'Plugins',
    'registryUi.detail.version': 'Version {version}',
    'registryUi.community.plugins': 'Plugins',
    'registryUi.community.source': 'Source',
    'registryUi.agentPage.activation': 'Activation',
    'registryUi.components.extensions': 'extensions',
  },
  hi: { 'registryUi.components.mcp': 'mcp' },
  pt: {
    'nav.plugins': 'Plugins',
    'registryUi.command.terminal': 'Terminal',
    'shell.header.plugins': 'Plugins',
    'shell.navigation.plugins': 'Plugins',
    'registryUi.detail.plugins': 'Plugins',
    'registryUi.community.plugins': 'Plugins',
    'registryUi.components.mcp': 'mcp',
  },
  ru: { 'registryUi.components.mcp': 'mcp' },
  zh: { 'registryUi.components.mcp': 'mcp' },
  uk: {
    'registryUi.components.mcp': 'mcp',
    'nav.faq': 'FAQ',
    'registryUi.detail.title': "{name} Agent Plugin {'|'} Universal Agent Plugins",
    'registryUi.detail.schemaName': '{name} Agent Plugin',
    'registryUi.community.title': "{name} Agent Plugin {'|'} Universal Agent Plugins",
  },
};
export function assertTranslatedCopy(reference, messages, locale) {
  if (locale === 'en') return;
  const en = flatten(reference);
  for (const [key, value] of Object.entries(flatten(messages))) {
    if (typeof value !== 'string' || value !== en[key] || unchangedLabels.has(value)) continue;
    if (reviewedCaptions[locale]?.[key] === value) continue;
    // This exact overlay caption names the Windows shell, with no prose.
    if (key === 'shell.download.channels.powershell.title' && value === 'Windows PowerShell')
      continue;
    const prose = value
      .replace(/\{[^}]*\}/g, '')
      .replace(/https?:\/\/\S+/g, '')
      .trim();
    const command =
      /^(?:npx |npm |brew |curl |irm |agentplugins |universal-agent-plugins |\$HOME|& "\$HOME)/.test(
        value,
      );
    assert(
      !/[A-Za-z]{2}/.test(prose) || command,
      `${locale}:${key}: unchanged English copy requires translation or explicit brand review`,
    );
  }
}

/** Validate only active owned namespaces; preserved legacy sections need no invented UK copy. */
export function validateMessages(reference, messages, locale) {
  const en = flatten(reference);
  const values = flatten(messages);
  assert.deepEqual(
    Object.keys(values).sort(),
    Object.keys(en).sort(),
    `${locale}: active keyset mismatch`,
  );
  const i18n = createI18n({
    legacy: false,
    locale,
    fallbackLocale: false,
    pluralRules: { ru: slavicPluralRule, uk: slavicPluralRule },
    messages: { [locale]: messages },
    missingWarn: false,
    fallbackWarn: false,
  });
  try {
    for (const [key, value] of Object.entries(values)) {
      assert.equal(typeof value, typeof en[key], `${locale}:${key}: type`);
      assert.equal(typeof value, 'string', `${locale}:${key}: expected message`);
      assert(value.trim(), `${locale}:${key}: empty`);
      validateBranchContracts(en[key], value, locale, key);
      for (const count of [0, 1, 2, 5, 11, 21, 22, 25, 101]) {
        const params = Object.fromEntries(
          placeholders(value).map((name) => [
            name,
            name === 'count' || name === 'n' ? count : 'example',
          ]),
        );
        const errors = [];
        const originalError = console.error;
        let rendered;
        try {
          console.error = (...args) => errors.push(args.join(' '));
          rendered = i18n.global.t(key, params, { plural: count });
        } catch (error) {
          throw new Error(`${locale}:${key}: compilation errors: ${error.message}`, {
            cause: error,
          });
        } finally {
          console.error = originalError;
        }
        assert.deepEqual(errors, [], `${locale}:${key}: compilation errors`);
        assert(rendered && !/\{\w+\}/.test(rendered), `${locale}:${key}: unresolved message`);
      }
    }
  } finally {
    i18n.dispose();
  }
}

/** Shared release-gate selection, including the live download controls. */
export function activeMessages(message) {
  // These legacy control namespaces still have active consumers in the shell.
  const namespaces = [
    'download',
    'shell',
    'registryUi',
    'language',
    'nav',
    'footer',
    'theme',
    'error',
    'publicAuthoring',
  ];
  return Object.fromEntries(
    namespaces.map((namespace) => {
      assert(message[namespace], `Missing active namespace: ${namespace}`);
      return [namespace, message[namespace]];
    }),
  );
}

export async function checkI18n() {
  const read = async (locale, folder) =>
    JSON.parse(await fs.readFile(new URL(`../${folder}/${locale}.json`, import.meta.url), 'utf8'));
  const reference = await read('en', 'locales');
  const en = activeMessages(reference);
  const download = await read('en', 'content/download');
  for (const locale of publishedLocales) {
    const messages = activeMessages(await read(locale, 'locales'));
    validateMessages(en, messages, locale);
    assertTranslatedCopy(en, messages, locale);
    const overlay = await read(locale, 'content/download');
    validateMessages(download, overlay, locale);
    assertTranslatedCopy(download, overlay, locale);
    assert(
      !Object.keys(flatten(overlay)).some((key) =>
        /\.(command|href|invocation|recommended|id)$/.test(key),
      ),
      `${locale}: technical field in text overlay`,
    );
  }
  const routes = productRoutes(
    ['context7'],
    clientLandingPages.map((client) => client.slug),
  );
  assert.equal(
    expandLocalizedRoutes(routes).length,
    publishedLocales.length * (5 + clientLandingPages.length + 1),
  );
  assert.throws(() => productRoutes(['community'], []), /Reserved/);
  assert.throws(() => productRoutes(['same', 'same'], []), /duplicate/);
  console.log(
    `Validated active messages, download overlays and route contract for ${publishedLocales.join(', ')}`,
  );
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url))
  await checkI18n();
