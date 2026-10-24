import type { ClientID, DistributionView, TargetAuthentication } from '../types/registry.ts';

export interface AuthenticationIndicator {
  state: TargetAuthentication;
  label: string;
  detail: string;
}
type Translate = (key: string, params?: Record<string, string | number>) => string;

/** Resolved release targets are authoritative; legacy product hints are not. */
export function authenticationIndicator(
  distribution: Pick<DistributionView, 'id' | 'release_sequence' | 'targets'> | undefined,
  selectedClients: readonly ClientID[],
  autoDetect: boolean,
  t: Translate,
): AuthenticationIndicator {
  const label = (state: TargetAuthentication | 'varies') =>
    t(`registryUi.authenticationIndicator.${state}`);
  const detail = (reason: string) => t(`registryUi.authenticationIndicator.${reason}`);
  // Auto mode installs into whatever agents the command detects, so every
  // reachable target policy counts; a single known answer needs all to agree.
  const entries = autoDetect
    ? (distribution?.targets ?? [])
    : selectedClients.map((client) => ({
        client,
        authentication:
          distribution?.targets.find((target) => target.client === client)?.authentication ??
          'unknown',
      }));
  if (!distribution || !entries.length)
    return {
      state: 'unknown',
      label: label('unknown'),
      detail: detail(autoDetect ? 'automatic' : 'unresolved'),
    };
  const grouped = new Map<ClientID, TargetAuthentication[]>();
  for (const entry of entries) {
    const policies = grouped.get(entry.client) ?? [];
    if (!policies.includes(entry.authentication)) policies.push(entry.authentication);
    grouped.set(entry.client, policies);
  }
  const states = new Set(entries.map((entry) => entry.authentication));
  const state = states.size === 1 ? entries[0]!.authentication : 'unknown';
  const release =
    distribution.release_sequence === undefined
      ? distribution.id
      : t('registryUi.authenticationIndicator.release', {
          id: distribution.id,
          sequence: distribution.release_sequence,
        });
  return {
    state,
    label: label(states.size > 1 ? 'varies' : state),
    detail: `${release}. ${[...grouped]
      .map(([client, policies]) => `${client}: ${policies.map((policy) => label(policy)).join(' / ')}`)
      .join('; ')}. ${detail(`${state}Detail`)}${autoDetect ? ` ${detail('automatic')}` : ''}`,
  };
}
