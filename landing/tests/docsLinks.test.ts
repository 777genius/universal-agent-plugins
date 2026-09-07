import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { hasRealDocsContent, replaceDocsLocale, resolveDocsLocale } from '../utils/docsLinks.ts';

describe('docsLinks locale fallback', () => {
  it('keeps a locale that has real translated content', () => {
    assert.equal(resolveDocsLocale('ru', 'guide/quickstart'), 'ru');
    assert.equal(hasRealDocsContent('ru', 'guide/quickstart'), true);
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
    const base = 'https://777genius.github.io/universal-agent-plugins/docs/en/guide/quickstart.html';
    assert.equal(
      replaceDocsLocale(base, 'ru', 'guide/quickstart'),
      'https://777genius.github.io/universal-agent-plugins/docs/ru/guide/quickstart.html',
    );
  });

  it('leaves the URL on English when the locale page is an English-only fallback', () => {
    const base = 'https://777genius.github.io/universal-agent-plugins/docs/en/use/';
    assert.equal(replaceDocsLocale(base, 'ru', 'use'), base);
  });

  it('leaves URLs without a recognised locale segment untouched', () => {
    const base = 'https://example.com/no-locale-segment';
    assert.equal(replaceDocsLocale(base, 'ru', 'anything'), base);
  });
});
