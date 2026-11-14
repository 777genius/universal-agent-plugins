import capabilities from '../data/clientCapabilities.generated.json' with { type: 'json' };
import { canonicalInstallTarget, type InstallTargetOption } from '../data/installTargets.ts';
import type { ComponentID, InstallScope, RegistryPlugin } from '../types/registry.ts';
import { pluginInstallAvailability } from './pluginInstallAvailability.ts';
import { projectComponentSupported } from './projectCatalogSource.ts';

export interface InstallTargetMessage {
  key: string;
  params: Record<string, string>;
}

export interface InstallTargetContext {
  scope: InstallScope;
  published: boolean;
  expired: boolean;
  current: boolean;
}

function userComponentSupported(client: InstallTargetOption, component: ComponentID): boolean {
  const row = capabilities.find(item => item.id === canonicalInstallTarget(client.id));
  if (!row) return false;
  if (component === 'skills') return row.skill_support !== 'unsupported';
  if (component === 'extensions') return row.extension_support !== 'unsupported';
  return row.mcp_transports.stdio !== 'unsupported' ||
    row.mcp_transports['streamable-http'] !== 'unsupported';
}

/** Shared explanation only. Installation authority stays in the existing resolver. */
export function pluginTargetDescription(
  plugin: RegistryPlugin, client: InstallTargetOption, context: InstallTargetContext,
): InstallTargetMessage {
  const message = (key: string): InstallTargetMessage => ({ key, params: { agent: client.name } });
  if (plugin.trust_state === 'conformant_unreviewed')
    return message(plugin.discovery?.availability === 'available'
      ? 'registryUi.card.checkedAgainBeforeInstallation'
      : 'registryUi.card.unavailableAtItsIndexedSource');
  if (!context.published)
    return message('registryUi.install.unavailableReviewDataIsNotInstallationAuthority');
  if (context.expired)
    return message('registryUi.install.unavailableSignedDirectorySnapshotExpired');

  const available = pluginInstallAvailability(plugin, [client.id], context.scope);
  if (available?.kind === 'pinned_git' && context.current)
    return message('registryUi.install.piPinnedProjectSource');
  if (available?.kind === 'guided_web')
    return message('registryUi.install.guidedChatgptSetup');
  const canonical = canonicalInstallTarget(client.id);
  const target = available?.kind === 'signed'
    ? available.resolution.distribution?.targets.find(item => item.client === canonical)
    : undefined;
  if (client.id === 'chatgptweb')
    return message(target?.app_binding
      ? 'registryUi.install.verifiedConnectionFinishSetupInChatgpt'
      : 'registryUi.install.notAvailableForChatgpt');
  if (client.id === 'chatgpt')
    return message(context.scope === 'project' &&
      pluginInstallAvailability(plugin, [client.id], 'user')
      ? 'registryUi.install.useComputerScope'
      : target ? 'registryUi.desktopGuide.description'
        : 'registryUi.install.noActiveReleaseSupportsThisClient');
  if (target) return message(`registryUi.delivery.${target.delivery}`);
  if (context.scope === 'project' && !available &&
      pluginInstallAvailability(plugin, [client.id], 'user'))
    return message('registryUi.install.useComputerScope');

  // This is a broad component explanation, never a claim that a transport,
  // dependency or native activation has been qualified for this exact source.
  if (plugin.components.length && context.scope === 'project' &&
      !plugin.components.every(component => projectComponentSupported(canonical, component)))
    return message('registryUi.install.unsupportedPluginComponents');
  if (plugin.components.length && context.scope === 'user' &&
      !plugin.components.every(component => userComponentSupported(client, component)))
    return message('registryUi.install.unsupportedPluginComponents');

  const releases = plugin.distributions.flatMap(distribution => distribution.releases);
  const previouslyDeclared = releases.some(release =>
    release.targets.some(item => item.client === canonical) ||
    release.blocking_clients.includes(canonical));
  return message(plugin.installable && !previouslyDeclared
    ? 'registryUi.install.catalogReleasePending'
    : 'registryUi.install.noCompatibleSignedRelease');
}
