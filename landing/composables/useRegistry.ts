import type { DiscoveryBundle } from '~/types/discovery';
import type { RegistryIndex } from '~/types/registry';
import type { SecuritySnapshot } from '~/types/security';
import { BrowserDiscoveryCache, discoveryPlugin, loadDiscovery } from '~/utils/discovery';
import { applySecurityAssessment, loadSecurity } from '~/utils/security';
import type { RegistryProjection } from '~/utils/registryProjection';

interface RegistryPageOptions {
  projection?: RegistryProjection;
  discovery?: boolean;
}

let discoveryPromise: Promise<DiscoveryBundle> | undefined;
let securityPromise: Promise<SecuritySnapshot | undefined> | undefined;

export async function useRegistryPage(options: RegistryPageOptions = {}): Promise<RegistryIndex> {
  const projection = options.projection ?? { kind: 'catalog' };
  const endpoint = registryEndpoint(projection);
  const key = `registry-page:${endpoint}`;
  const config = useRuntimeConfig();
  const registry = useState<RegistryIndex | undefined>('registry-index');
  const activeKey = useState('registry-page-key', () => '');
  const status = useDiscoveryStatus();
  const seed = shallowRef<RegistryIndex>();

  if (import.meta.client && options.discovery) {
    onMounted(() => {
      if (seed.value) {
        void augmentWithDiscovery(registry, seed.value, key, activeKey, status, config);
      }
    });
  }

  const { data, error } = await useAsyncData<RegistryIndex>(
    key,
    () => $fetch<RegistryIndex>(endpoint),
    { deep: false },
  );
  if (error.value || !data.value) {
    throw createError({
      statusCode: 500,
      statusMessage: 'Plugin directory is unavailable',
      cause: error.value,
    });
  }

  seed.value = structuredClone(data.value);
  activeKey.value = key;
  registry.value = seed.value;
  return registry.value;
}

export function useRegistry(): RegistryIndex {
  const registry = useState<RegistryIndex | undefined>('registry-index');
  if (!registry.value) {
    throw createError({ statusCode: 500, statusMessage: 'Plugin directory is not initialized' });
  }
  return registry.value;
}

export function registryEndpoint(projection: RegistryProjection): string {
  if (projection.kind === 'plugin') {
    return `/api/registry/plugin/${encodeURIComponent(projection.value)}`;
  }
  if (projection.kind === 'client') {
    return `/api/registry/client/${encodeURIComponent(projection.value)}`;
  }
  return `/api/registry/${projection.kind}`;
}

async function augmentWithDiscovery(
  registry: Ref<RegistryIndex | undefined>,
  seed: RegistryIndex,
  key: string,
  activeKey: Ref<string>,
  status: ReturnType<typeof useDiscoveryStatus>,
  config: ReturnType<typeof useRuntimeConfig>,
) {
  status.value = { state: 'loading', count: 0 };
  const baseURL = String(config.public.baseURL).replace(/\/?$/, '/');
  const discoveryOrigin = new URL(`${baseURL}discovery/`, location.origin);
  const securityOrigin = new URL(`${baseURL}security/`, location.origin);
  discoveryPromise ??= loadDiscovery({
    origin: discoveryOrigin,
    trust: {
      keyID: String(config.public.discoveryKeyID),
      publicKeyBase64: String(config.public.discoveryPublicKey),
    },
    cache: new BrowserDiscoveryCache(discoveryOrigin),
  });
  securityPromise ??= loadSecurity({
    origin: securityOrigin,
    trust: {
      keyID: String(config.public.discoveryKeyID),
      publicKeyBase64: String(config.public.discoveryPublicKey),
    },
  })
    .then((bundle) => bundle.snapshot)
    .catch(() => undefined);

  try {
    const bundle = await discoveryPromise;
    if (activeKey.value !== key || !registry.value) return;
    await waitForCatalogInteractionToFinish();
    if (activeKey.value !== key || !registry.value) return;
    const discovered = bundle.search.records.map((record) =>
      discoveryPlugin(record, bundle.snapshot),
    );
    registry.value.plugins = [...seed.plugins, ...discovered];
    status.value = {
      state: bundle.source === 'remote' ? 'current' : 'cached',
      count: discovered.length,
      sequence: bundle.snapshot.sequence,
      generatedAt: bundle.snapshot.generated_at,
    };

    const security = await securityPromise;
    if (!security || activeKey.value !== key || !registry.value) return;
    await waitForCatalogInteractionToFinish();
    if (activeKey.value !== key || !registry.value) return;
    registry.value.plugins = registry.value.plugins.map((plugin) =>
      applySecurityAssessment(plugin, security),
    );
  } catch (error) {
    if (activeKey.value !== key || !registry.value) return;
    registry.value.plugins = [...seed.plugins];
    status.value = {
      state:
        error instanceof Error && /stale|expired/i.test(error.message) ? 'stale' : 'unavailable',
      count: 0,
      message: error instanceof Error ? error.message : 'Signed Discovery Index is unavailable',
    };
  }
}

function waitForCatalogInteractionToFinish(): Promise<void> {
  const isCatalogInteraction = () =>
    document.activeElement instanceof Element &&
    Boolean(
      document.activeElement.closest(
        '.catalog, .app-combobox__content, .app-select__content, .app-multiselect__content',
      ),
    );
  if (!isCatalogInteraction()) return Promise.resolve();
  return new Promise((resolve) => {
    const observe = () =>
      queueMicrotask(() => {
        if (isCatalogInteraction()) return;
        document.removeEventListener('focusin', observe);
        document.removeEventListener('focusout', observe);
        resolve();
      });
    document.addEventListener('focusin', observe);
    document.addEventListener('focusout', observe);
  });
}
