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
  // Auto mode installs into whatever agents are present, so every resolved
  // release target counts; a single known answer only exists when they agree.
  const considered = autoDetect
    ? (distribution?.targets.map((target) => target.client) ?? [])
    : selectedClients;
  if (!distribution || !considered.length)
    return {
      state: 'unknown',
      label: label('unknown'),
      detail: detail(autoDetect ? 'automatic' : 'unresolved'),
    };
  const values = considered.map((client) => ({
    client,
    authentication:
      distribution.targets.find((target) => target.client === client)?.authentication ?? 'unknown',
  }));
  const states = new Set(values.map((value) => value.authentication));
  const state = states.size === 1 ? values[0]!.authentication : 'unknown';
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
    detail: `${release}. ${values.map((value) => `${value.client}: ${label(value.authentication)}`).join('; ')}. ${detail(`${state}Detail`)}${autoDetect ? ` ${detail('automatic')}` : ''}`,
  };
}
