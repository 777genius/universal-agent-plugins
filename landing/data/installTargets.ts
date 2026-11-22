import { clients } from './clients.ts';
import type { ClientID, ClientTarget } from '../types/registry.ts';

// UI choices describe the surface; signed Directory compatibility remains
// keyed by the existing canonical clients. No second Codex installation exists.
export type InstallTargetID = ClientID | 'chatgptweb';
export type InstallTargetOption = Omit<ClientTarget, 'id'> & { id: InstallTargetID };

export const installTargets: InstallTargetOption[] = clients.flatMap<InstallTargetOption>((client) =>
  client.id === 'chatgpt'
    ? [
        {
          ...client,
          name: 'ChatGPT Desktop',
          id: 'chatgpt' as const,
          status: 'Supported' as const,
          note: 'Local plugin via the Codex CLI; verify in Desktop',
        },
        {
          ...client,
          id: 'chatgptweb' as const,
          name: 'ChatGPT Web',
          note: 'Remote app connection completed in ChatGPT',
          status: 'Setup in app' as const,
        },
      ]
    : [{ ...client, name: client.id === 'codex' ? 'Codex CLI' : client.name }],
);

export function canonicalInstallTarget(id: InstallTargetID): ClientID {
  return id === 'chatgpt' ? 'codex' : id === 'chatgptweb' ? 'chatgpt' : id;
}

export function canonicalInstallTargets(targets: readonly InstallTargetID[]): ClientID[] {
  return [...new Set(targets.map(canonicalInstallTarget))];
}

// Selection describes user intent. Shared execution is deduplicated separately.
export function reconcileLocalChatGPTTargets(next: InstallTargetID[], _previous: readonly InstallTargetID[]): InstallTargetID[] {
  return [...new Set(next)];
}

// Keep visible surface names in commands. The CLI maps them to canonical
// signed target IDs at the --target flag boundary.
export function cliInstallTargets(targets: readonly InstallTargetID[]): string[] {
  const canonical = new Set<ClientID>();
  return targets.filter((target) => {
    const client = canonicalInstallTarget(target);
    if (canonical.has(client)) return false;
    canonical.add(client);
    return true;
  });
}
