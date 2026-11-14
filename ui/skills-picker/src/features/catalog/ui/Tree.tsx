import { useMemo, useRef } from 'react';
import { useTree } from '@headless-tree/react';
import { buildProxiedInstance, CheckedState, checkboxesFeature, hotkeysCoreFeature, searchFeature, syncDataLoaderFeature, type FeatureImplementation } from '@headless-tree/core';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen } from '../../../shared/ui/icons';
import { checkedState, indexCatalog, type CatalogNode } from '../model/catalog';
import { useExpansion } from '../model/store';
import type { Candidate } from '../../../shared/api/types';

export function Tree({ candidates, selected, locked, path, choose, browse, preview }: {
  candidates: Candidate[]; selected: string[]; locked: boolean; path: string;
  choose: (ids: string[], checked: boolean) => void; browse: (path: string) => void; preview: (id: string) => void;
}) {
  const nodes = useMemo(() => indexCatalog(candidates), [candidates]);
  const expanded = useExpansion((s) => s.ids), setExpanded = useExpansion((s) => s.set);
  const scroll = useRef<HTMLDivElement>(null);
  const scrollToIndex = useRef<(index: number) => void>(() => {});
  const pendingFocus = useRef<string | null>(null);
  // Library instances retain configuration; current inputs stay behind one adapter ref.
  const current = useRef({ selected, choose, locked, browse, preview });
  current.current = { selected, choose, locked, browse, preview };
  const checkboxPolicy = useMemo<FeatureImplementation<CatalogNode>>(() => ({
    key: 'picker-render-adapter',
    treeInstance: {
      // Navigation remains Headless Tree's responsibility. Virtual rows, however,
      // may mount after its fixed 500ms DOM poll under concurrent browser load.
      // Fulfil the focus request when React actually registers that row instead.
      updateDomFocus: ({ tree: instance }) => {
        const item = instance.getFocusedItem();
        if (!item) return;
        pendingFocus.current = item.getId();
        instance.getConfig().scrollToItem?.(item);
        const element = item.getElement();
        if (element?.isConnected) {
          element.focus({ preventScroll: true });
          pendingFocus.current = null;
        }
      },
    },
    onItemMount: (item, element, instance) => {
      if (pendingFocus.current !== item.getId()) return;
      queueMicrotask(() => {
        if (element.isConnected && pendingFocus.current === item.getId() && instance.getFocusedItem()?.getId() === item.getId()) {
          element.focus({ preventScroll: true });
          pendingFocus.current = null;
        }
      });
    },
    itemInstance: {
      toggleCheckedState: async ({ item }) => {
        if (current.current.locked) return;
        const ids = item.getItemData().eligible;
        current.current.choose(ids, !checkedState(ids, current.current.selected).checked);
      },
      getCheckedState: ({ item }) => {
        const state = checkedState(item.getItemData().eligible, current.current.selected);
        return state.checked ? CheckedState.Checked : state.mixed ? CheckedState.Indeterminate : CheckedState.Unchecked;
      },
    },
  }), []);
  const tree = useTree<CatalogNode>({
    instanceBuilder: buildProxiedInstance,
    rootItemId: 'root', state: { expandedItems: expanded, checkedItems: selected }, setExpandedItems: setExpanded,
    getItemName: (item) => item.getItemData().name, isItemFolder: (item) => !item.getItemData().candidate,
    dataLoader: { getItem: (id) => nodes.get(id)!, getChildren: (id) => nodes.get(id)!.children },
    features: [syncDataLoaderFeature, checkboxesFeature, hotkeysCoreFeature, searchFeature, checkboxPolicy],
    onPrimaryAction: (item) => {
      const node = item.getItemData();
      if (node.candidate) current.current.preview(node.candidate.id); else current.current.browse(node.path);
    },
    scrollToItem: (item) => scrollToIndex.current(item.getItemMeta().index),
    hotkeys: {
      customCheck: { hotkey: 'Space', handler: (event, instance) => { event.preventDefault(); void instance.getFocusedItem()?.toggleCheckedState(); } },
      customOpen: { hotkey: 'Enter', handler: (event, instance) => { event.preventDefault(); instance.getFocusedItem()?.primaryAction(); } },
      submitSearch: { hotkey: 'Enter', allowWhenInputFocused: true, isEnabled: (instance) => instance.isSearchOpen(), handler: (_event, instance) => { instance.closeSearch(); instance.getFocusedItem()?.primaryAction(); } },
    },
  });
  const items = tree.getItems();
  const virtual = useVirtualizer({ count: items.length, getScrollElement: () => scroll.current, estimateSize: () => 40,
    overscan: 5, getItemKey: (index) => items[index].getId() });
  scrollToIndex.current = (index) => virtual.scrollToIndex(index, { align: 'auto' });
  const containerProps = tree.getContainerProps('Skill folders and candidates');
  return <nav className="folder-panel" aria-label="Repository folders">
    <div className="panel-heading"><h2>Folders</h2><span className="eyebrow">REPOSITORY</span></div>
    <button className={`repo-root ${!path ? 'active' : ''}`} onClick={() => browse('')}><FolderOpen size={16}/> All skills <span>{candidates.length}</span></button>
    <div {...containerProps} ref={(element) => { scroll.current = element; containerProps.ref?.(element); }} className="tree-scroll">
      <div style={{ height: virtual.getTotalSize(), position: 'relative' }}>
        {virtual.getVirtualItems().map((row) => {
          const item = items[row.index], node = item.getItemData(), state = checkedState(node.eligible, selected);
          const itemProps = item.getProps();
          const checkboxProps = item.getCheckboxProps();
          return <div {...itemProps} key={node.id} className={`tree-row ${path === node.path && !node.candidate ? 'active' : ''}`}
            style={{ position: 'absolute', top: 0, width: '100%', height: 40, transform: `translateY(${row.start}px)`, paddingLeft: 8 + item.getItemMeta().level * 16 }}
            onClick={() => { item.setFocused(); item.primaryAction(); }}>
            {!node.candidate ? <button tabIndex={-1} className="tree-chevron" aria-label={`${item.isExpanded() ? 'Collapse' : 'Expand'} ${node.path}`} onClick={(event) => { event.stopPropagation(); item.isExpanded() ? item.collapse() : item.expand(); }}>{item.isExpanded() ? <ChevronDown size={13}/> : <ChevronRight size={13}/>}</button> : <span className="tree-chevron"/>}
            <input {...checkboxProps} tabIndex={-1} type="checkbox" aria-label={`Select all ${node.eligible.length} in ${node.path || node.name}`} disabled={locked || !node.eligible.length}
              aria-checked={state.mixed ? 'mixed' : state.checked} onClick={(event) => { event.stopPropagation(); item.setFocused(); tree.updateDomFocus(); }}/>
            {node.candidate ? <FileText size={14}/> : <Folder size={14}/>}<span title={node.path}>{node.name}</span>
          </div>;
        })}
      </div>
    </div>
    {tree.isSearchOpen() && <input {...tree.getSearchInputElementProps()} aria-label="Jump to tree item" className="tree-typeahead"/>}
  </nav>;
}
