import { defineStore } from 'pinia';

const defaults = () => ({ identity: '', targetIds: [] as string[], autoDetect: true, expanded: false });
export const useInstallPreferencesStore = defineStore('installPreferences', {
  state: () => ({ channelId: null as string | null, channelUserSelected: false, package: defaults() }),
  actions: {
    selectChannel(id: string, available: readonly string[], manual = true) {
      if (!available.includes(id) || (!manual && this.channelUserSelected && this.channelId && available.includes(this.channelId))) return;
      this.channelId = id;
      this.channelUserSelected = manual;
    },
    reconcileChannels(available: readonly string[]) {
      if (this.channelId && !available.includes(this.channelId)) {
        this.channelId = null;
        this.channelUserSelected = false;
      }
    },
    readPackage(identity: string, available: readonly string[]) {
      return this.package.identity === identity && this.package.targetIds.every(id => available.includes(id))
        ? { ...this.package, targetIds: [...this.package.targetIds] } : { ...defaults(), identity };
    },
    reconcilePackage(identity: string, available: readonly string[]) {
      if (this.package.identity !== identity || this.package.targetIds.some(id => !available.includes(id))) this.package = defaults();
    },
    selectPackage(identity: string, choice: { targetIds: readonly string[]; autoDetect: boolean; expanded: boolean }, available: readonly string[]) {
      if (!identity || choice.targetIds.some(id => !available.includes(id))) {
        this.package = defaults();
        return;
      }
      this.package = { identity, targetIds: [...new Set(choice.targetIds)], autoDetect: choice.autoDetect, expanded: choice.expanded };
    },
  },
});
