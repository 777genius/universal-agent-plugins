import { test } from 'node:test';
import assert from 'node:assert/strict';
import { useSelection } from '../../src/features/selection/model/store.ts';
import { checkedState, indexCatalog } from '../../src/features/catalog/model/catalog.ts';

test('a filtered bulk action preserves global selection, and a new snapshot clears IDs', () => {
  const state = useSelection.getState(); state.reset('catalog-a');
  state.setChecked(['outside'], true); state.setChecked(['visible', 'visible'], true);
  state.setChecked(['visible'], false);
  assert.deepEqual(useSelection.getState().ids, ['outside']);
  state.reset('catalog-b');
  assert.deepEqual(useSelection.getState().ids, []);
});
test('parent Skill and nested Skill are separate leaves; disabled leaves never enter folder bulk', () => {
  const candidates = [
    { id: 'parent', name: 'parent', path: 'parent', status: 'available' as const, description: '', diagnosticCodes: [] },
    { id: 'child', name: 'child', path: 'parent/child', status: 'available' as const, description: '', diagnosticCodes: [] },
    { id: 'broken', name: 'broken', path: 'parent/broken', status: 'invalid' as const, description: '', diagnosticCodes: ['invalid'] },
  ];
  const tree = indexCatalog(candidates);
  assert.deepEqual(new Set(tree.get('folder:parent')!.eligible), new Set(['parent', 'child']));
  assert.equal(tree.get('skill:parent')!.candidate?.id, 'parent');
  assert.deepEqual(checkedState(tree.get('folder:parent')!.eligible, ['child']), { checked: false, mixed: true });
});
