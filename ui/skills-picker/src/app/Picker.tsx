import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { ArrowUpRight, Check, ChevronRight, GitBranch, Search, ShieldCheck, ShoppingBag, X } from '../shared/ui/icons';
import { api, APIError } from '../shared/api/client';
import type { Catalog, Session } from '../shared/api/types';
import { ancestors, filterCatalog, Preview, Results, Tree, useCatalog, useExpansion } from '../features/catalog';
import { Basket, useSelection } from '../features/selection';
import { InstallActions, Targets, useInstallation } from '../features/install';
import { Busy, Modal, Problem } from '../shared/ui/primitives';

export type BrowseSearch = { q: string; path: string; view: 'tree' | 'results'; skill?: string };
export function validateSearch(input: Record<string, unknown>): BrowseSearch {
  return { q: typeof input.q === 'string' ? input.q.slice(0, 512) : '', path: typeof input.path === 'string' ? input.path.slice(0, 4096) : '',
    view: input.view === 'results' ? 'results' : 'tree', skill: typeof input.skill === 'string' ? input.skill.slice(0, 256) : undefined };
}
export function Picker() {
  const [expired, setExpired] = useState(false);
  useEffect(() => { const expire = () => setExpired(true); window.addEventListener('uap-session-expired', expire); return () => window.removeEventListener('uap-session-expired', expire); }, []);
  const session = useQuery({ queryKey: ['session'], queryFn: ({ signal }) => api.session(signal), enabled: !expired,
    retry: (count, error) => !(error instanceof APIError) && count < 1,
    refetchInterval: (query) => !expired && query.state.data && ['resolving', 'scanning', 'installing'].includes(query.state.data.state) ? 1000 : false });
  const catalog = useCatalog(session.data?.sessionId, session.data?.catalogId);
  if (expired) return <main className="startup"><h1>This picker session has ended</h1><p>Return to your terminal and start a new Skills command.</p></main>;
  if (session.isError) return <main className="startup"><h1>Connect to your CLI</h1><Problem error={session.error} retry={() => void session.refetch()}/><p>Open the complete URL printed by your running CLI.</p></main>;
  if (session.data?.state === 'failed' && !session.data.catalogId) return <main className="startup"><h1>The repository scan failed</h1><p>Return to your terminal for details and run the Skills command again.</p></main>;
  if (!session.data || !session.data.catalogId || ['resolving', 'scanning'].includes(session.data.state)) return <main className="startup"><div className="brand-mark">u</div><h1>Finding your next skills</h1><Busy>{session.data?.state === 'resolving' ? 'Resolving repository…' : 'Scanning skills…'}</Busy><p>{session.data?.scan.validated ?? 0} validated / {session.data?.scan.discovered ?? 0} discovered</p>
    </main>;
  if (catalog.isError) return <main className="startup"><Problem error={catalog.error} retry={() => void catalog.refetch()}/></main>;
  if (!catalog.data || catalog.data.catalogId !== session.data.catalogId) return <main className="startup"><Busy>Loading catalog…</Busy></main>;
  return <Workspace key={`${session.data.sessionId}:${catalog.data.catalogId}`} session={session.data} catalog={catalog.data}/>;
}

function Workspace({ session, catalog }: { session: Session; catalog: Catalog }) {
  const search = useSearch({ strict: false }) as BrowseSearch;
  const navigate = useNavigate();
  const updateSearch = (patch: Partial<BrowseSearch>, replace = false) => void navigate({ to: '/', search: { ...search, ...patch }, replace });
  const selection = useSelection();
  const [targetIds, setTargetIds] = useState<string[]>([]);
  const [mobilePanel, setMobilePanel] = useState<'skills' | 'folders' | 'selection'>('skills');
  const [scopeOpen, setScopeOpen] = useState(false), [scopePath, setScopePath] = useState(catalog.scope);
  const [narrow, setNarrow] = useState(() => window.matchMedia('(max-width:1099px)').matches);
  useEffect(() => { const media = window.matchMedia('(max-width:1099px)'); const update = () => setNarrow(media.matches); media.addEventListener('change', update); return () => media.removeEventListener('change', update); }, []);
  const cache = useQueryClient();
  const selected = selection.catalogId === catalog.catalogId ? selection.ids : [];
  const chosen = catalog.candidates.filter((candidate) => selected.includes(candidate.id));
  const request = { catalogId: catalog.catalogId, catalogDigest: catalog.catalogDigest, selectedIds: [...selected].sort(), targetIds: [...targetIds].sort() };
  const scan = useMutation({ mutationFn: api.scan, retry: false,
    onSuccess: (next) => {
      selection.reset(''); useExpansion.getState().reset(); updateSearch({ skill: undefined, path: '', q: '' });
      cache.setQueryData(['session'], next); setScopeOpen(false); void cache.invalidateQueries({ queryKey: ['session'] });
    },
  });
  const install = useInstallation(session, request, scan.isPending);
  const locked = install.locked || scan.isPending;
  useEffect(() => { if (selection.catalogId !== catalog.catalogId) { selection.reset(catalog.catalogId); useExpansion.getState().reset(); } }, [catalog.catalogId, selection.catalogId, selection.reset]);
  const results = useMemo(() => filterCatalog(catalog.candidates, search.q, search.path), [catalog.candidates, search.q, search.path]);
  const activeId = catalog.candidates.some((candidate) => candidate.id === search.skill) ? search.skill : undefined;
  const eligible = new Set(catalog.candidates.filter((candidate) => candidate.status === 'available').map((candidate) => candidate.id));
  const choose = (ids: string[], checked: boolean) => { if (!locked) selection.setChecked(ids.filter((id) => eligible.has(id)), checked); };
  const preview = (id: string) => updateSearch({ skill: id });
  const reveal = (candidate: Catalog['candidates'][number]) => {
    useExpansion.getState().set((old) => [...new Set([...old, ...ancestors(candidate.path)])]);
    updateSearch({ q: '', path: candidate.path, view: 'tree' }); setMobilePanel('folders');
  };
  return <main className="picker-shell">
    <header className="topbar"><a className="brand" href="#" onClick={(event) => event.preventDefault()}><span className="brand-mark">u</span><span>universal agent plugins</span></a><span className="local-badge"><span/> Local session</span></header>
    <div className="page-heading"><div className="eyebrow">YOUR TOOLKIT, EXPANDED</div><h1>Find your next skill<span>.</span></h1><p>Explore this repository and choose what your agents learn next.</p></div>
    <div className="source-bar"><GitBranch size={19}/><strong>{catalog.repository}</strong><span className="revision" title={catalog.sourceRevision}>{catalog.sourceRevision.slice(0, 9)}</span><span className="catalog-status">{catalog.complete ? <><Check size={13}/> Scan complete</> : 'Partial catalog'}</span><button className="text-button" disabled={locked} onClick={() => { setScopePath(catalog.scope); setScopeOpen(true); }}>Change scan scope <ArrowUpRight size={13}/></button></div>
    {!catalog.complete && <div className="notice"><strong>This catalog is incomplete.</strong> Showing validated discoveries in <code>{catalog.scope || '/'}</code>. Narrow the scan to discover more; selection does not imply repository completeness.</div>}
    {catalog.diagnostics.length > 0 && <details className="scan-diagnostics"><summary>{catalog.diagnostics.length} scan diagnostics</summary>{catalog.diagnostics.map((item, index) => <p key={index}><strong>{item.code}</strong> {item.message} {item.path}</p>)}</details>}
    <div className="search-row"><Search size={19}/><input id="skill-search" aria-label="Search skills" placeholder="Search skills, descriptions, or paths…" maxLength={512} value={search.q} onChange={(event) => updateSearch({ q: event.target.value }, true)}/>{search.q && <button className="icon-button" aria-label="Clear search" onClick={() => updateSearch({ q: '' }, true)}><X size={16}/></button>}<span className="search-hint">{catalog.counts.total} skills</span></div>
    <div className="breadcrumb"><span>Scan scope: {catalog.scope || 'Entire repository'}</span>{search.path && <><ChevronRight size={13}/><button className="filter-chip" onClick={() => updateSearch({ path: '' })}>Inside {search.path} <X size={12}/></button></>}</div>
    <div className="mobile-tabs" role="tablist" aria-label="Picker panels">{(['skills', 'folders', 'selection'] as const).map((panel) => <button role="tab" aria-selected={mobilePanel === panel} key={panel} onClick={() => setMobilePanel(panel)}>{panel === 'selection' ? `Selection (${selected.length})` : panel}</button>)}</div>
    <div className={`workspace ${activeId ? 'with-preview' : ''}`} data-mobile-panel={mobilePanel}>
      <div className="tree-column"><Tree key={catalog.catalogId} candidates={catalog.candidates} selected={selected} locked={locked} path={search.path} choose={choose} browse={(path) => { updateSearch({ path, q: '', view: 'tree' }); setMobilePanel('skills'); }} preview={preview}/></div>
      <Results candidates={results} selected={selected} activeId={activeId} query={search.q} locked={locked} choose={choose} preview={preview} reveal={reveal}/>
      {activeId && <div className="desktop-preview"><Preview sessionId={session.sessionId} catalogId={catalog.catalogId} candidateId={activeId} close={() => updateSearch({ skill: undefined })}/></div>}
      <aside className="sidebar">
        <Basket candidates={chosen} locked={locked} remove={(id) => choose([id], false)} clear={() => choose(selected, false)}/>
        <Targets controller={install} selected={targetIds} setSelected={setTargetIds} locked={locked}/>
        <InstallActions controller={install} request={request}/>
      </aside>
    </div>
    <footer className="bottom-bar"><span><ShoppingBag size={16}/><strong>{selected.length} selected</strong><span className="muted">across this repository</span></span><span className="security-note"><ShieldCheck size={15}/> Installed locally. Reviewed before changes.</span><button className="primary" disabled={locked || !selected.length || !targetIds.length} onClick={(event) => install.prepareReview(event.currentTarget)}>{install.prepare.isPending ? <Busy>Checking installation…</Busy> : <>Review installation <ArrowRightIcon/></>}</button></footer>
    {narrow && <Modal open={!!activeId} title="Skill preview" onOpenChange={(open) => { if (!open) updateSearch({ skill: undefined }); }}><Preview sessionId={session.sessionId} catalogId={catalog.catalogId} candidateId={activeId} close={() => updateSearch({ skill: undefined })}/></Modal>}
    <Modal open={scopeOpen} title="Change scan scope" onOpenChange={(open) => { if (!scan.isPending) setScopeOpen(open); }}>
      <p>Scan another folder on the same commit. Leave empty to scan the whole repository.</p><label className="field-label">Repository folder<input value={scopePath} onChange={(event) => setScopePath(event.target.value)} maxLength={4096} placeholder="e.g. skills/design"/></label>
      {selected.length > 0 && <p className="notice">Starting a new scan clears your {selected.length} selected skills and current review.</p>}{scan.isError && <Problem error={scan.error}/>}
      <div className="modal-actions"><button disabled={scan.isPending} onClick={() => setScopeOpen(false)}>Back</button><button className="primary" disabled={locked} onClick={() => { install.invalidateReview(); scan.mutate(scopePath); }}>{scan.isPending ? <Busy>Scanning…</Busy> : 'Scan folder'}</button></div>
    </Modal>
  </main>;
}
function ArrowRightIcon() { return <ChevronRight size={16}/>; }
