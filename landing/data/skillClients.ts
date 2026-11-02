import { clients } from './clients.ts';
import capabilities from './clientCapabilities.generated.json' with { type: 'json' };

const supported = new Set(
  capabilities
    .filter((client) => client.saved_dependency && client.skill_support !== 'unsupported')
    .map((client) => client.id),
);
export const skillClients = clients.filter((client) => supported.has(client.id));
