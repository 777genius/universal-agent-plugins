import { defineStore } from 'pinia';

export type ThemeName = 'light' | 'dark';

export const useThemeStore = defineStore('theme', {
  state: () => ({
    current: 'dark' as ThemeName,
    userSelected: false,
  }),
  actions: {
    restoreTheme(saved: unknown) {
      // A choice made before the adapter starts takes precedence over storage.
      if (!this.userSelected && (saved === 'dark' || saved === 'light')) {
        this.current = saved;
        this.userSelected = true;
      }
    },
    setTheme(theme: ThemeName, fromUser: boolean) {
      this.current = theme;
      if (fromUser) this.userSelected = true;
    },
    toggleTheme() {
      this.setTheme(this.current === 'dark' ? 'light' : 'dark', true);
    },
  },
});
