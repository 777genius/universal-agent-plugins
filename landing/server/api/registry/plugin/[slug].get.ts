import type { RegistryIndex } from '~/types/registry';
import { projectRegistry } from '~/utils/registryProjection';

export default defineEventHandler((event) => {
  const slug = String(getRouterParam(event, 'slug') ?? '');
  const projected = projectRegistry(
    useRuntimeConfig(event).registryIndex as unknown as RegistryIndex,
    { kind: 'plugin', value: slug },
  );
  if (projected.plugins.length !== 1) {
    throw createError({ statusCode: 404, statusMessage: 'Plugin not found' });
  }
  return projected;
});
