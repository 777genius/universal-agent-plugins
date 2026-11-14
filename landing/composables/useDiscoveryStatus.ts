export interface DiscoveryStatus {
  state: 'idle' | 'loading' | 'current' | 'cached' | 'stale' | 'unavailable';
  count: number;
  sequence?: number;
  generatedAt?: string;
  expiresAt?: string;
  message?: string;
}

export function useDiscoveryIsStale(plugin: () => { discovery?: { expires_at: string } }) {
  const status = useDiscoveryStatus();
  return computed(() => {
    const discovery = plugin().discovery;
    const snapshotStale =
      status.value.expiresAt === discovery?.expires_at && status.value.state === 'stale';
    return Boolean(discovery && (snapshotStale || Date.now() >= Date.parse(discovery.expires_at)));
  });
}

export function useDiscoveryStatus() {
  return useState<DiscoveryStatus>('discovery-status', () => ({ state: 'idle', count: 0 }));
}
