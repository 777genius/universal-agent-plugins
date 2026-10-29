import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { hasRealDocsContent, replaceDocsLocale, resolveDocsLocale } from '../utils/docsLinks.ts';

describe('docsLinks locale fallback', () => {
  it('keeps a locale that has real translated content', () => {
    assert.equal(resolveDocsLocale('ru', 'guide/quickstart'), 'ru');
    assert.equal(hasRealDocsContent('ru', 'guide/quickstart'), true);
    assert.equal(resolveDocsLocale('es', 'guide/quickstart'), 'es');
    assert.equal(resolveDocsLocale('fr', 'guide/quickstart'), 'fr');
    assert.equal(resolveDocsLocale('zh', 'guide/quickstart'), 'zh');
  });

  it('recognizes current Use and Build translations in every maintained docs locale', () => {
    for (const locale of ['en', 'ru', 'es', 'fr', 'zh'] as const) {
      for (const page of ['use', 'use/install', 'use/manage', 'build', 'build/checks']) {
        assert.equal(resolveDocsLocale(locale, page), locale);
        assert.equal(hasRealDocsContent(locale, page), true);
      }
    }
  });

  it('treats English as always available', () => {
    assert.equal(resolveDocsLocale('en', 'anything/not-in-registry'), 'en');
    assert.equal(hasRealDocsContent('en', 'anything/not-in-registry'), true);
  });

  it('replaces the locale segment only when the target page actually exists for that locale', () => {
    const base =
      'https://777genius.github.io/universal-agent-plugins/docs/en/guide/quickstart.html';
    assert.equal(
      replaceDocsLocale(base, 'ru', 'guide/quickstart'),
      'https://777genius.github.io/universal-agent-plugins/docs/ru/guide/quickstart.html',
    );
    assert.equal(
      replaceDocsLocale(
        'https://docs.example.test/custom/base/en/guide/quickstart.html',
        'fr',
        'guide/quickstart',
      ),
      'https://docs.example.test/custom/base/fr/guide/quickstart.html',
    );
  });

  it('leaves the URL on English when the reference has no translated page', () => {
    const base = 'https://777genius.github.io/universal-agent-plugins/docs/en/api/cli/prepared-authoring-v2-agentplugins-author';
    assert.equal(replaceDocsLocale(base, 'ru', 'api/cli/prepared-authoring-v2-agentplugins-author'), base);
  });

  it('routes support links to the current Build landing page', () => {
    assert.equal(
      replaceDocsLocale(
        'https://777genius.github.io/universal-agent-plugins/docs/en/build/',
        'ru',
        'build',
      ),
      'https://777genius.github.io/universal-agent-plugins/docs/ru/build/',
    );
  });

  it('leaves URLs without a recognised locale segment untouched', () => {
    const base = 'https://example.com/no-locale-segment';
    assert.equal(replaceDocsLocale(base, 'ru', 'anything'), base);
  });
});
