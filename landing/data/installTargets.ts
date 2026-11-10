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

// Desktop and Codex display as separate choices but share one native backend.
// When both appear in a picker change, preserve the newly selected option.
export function reconcileLocalChatGPTTargets(next: InstallTargetID[], previous: readonly InstallTargetID[]): InstallTargetID[] {
  if (!next.includes('codex') || !next.includes('chatgpt')) return next;
  const added = next.find((id) =>
    (id === 'codex' || id === 'chatgpt') && !previous.includes(id),
  );
  const keep = added ?? (previous.includes('chatgpt') ? 'chatgpt' : 'codex');
  return next.filter((id) => id !== (keep === 'codex' ? 'chatgpt' : 'codex'));
}

// Keep visible surface names in commands. The CLI maps them to canonical
// signed target IDs at the --target flag boundary.
export function cliInstallTargets(targets: readonly InstallTargetID[]): string[] {
  return [...targets];
}
