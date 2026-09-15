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

  it('falls back to English when the target locale has no real content for that page', () => {
    assert.equal(resolveDocsLocale('ru', 'use'), 'en');
    assert.equal(hasRealDocsContent('ru', 'use'), false);
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

  it('leaves the URL on English when the locale page is an English-only fallback', () => {
    const base = 'https://777genius.github.io/universal-agent-plugins/docs/en/use/';
    assert.equal(replaceDocsLocale(base, 'ru', 'use'), base);
  });

  it('routes support links to the current Build landing page', () => {
    assert.equal(
      replaceDocsLocale(
        'https://777genius.github.io/universal-agent-plugins/docs/en/build/',
        'ru',
        'build',
      ),
      'https://777genius.github.io/universal-agent-plugins/docs/en/build/',
    );
  });

  it('leaves URLs without a recognised locale segment untouched', () => {
    const base = 'https://example.com/no-locale-segment';
    assert.equal(replaceDocsLocale(base, 'ru', 'anything'), base);
  });
});
