import { defineStore } from 'pinia';
import type { InstallScope } from '../types/registry';
import { hiddenPublicClientIds } from '../data/clients.ts';

// Retired public targets must not erase the user's explicit project scope.
const visibleTargetIds = (ids: readonly string[]) =>
  ids.filter((id) => !hiddenPublicClientIds.has(id));

const defaults = () => ({
  identity: '',
  targetIds: [] as string[],
  autoDetect: true,
  expanded: false,
  scope: 'user' as InstallScope,
});
export const useInstallPreferencesStore = defineStore('installPreferences', {
  state: () => ({
    channelId: null as string | null,
    channelUserSelected: false,
    package: defaults(),
  }),
  actions: {
    selectChannel(id: string, available: readonly string[], manual = true) {
      if (
        !available.includes(id) ||
        (!manual &&
          this.channelUserSelected &&
          this.channelId &&
          available.includes(this.channelId))
      )
        return;
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
      // Keep explicit scope when a previously visible agent is retired.
      const targetIds = visibleTargetIds(this.package.targetIds);
      return this.package.identity === identity &&
        (this.package.scope === 'project' ||
          targetIds.every((id) => available.includes(id)))
        ? { ...this.package, targetIds, autoDetect: this.package.autoDetect || !targetIds.length }
        : { ...defaults(), identity };
    },
    reconcilePackage(identity: string, available: readonly string[]) {
      const targetIds = visibleTargetIds(this.package.targetIds);
      if (
        this.package.identity !== identity ||
        (this.package.scope !== 'project' &&
          targetIds.some((id) => !available.includes(id)))
      ) {
        this.package = defaults();
        return;
      }
      if (targetIds.length !== this.package.targetIds.length) {
        this.package.targetIds = targetIds;
        if (!targetIds.length) this.package.autoDetect = true;
      }
    },
    selectPackage(
      identity: string,
      choice: {
        targetIds: readonly string[];
        autoDetect: boolean;
        expanded: boolean;
        scope?: InstallScope;
      },
      available: readonly string[],
    ) {
      const targetIds = [...new Set(visibleTargetIds(choice.targetIds))];
      const scope = choice.scope ?? 'user';
      if (!identity || (scope !== 'project' && targetIds.some((id) => !available.includes(id)))) {
        this.package = defaults();
        return;
      }
      this.package = {
        identity,
        targetIds,
        autoDetect: choice.autoDetect || !targetIds.length,
        expanded: choice.expanded,
        scope,
      };
    },
  },
});
