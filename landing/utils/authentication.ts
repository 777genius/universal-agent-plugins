import type { ClientID, DistributionView, TargetAuthentication } from '../types/registry.ts';

export interface AuthenticationPolicyGroup {
  authentication: TargetAuthentication;
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
const SEVERITY: TargetAuthentication[] = ['required', 'unknown', 'not_required'];

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
      varies: false,
      label: label('unknown'),
      release: '',
      groups: [],
      summary: text(autoDetect ? 'automatic' : 'unresolved'),
      note: '',
    };
  const varies = new Set(entries.map((entry) => entry.authentication)).size > 1;
  const state = varies ? 'unknown' : entries[0]!.authentication;
  const byPolicy = new Map<TargetAuthentication, ClientID[]>();
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
    summary: text(`${state}Detail`),
    note: autoDetect ? text('automatic') : '',
  };
}
