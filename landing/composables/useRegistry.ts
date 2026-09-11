import { withAppBase } from '~/utils/localizedRoutes';
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
  const { t } = useI18n();
  const projection = options.projection ?? { kind: 'catalog' };
  const endpoint = registryEndpoint(projection);
  const key = `registry-page:${endpoint}`;
  const config = useRuntimeConfig();
  const registry = useState<RegistryIndex | undefined>('registry-index');
  const activeKey = useState('registry-page-key', () => '');
  // Endpoint identity alone cannot distinguish locale remounts or A → B → A.
  // Claim before the first await and invalidate on scope disposal (including download).
  const generation = useState('registry-page-generation', () => 0);
  const ownGeneration = ++generation.value;
  let disposed = false;
  const isCurrent = () => !disposed && generation.value === ownGeneration;
  onScopeDispose(() => {
    disposed = true;
    if (generation.value === ownGeneration) generation.value++;
  });
  const status = useDiscoveryStatus();
  const seed = shallowRef<RegistryIndex>();

  if (import.meta.client && options.discovery) {
    onMounted(() => {
      if (seed.value && isCurrent()) {
        void augmentWithDiscovery(registry, seed.value, isCurrent, status, config, () =>
          t('registryUi.errors.discoveryUnavailable'),
        );
      }
    });
  }

  const { data, error } = await useAsyncData<RegistryIndex>(
    key,
    () =>
      $fetch<RegistryIndex>(withAppBase(endpoint, String(config.public.baseURL)), {
        responseType: 'json',
      }),
    { deep: false },
  );
  if (error.value || !data.value || !Array.isArray(data.value.plugins)) {
    throw createError({
      statusCode: 500,
      statusMessage: t('registryUi.errors.unavailable'),
      cause: error.value,
    });
  }

  seed.value = structuredClone(data.value);
  if (!isCurrent()) return seed.value;
  activeKey.value = key;
  registry.value = seed.value;
  return registry.value;
}

export function useRegistry(): RegistryIndex {
  const { t } = useI18n();
  const registry = useState<RegistryIndex | undefined>('registry-index');
  if (!registry.value) {
    throw createError({ statusCode: 500, statusMessage: t('registryUi.errors.uninitialized') });
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
  isCurrent: () => boolean,
  status: ReturnType<typeof useDiscoveryStatus>,
  config: ReturnType<typeof useRuntimeConfig>,
  unavailableMessage: () => string,
) {
  if (!isCurrent()) return;
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
    if (!isCurrent() || !registry.value) return;
    await waitForCatalogInteractionToFinish();
    if (!isCurrent() || !registry.value) return;
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
    if (!security || !isCurrent() || !registry.value) return;
    await waitForCatalogInteractionToFinish();
    if (!isCurrent() || !registry.value) return;
    registry.value.plugins = registry.value.plugins.map((plugin) =>
      applySecurityAssessment(plugin, security),
    );
  } catch (error) {
    if (!isCurrent() || !registry.value) return;
    registry.value.plugins = [...seed.plugins];
    status.value = {
      state:
        error instanceof Error && /stale|expired/i.test(error.message) ? 'stale' : 'unavailable',
      count: 0,
      message: error instanceof Error ? error.message : unavailableMessage(),
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
