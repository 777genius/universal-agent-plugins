import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import { test } from 'node:test';

test('keyboard opening retries focus after hidden lazy overlay enters and cancels when closed', () => {
  const source = readFileSync(new URL('../components/layout/LanguageSwitcher.vue', import.meta.url), 'utf8');
  const logic = source.slice(source.indexOf('let openingEdge:'), source.indexOf('// Vuetify handles Escape/Tab'));
  const document: { activeElement: object | null } = { activeElement: null };
  let visible = false;
  const options = Array.from({ length: 3 }, () => ({
    focus() { if (visible) document.activeElement = this; },
  }));
  const menuOpen = { value: false };
  const ticks: (() => void)[] = [];
  const state = runInNewContext(stripTypeScriptTypes(logic) + '\n({ onActivatorKeydown, focusOpeningItem })', {
    document, menuOpen, pending: { value: false },
    menu: { value: { contentEl: { querySelectorAll: () => options } } },
    nextTick: (callback: () => void) => ticks.push(callback),
  });
  const press = (key: string) => state.onActivatorKeydown({ key, preventDefault() {}, stopImmediatePropagation() {} });
  press('ArrowDown');
  ticks.shift()!();
  assert.equal(document.activeElement, null);
  visible = true;
  state.focusOpeningItem(); // VMenu after-enter, when the hidden item becomes focusable.
  assert.equal(document.activeElement, options[0]);
  document.activeElement = options[1];
  state.focusOpeningItem();
  assert.equal(document.activeElement, options[1], 'completed request must not steal later focus');
  press('ArrowUp');
  ticks.shift()!();
  assert.equal(document.activeElement, options[2]);
  press('ArrowDown');
  menuOpen.value = false;
  ticks.shift()!();
  assert.equal(document.activeElement, options[2], 'dismissed menu must not reclaim focus');
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
