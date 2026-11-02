import { useQuery } from '@tanstack/react-query';
import { api } from '../../../shared/api/client';
export function useCatalog(sessionId?: string, catalogId?: string) {
  return useQuery({ queryKey: ['catalog', sessionId, catalogId], queryFn: ({ signal }) => api.catalog(signal), enabled: !!sessionId && !!catalogId,
    staleTime: Infinity, refetchOnWindowFocus: false, refetchOnReconnect: false });
}
export function usePreview(sessionId: string, catalogId: string, candidateId?: string) {
  return useQuery({ queryKey: ['preview', sessionId, catalogId, candidateId], queryFn: ({ signal }) => api.preview(candidateId!, signal),
    enabled: !!candidateId, staleTime: Infinity, refetchOnWindowFocus: false });
}
