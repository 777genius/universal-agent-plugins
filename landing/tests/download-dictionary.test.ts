import assert from 'node:assert/strict';
import { loadI18nMessages } from '../scripts/load-i18n-messages.ts';
import { test } from 'node:test';
import {
  activeMessages,
  assertTranslatedCopy,
  checkI18n,
  validateMessages,
} from '../scripts/check-i18n.mjs';

// Negative fixtures intentionally allow removing active namespaces and keys.
type MutableMessages = { download?: Record<string, string> } & Record<string, unknown>;
const read = (locale: string) => loadI18nMessages(locale) as unknown as MutableMessages;
const reference = activeMessages(read('en'));

test('real published dictionaries pass the shared compile/key/placeholder/copy gate', async () => {
  await checkI18n();
});

for (const locale of ['ru', 'uk']) {
  for (const key of ['latestRelease', 'copy', 'copied', 'detectedRecommendation', 'platforms']) {
    test(`${locale}: missing active download.${key} fails release selection`, () => {
      const messages = read(locale);
      delete messages.download![key];
      assert.throws(
        () => validateMessages(reference, activeMessages(messages), locale),
        /active keyset mismatch/,
      );
    });
  }
  test(`${locale}: missing download namespace fails`, () => {
    const messages = read(locale);
    delete messages.download;
    assert.throws(() => activeMessages(messages), /Missing active namespace: download/);
  });
  for (const [value, error] of [
    ['text {device}', /placeholders/],
    ['text {platform} @', /compilation errors/],
    ['', /empty/],
  ] as const) {
    test(`${locale}: corrupt download recommendation ${JSON.stringify(value)} fails`, () => {
      const messages = read(locale);
      messages.download!.detectedRecommendation = value;
      assert.throws(() => validateMessages(reference, activeMessages(messages), locale), error);
    });
  }
  test(`${locale}: copied English download prose fails without an allowlist`, () => {
    const messages = read(locale);
    messages.download!.quickstartSubtitle = reference.download.quickstartSubtitle;
    assert.throws(
      () => assertTranslatedCopy(reference, activeMessages(messages), locale),
      /download.quickstartSubtitle: unchanged English copy/,
    );
  });
}
