export interface DiscoveryStatus {
  state: 'idle' | 'loading' | 'current' | 'cached' | 'stale' | 'unavailable';
  count: number;
  sequence?: number;
  generatedAt?: string;
  message?: string;
}

export function useDiscoveryStatus() {
  return useState<DiscoveryStatus>('discovery-status', () => ({ state: 'idle', count: 0 }));
}
