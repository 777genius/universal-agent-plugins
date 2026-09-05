import type { ClientID, RegistryIndex } from '../types/registry';

export type RegistryProjection =
  | { kind: 'catalog' }
  | { kind: 'empty' }
  | { kind: 'plugin'; value: string }
  | { kind: 'client'; value: ClientID };

export function projectRegistry(
  registry: RegistryIndex,
  projection: RegistryProjection,
): RegistryIndex {
  const reviewed = registry.plugins.filter(
    (plugin) => plugin.trust_state !== 'conformant_unreviewed',
  );
  const plugins =
    projection.kind === 'empty'
      ? []
      : projection.kind === 'plugin'
        ? reviewed.filter((plugin) => plugin.name === projection.value)
        : projection.kind === 'client'
          ? reviewed.filter((plugin) => plugin.client_support.clients.includes(projection.value))
          : reviewed;
  return {
    ...registry,
    plugins,
  };
}
