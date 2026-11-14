import { defineStore } from 'pinia';
type Family = 'home' | 'catalog';
type Entry = { fingerprint: string; displayLimit: number };
export const useCatalogUiStore = defineStore('catalogUi', {
  state: () => ({ home: null as Entry | null, catalog: null as Entry | null }),
  actions: {
    displayLimit(family: Family, fingerprint: string, initial: number) {
      const entry = this[family];
      return entry?.fingerprint === fingerprint ? entry.displayLimit : initial;
    },
    reconcile(family: Family, fingerprint: string) {
      if (this[family]?.fingerprint !== fingerprint) this[family] = null;
    },
    showMore(family: Family, fingerprint: string, limit: number) {
      if (Number.isSafeInteger(limit) && limit > 0) this[family] = { fingerprint, displayLimit: limit };
    },
  },
});
