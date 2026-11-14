import { defineStore } from 'pinia';
import type { InstallScope } from '../types/registry';
import { installTargets } from '../data/installTargets.ts';

const publicTargetIds = new Set<string>(installTargets.map((target) => target.id));
// Only known public targets survive persistence; their current install
// eligibility is separate from the user's explicit scope and agent intent.
const visibleTargetIds = (ids: readonly string[]) =>
  ids.filter((id) => publicTargetIds.has(id));

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
      // A known manual choice stays visible when its source is withdrawn.
      // The installation selector must refuse it until the user chooses again.
      const targetIds = visibleTargetIds(this.package.targetIds);
      return this.package.identity === identity &&
        (this.package.scope === 'project' ||
          !this.package.autoDetect ||
          targetIds.every((id) => available.includes(id)))
        ? { ...this.package, targetIds, autoDetect: this.package.autoDetect || !targetIds.length }
        : { ...defaults(), identity };
    },
    reconcilePackage(identity: string, available: readonly string[]) {
      const targetIds = visibleTargetIds(this.package.targetIds);
      if (
        this.package.identity !== identity ||
        (this.package.scope !== 'project' && this.package.autoDetect &&
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
      if (!identity || (scope !== 'project' && choice.autoDetect && targetIds.some((id) => !available.includes(id)))) {
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
