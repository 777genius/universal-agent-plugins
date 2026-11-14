import { X, ShoppingBag } from '../../../shared/ui/icons';
import type { Candidate } from '../../../shared/api/types';

export function Basket({ candidates, locked, remove, clear }: {
  candidates: Candidate[]; locked: boolean; remove: (id: string) => void; clear: () => void;
}) {
  return <section className="basket" aria-label="Selection basket">
    <div className="panel-heading"><h2><ShoppingBag size={16}/> Your selection <span className="count">{candidates.length}</span></h2>
      {candidates.length > 0 && <button className="text-button" disabled={locked} onClick={clear}>Clear</button>}</div>
    {candidates.length === 0 ? <p className="muted">Pick skills from any folder. Your selection stays here as you explore.</p> :
      <ul>{candidates.map((candidate) => <li key={candidate.id}><div><strong>{candidate.name}</strong><small>{candidate.path || '/ (repository root)'}</small></div>
        <button className="icon-button" aria-label={`Remove ${candidate.path}`} disabled={locked} onClick={() => remove(candidate.id)}><X size={15}/></button></li>)}</ul>}
  </section>;
}
