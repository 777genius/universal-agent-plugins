import type { ClientID, InstallScope, RegistryPlugin } from '../types/registry.ts';
import type { InstallTargetID } from '../data/installTargets.ts';
import { pluginInstallAvailability } from './pluginInstallAvailability.ts';

/** Agent pages prefer Context7 when eligible and preserve the actual public surface. */
export function agentInstallExample(
  plugins: readonly RegistryPlugin[],
  client: ClientID,
  scope: InstallScope = 'user',
): { plugin: RegistryPlugin; target: InstallTargetID } | undefined {
  const targets: InstallTargetID[] = client === 'chatgpt' ? ['chatgpt', 'chatgptweb'] : [client];
  const ordered = [
    ...plugins.filter((plugin) => plugin.name === 'context7'),
    ...plugins.filter((plugin) => plugin.name !== 'context7'),
  ];
  for (const plugin of ordered) {
    for (const target of targets) {
      if (pluginInstallAvailability(plugin, [target], scope)) return { plugin, target };
    }
  }
  return undefined;
}
