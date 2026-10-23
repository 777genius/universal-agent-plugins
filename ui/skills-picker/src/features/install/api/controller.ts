import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { api, APIError } from '../../../shared/api/client';
import type { PlanRequest, Session } from '../../../shared/api/types';

export function useInstallation(session: Session, request: PlanRequest, scanPending: boolean) {
  const cache = useQueryClient();
  const identity = JSON.stringify(request);
  const current = useRef(identity); current.current = identity;
  const [reviewIdentity, setReviewIdentity] = useState('');
  const [uncertain, setUncertain] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const reviewTrigger = useRef<HTMLElement | null>(null);
  const targets = useQuery({ queryKey: ['targets', session.sessionId], queryFn: ({ signal }) => api.targets(signal) });
  const prepare = useMutation({ mutationFn: api.plan, retry: false,
    onSuccess: (_plan, input) => { if (current.current === JSON.stringify(input)) { setReviewIdentity(current.current); setReviewOpen(true); } },
  });
  const plan = reviewIdentity === identity && !scanPending ? prepare.data : undefined;
  const operation = useQuery({ queryKey: ['operation', session.sessionId, session.operationId],
    queryFn: ({ signal }) => api.operation(session.operationId!, signal), enabled: !!session.operationId,
    refetchInterval: (query) => query.state.data && ['queued', 'running'].includes(query.state.data.state) ? 1000 : false,
  });
  const accepted = !!operation.data && operation.data.planId === plan?.planId;
  useEffect(() => { if (accepted) { setUncertain(false); setReviewOpen(false); } }, [accepted]);
  const active = session.state === 'installing' || (!!operation.data && ['queued', 'running'].includes(operation.data.state));
  const apply = useMutation({ mutationFn: api.apply, retry: false,
    onSuccess: (result) => {
      setUncertain(false); setReviewOpen(false);
      cache.setQueryData<Session>(['session'], (previous) => previous && ({ ...previous, operationId: result.operationId, state: 'installing' }));
      void cache.invalidateQueries({ queryKey: ['session'] });
    },
    onError: (error) => {
      if (error instanceof APIError && error.status >= 400 && error.status < 500 && error.code !== 'OPERATION_IN_PROGRESS') {
        setReviewIdentity(''); setReviewOpen(false);
      } else setUncertain(true);
      void cache.invalidateQueries({ queryKey: ['session'] });
    },
  });
  const terminalId = operation.data && !['queued', 'running'].includes(operation.data.state) ? operation.data.operationId : '';
  useEffect(() => {
    if (terminalId) {
      void cache.invalidateQueries({ queryKey: ['targets', session.sessionId] });
      void cache.invalidateQueries({ queryKey: ['session'] });
    }
  }, [terminalId, cache, session.sessionId]);
  const locked = prepare.isPending || apply.isPending || uncertain || active;
  return { targets, prepare, plan, operation, apply, uncertain, active, locked, reviewOpen,
    invalidateReview: () => { setReviewIdentity(''); setReviewOpen(false); prepare.reset(); },
    closeReview: () => setReviewOpen(false),
    reviewTrigger,
    prepareReview: (trigger?: HTMLElement) => { if (!locked && !scanPending) { reviewTrigger.current = trigger ?? null; apply.reset(); setReviewIdentity(''); prepare.mutate(request); } },
    confirm: () => { if (plan && !apply.isPending && !active) apply.mutate(plan.planId); },
    reconnect: () => { void cache.invalidateQueries({ queryKey: ['session'] }); void operation.refetch(); },
  };
}
