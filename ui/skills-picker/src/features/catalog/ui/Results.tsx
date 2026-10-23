import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ArrowUpRight, FileText, FolderSearch } from '../../../shared/ui/icons';
import type { Candidate } from '../../../shared/api/types';
import { Tick } from '../../../shared/ui/primitives';

export function Results({ candidates, selected, activeId, query, locked, choose, preview, reveal }: {
  candidates: Candidate[]; selected: string[]; activeId?: string; query: string; locked: boolean;
  choose: (ids: string[], checked: boolean) => void; preview: (id: string) => void; reveal: (candidate: Candidate) => void;
}) {
  const scroll = useRef<HTMLDivElement>(null);
  const virtual = useVirtualizer({ count: candidates.length, getScrollElement: () => scroll.current,
    estimateSize: () => 100, overscan: 5, getItemKey: (index) => candidates[index].id });
  const eligible = candidates.filter((c) => c.status === 'available').map((c) => c.id);
  const all = eligible.length > 0 && eligible.every((id) => selected.includes(id));
  return <section className="results" aria-label="Skills">
    <div className="results-heading"><div><h2>{query ? 'Search results' : 'Explore skills'}</h2><span className="muted">{candidates.length} {candidates.length === 1 ? 'skill' : 'skills'}</span></div>
      <button disabled={locked || !eligible.length} className="text-button" onClick={() => choose(eligible, !all)}>{all ? 'Deselect' : 'Select'} {eligible.length} {query ? 'results' : 'shown'}</button></div>
    {!candidates.length ? <div className="empty"><FolderSearch size={36}/><h3>{query ? 'No matching skills' : 'No skills here yet'}</h3><p>Try another folder or a broader search.</p></div> :
      <div ref={scroll} className="result-scroll"><div style={{ height: virtual.getTotalSize(), position: 'relative' }}>
        {virtual.getVirtualItems().map((row) => {
          const candidate = candidates[row.index];
          return <article key={candidate.id} className={`skill-card ${activeId === candidate.id ? 'active' : ''} ${selected.includes(candidate.id) ? 'is-checked' : ''}`}
            data-index={row.index} ref={virtual.measureElement} style={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${row.start}px)` }}>
            <Tick label={`Select ${candidate.path || candidate.name}`} checked={selected.includes(candidate.id)} disabled={locked || candidate.status !== 'available'} onChange={(checked) => choose([candidate.id], checked)}/>
            <button className="skill-description" onClick={() => preview(candidate.id)}><strong><FileText size={16}/>{candidate.name}</strong><span>{candidate.description || 'No description provided.'}</span><small>{candidate.path || '/ (repository root)'}</small></button>
            <button className="icon-button reveal" title="Show in tree" aria-label={`Show ${candidate.path} in tree`} onClick={() => reveal(candidate)}><ArrowUpRight size={16}/></button>
            {candidate.status !== 'available' && <span className="unavailable">{candidate.status} · {candidate.diagnosticCodes.join(', ')}</span>}
          </article>;
        })}
      </div></div>}
  </section>;
}
