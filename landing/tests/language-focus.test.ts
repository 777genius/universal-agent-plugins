import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import { test } from 'node:test';

test('Vuetify owns ArrowUp/Down keydown and receives the forwarded activator props', () => {
  const source = readFileSync(new URL('../components/layout/LanguageSwitcher.vue', import.meta.url), 'utf8');
  assert.match(source, /<v-menu\\b[^>]*v-model="menuOpen"/);
  assert.match(source, /<v-btn\\s+v-bind="menuProps"/);
  assert.doesNotMatch(source, /@keydown\\.capture|stopImmediatePropagation\\(/);
  // The mounted real Nuxt/Vuetify keyboard and history contract is exercised in
  // tests/fixtures/i18n-candidate/candidate.spec.ts.template at both base paths.
});

test('mobile Escape reserves only an open nested menu', () => {
  const source = readFileSync(new URL('../components/layout/AppHeader.vue', import.meta.url), 'utf8');
  const logic = source.slice(source.indexOf('function onMobileEscape'), source.indexOf('const interactiveReady'));
  const languageMenuOpen = { value: true };
  const onMobileEscape = runInNewContext(stripTypeScriptTypes(logic) + '\nonMobileEscape', { languageMenuOpen });
  let prevented = 0;
  const event = { preventDefault() { prevented++; } };
  onMobileEscape(event);
  assert.equal(prevented, 1, 'nested menu consumes the first Escape');
  languageMenuOpen.value = false;
  onMobileEscape(event);
  assert.equal(prevented, 1, 'closed menu allows the dialog to handle Escape');
});
