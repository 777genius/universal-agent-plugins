import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { createI18n, type LocaleMessageDictionary, type VueMessageType } from 'vue-i18n';
import { slavicPluralRule } from '../utils/slavicPluralRule.ts';

test('actual Nuxt configuration selects ONE | FEW | MANY for RU and UK', () => {
  const nuxt = readFileSync(new URL('../nuxt.config.ts', import.meta.url), 'utf8');
  assert.match(nuxt, /vueI18n: '\.\/i18n.config.ts'/);
  const source = readFileSync(new URL('../i18n.config.ts', import.meta.url), 'utf8')
    .replace(/^import[^;]+;/m, '').replace('export default', '');
  const config = runInNewContext(source, { defineI18nConfig: (factory: () => object) => factory(), slavicPluralRule });
  const { t, locale } = createI18n<[LocaleMessageDictionary<VueMessageType>], 'ru' | 'uk', false>({
    ...config, legacy: false, locale: 'ru', fallbackLocale: false,
    messages: { ru: { count: 'ONE | FEW | MANY' }, uk: { count: 'ONE | FEW | MANY' } },
  }).global;
  for (const code of ['ru', 'uk'] as const) {
    locale.value = code;
    for (const [count, expected] of [[0, 'MANY'], [1, 'ONE'], [2, 'FEW'], [5, 'MANY'],
      [11, 'MANY'], [21, 'ONE'], [22, 'FEW'], [25, 'MANY'], [101, 'ONE']] as const) {
      assert.equal(t('count', count), expected, `${code}:${count}`);
    }
  }
});
