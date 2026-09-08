import type { RegistryIndex } from '~/types/registry';
import { projectRegistry } from '~/utils/registryProjection';

export default defineEventHandler((event) =>
  projectRegistry(useRuntimeConfig(event).registryIndex as unknown as RegistryIndex, {
    kind: 'empty',
  }),
);
