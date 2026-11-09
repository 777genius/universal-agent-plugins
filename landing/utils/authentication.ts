import type { ClientID, DistributionView, TargetAuthentication } from '../types/registry.ts';

export interface AuthenticationPolicyGroup {
  authentication: TargetAuthentication | 'varies';
  label: string;
  clients: ClientID[];
}

export interface AuthenticationIndicator {
  state: TargetAuthentication;
  varies: boolean;
  label: string;
  release: string;
  groups: AuthenticationPolicyGroup[];
  summary: string;
  note: string;
}
type Translate = (key: string, params?: Record<string, string | number>) => string;

// Groups render most-consequential first so a mixed answer reads at a glance.
type DisplayPolicy = TargetAuthentication | 'varies';
const SEVERITY: DisplayPolicy[] = ['required', 'varies', 'unknown', 'not_required'];

/** Resolved release targets are authoritative; legacy product hints are not. */
export function authenticationIndicator(
  distribution: Pick<DistributionView, 'id' | 'release_sequence' | 'targets'> | undefined,
  selectedClients: readonly ClientID[],
  autoDetect: boolean,
  t: Translate,
): AuthenticationIndicator {
  const label = (state: TargetAuthentication | 'varies') =>
    t(`registryUi.authenticationIndicator.${state}`);
  const text = (reason: string) => t(`registryUi.authenticationIndicator.${reason}`);
  // Auto mode can resolve different signed releases for the same client.
  // Keep every policy but render each client once, as release-dependent when
  // the eligible candidates disagree. Never suggest simultaneous sign-in states.
  const policies = new Map<ClientID, Set<TargetAuthentication>>();
  if (autoDetect) {
    for (const target of distribution?.targets ?? []) {
      const values = policies.get(target.client) ?? new Set<TargetAuthentication>();
      values.add(target.authentication);
      policies.set(target.client, values);
    }
  }
  const entries: { client: ClientID; authentication: DisplayPolicy }[] = autoDetect
    ? [...policies].map(([client, values]) => ({
        client,
        authentication: values.size === 1 ? [...values][0]! : 'varies',
      }))
    : selectedClients.map((client) => ({
        client,
        authentication:
          distribution?.targets.find((target) => target.client === client)?.authentication ??
          'unknown',
      }));
  if (!distribution || !entries.length)
    return {
      state: 'unknown',
      varies: false,
      label: label('unknown'),
      release: '',
      groups: [],
      summary: text(autoDetect ? 'automatic' : 'unresolved'),
      note: '',
    };
  const varies = entries.some((entry) => entry.authentication === 'varies') ||
    new Set(entries.map((entry) => entry.authentication)).size > 1;
  const selectedState = entries[0]!.authentication;
  const state: TargetAuthentication =
    varies || selectedState === 'varies' ? 'unknown' : selectedState;
  const byPolicy = new Map<DisplayPolicy, ClientID[]>();
  for (const entry of entries) {
    const clients = byPolicy.get(entry.authentication) ?? [];
    if (!clients.includes(entry.client)) clients.push(entry.client);
    byPolicy.set(entry.authentication, clients);
  }
  return {
    state,
    varies,
    label: label(varies ? 'varies' : state),
    release:
      distribution.release_sequence === undefined
        ? distribution.id
        : t('registryUi.authenticationIndicator.release', {
            id: distribution.id,
            sequence: distribution.release_sequence,
          }),
    groups: SEVERITY.filter((policy) => byPolicy.has(policy)).map((policy) => ({
      authentication: policy,
      label: label(policy),
      clients: byPolicy.get(policy)!,
    })),
    summary: text(varies ? 'variesDetail' : `${state}Detail`),
    note: autoDetect ? text('automatic') : '',
  };
}
