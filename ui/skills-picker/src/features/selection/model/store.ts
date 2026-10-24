import { create } from 'zustand';

export const useSelection = create<{
  catalogId: string;
  ids: string[];
  reset: (catalogId: string) => void;
  setChecked: (ids: readonly string[], checked: boolean) => void;
}>((set) => ({
  catalogId: '', ids: [],
  reset: (catalogId) => set({ catalogId, ids: [] }),
  setChecked: (ids, checked) => set((state) => ({
    ids: checked ? [...new Set([...state.ids, ...ids])] : state.ids.filter((id) => !ids.includes(id)),
  })),
}));
