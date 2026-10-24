import { create } from 'zustand';
export const useExpansion = create<{
  ids: string[];
  reset: () => void;
  set: (value: string[] | ((old: string[]) => string[])) => void;
}>((set) => ({ ids: [], reset: () => set({ ids: [] }), set: (value) => set((state) => ({ ids: typeof value === 'function' ? value(state.ids) : value })) }));
