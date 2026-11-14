import { defineStore } from 'pinia';
import { isPublishedLocale, type PublishedLocale } from '../data/i18n.ts';

/** Preference metadata only: the route owns the active locale. */
export const useLocaleStore = defineStore('locale', {
  state: () => ({ preferredLocale: null as PublishedLocale | null, userSelected: false }),
  actions: {
    rememberChoice(value: unknown) {
      if (!isPublishedLocale(value)) return;
      this.preferredLocale = value;
      this.userSelected = true;
    },
  },
});
