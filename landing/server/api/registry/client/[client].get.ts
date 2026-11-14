import type { ClientID, RegistryIndex } from '~/types/registry';
import { clients } from '~/data/clients';
import { projectRegistry } from '~/utils/registryProjection';

const supportedClients = new Set<ClientID>(clients.map((client) => client.id));

export default defineEventHandler((event) => {
  const client = String(getRouterParam(event, 'client') ?? '') as ClientID;
  if (!supportedClients.has(client)) {
    throw createError({ statusCode: 404, statusMessage: 'Agent not found' });
  }
  return projectRegistry(useRuntimeConfig(event).registryIndex as unknown as RegistryIndex, {
    kind: 'client',
    value: client,
  });
});
