import { clients } from './clients.ts';
import type { ClientID, ClientTarget } from '../types/registry.ts';

// UI choices describe the surface; signed Directory compatibility remains
// keyed by the existing canonical clients. No second Codex installation exists.
export type InstallTargetID = ClientID | 'chatgpt-desktop';
export type InstallTargetOption = Omit<ClientTarget, 'id'> & { id: InstallTargetID };

export const installTargets: InstallTargetOption[] = clients.flatMap((client) =>
  client.id === 'chatgpt'
    ? [
        {
          ...client,
          name: 'ChatGPT Desktop',
          id: 'chatgpt-desktop' as const,
          status: 'Supported' as const,
          note: 'Local plugin via the Codex CLI; verify in Desktop',
        },
        {
          ...client,
          name: 'ChatGPT Web',
          note: 'Remote app connection completed in ChatGPT',
          status: 'Setup in app' as const,
        },
      ]
    : [client],
);

export function canonicalInstallTarget(id: InstallTargetID): ClientID {
  return id === 'chatgpt-desktop' ? 'codex' : id;
}

export function canonicalInstallTargets(targets: readonly InstallTargetID[]): ClientID[] {
  return [...new Set(targets.map(canonicalInstallTarget))];
}
