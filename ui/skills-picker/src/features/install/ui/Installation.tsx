import { CheckCircle2, ArrowRight, AlertTriangle } from '../../../shared/ui/icons';
import { useInstallation } from '../api/controller';
import type { Diagnostic, PlanRequest, Session } from '../../../shared/api/types';
import { Busy, Modal, Problem, Tick } from '../../../shared/ui/primitives';
import { useEffect, useState } from 'react';

export type InstallationController = ReturnType<typeof useInstallation>;
function Diagnostics({ items }: { items: Diagnostic[] }) {
  return <>{items.map((item, index) => <div key={`${item.code}:${index}`} className={`notice ${item.severity === 'error' ? 'error' : ''}`}><strong>{item.code}</strong> {item.message}{item.path && <small>{item.path}</small>}</div>)}</>;
}
export function Targets({ controller, selected, setSelected, locked }: {
  controller: InstallationController; selected: string[]; setSelected: (value: string[]) => void; locked: boolean;
}) {
  const { targets } = controller;
  const available = targets.data?.targets.filter((target) => target.available) ?? [];
  const unavailable = targets.data?.targets.filter((target) => !target.available) ?? [];
  const renderTarget = (target: NonNullable<typeof targets.data>['targets'][number]) => <label key={target.id} className={`target-option ${selected.includes(target.id) ? 'active' : ''}`}>
    <Tick label={`Target ${target.label}`} checked={selected.includes(target.id)} disabled={locked || !target.available}
      onChange={(checked) => setSelected(checked ? [...selected, target.id] : selected.filter((id) => id !== target.id))}/>
    <span><strong>{target.label}</strong><small className="target-path" title={`${target.scope} · ${target.displayPath}`}>{target.scope} · {target.displayPath}</small>{!target.available && <small>{target.reason}</small>}</span>
  </label>;
  return <section className="targets" aria-label="Install targets"><h2>Install to</h2><p className="muted">Choose where these skills will be available.</p>
    {targets.isPending ? <Busy>Finding targets…</Busy> : targets.isError ? <Problem error={targets.error} retry={() => void targets.refetch()}/> : targets.data.targets.length === 0 ? <p className="notice">No supported targets are available.</p> :
      <>{available.length ? available.map(renderTarget) : <p className="notice">No targets are currently available. Expand the list below for details.</p>}
        {unavailable.length > 0 && <details className="unavailable-targets"><summary>{unavailable.length} unavailable {unavailable.length === 1 ? 'target' : 'targets'}</summary>{unavailable.map(renderTarget)}</details>}
      </>}
  </section>;
}
export function InstallActions({ controller, request }: { controller: InstallationController; request: PlanRequest }) {
  const { prepare, plan, operation, apply, uncertain, active, locked } = controller;
  const [deadlinePassed, setDeadlinePassed] = useState(false);
  useEffect(() => {
    const remaining = plan ? Date.parse(plan.expiresAt) - Date.now() : Infinity;
    setDeadlinePassed(remaining <= 0);
    if (!controller.reviewOpen || !Number.isFinite(remaining) || remaining <= 0) return;
    const timer = window.setTimeout(() => setDeadlinePassed(true), Math.min(remaining, 2_147_483_647));
    return () => window.clearTimeout(timer);
  }, [plan?.planId, plan?.expiresAt, controller.reviewOpen]);
  const safe = !!plan && plan.confirmable && !plan.conflicts.length && plan.actions.every((action) => ['install', 'update', 'unchanged'].includes(action.action));
  const expired = !!plan && (deadlinePassed || Date.parse(plan.expiresAt) <= Date.now());
  const previousOperation = !!operation.data && !active && request.selectedIds.length > 0 && operation.data.planId !== plan?.planId;
  return <>
    <div className="install-action"><button className="primary" disabled={locked || !request.selectedIds.length || !request.targetIds.length} onClick={(event) => controller.prepareReview(event.currentTarget)}>
      {prepare.isPending ? <Busy>Checking installation…</Busy> : <>Review installation <ArrowRight size={16}/></>}</button>
      <small>No changes until you confirm.</small></div>
    {prepare.isError && <Problem error={prepare.error}/>}
    {apply.isError && !uncertain && <Problem error={apply.error}/>}
    {uncertain && <div role="alert" className="notice"><strong>Installation status is unknown</strong><p>The connection was interrupted. Check the accepted operation before starting another review.</p>
      <button onClick={controller.reconnect}>Reconnect</button> <button disabled={apply.isPending || active} onClick={controller.confirm}>Repeat confirmation of the same plan</button></div>}
    {operation.data && <section className="operation" aria-label="Installation progress" aria-live="polite">
      {previousOperation && <p className="muted"><strong>Previous installation result</strong></p>}
      <h2>{operation.data.state === 'succeeded' ? <><CheckCircle2 size={18}/> Installation complete</> : operation.data.state === 'partial' || operation.data.state === 'failed' ? <><AlertTriangle size={18}/> Installation needs attention</> : <Busy>{operation.data.phase || 'Installing…'}</Busy>}</h2>
      <progress max={Math.max(1, operation.data.total)} value={operation.data.completed}/>
      <p className="muted">{operation.data.completed} / {operation.data.total} · {operation.data.state}</p>
      {operation.data.items.map((item, index) => <div className="operation-item" key={index}><strong>{item.alias || item.candidateId}</strong><span>{item.targetId} · {item.status}</span><small>{item.destinationPath}</small>{item.message && <p>{item.message}</p>}</div>)}
      <Diagnostics items={operation.data.diagnostics}/>
      {operation.data.result && <><p>{operation.data.result.message}</p>{operation.data.result.repairInstructions.map((instruction) => <p key={instruction} className="notice">{instruction}</p>)}</>}
    </section>}
    {operation.isError && <Problem error={operation.error} retry={controller.reconnect}/>}
    <Modal open={controller.reviewOpen && !!plan && !uncertain} returnFocus={() => controller.reviewTrigger.current} title="Review installation" onOpenChange={(open) => { if (!open && !apply.isPending) controller.closeReview(); }}>
      {plan && <><p className="muted">Check the exact destinations. Your files change only after confirmation.</p>
        <div className="plan-actions">{plan.actions.map((action, index) => <div key={index} className="plan-action"><span className="action-badge">{action.action}</span><div><strong>{action.alias} <span className="muted">→ {action.targetId}</span></strong><small>{action.destinationPath}</small>
          {action.sourceMoved && <p className="notice">Source moved: {action.sourceMoved.from} → {action.sourceMoved.to}</p>}{action.reasonCodes.length > 0 && <small>{action.reasonCodes.join(', ')}</small>}</div></div>)}</div>
        <Diagnostics items={plan.warnings}/><Diagnostics items={plan.conflicts}/>
        {expired && <p className="notice error">This plan expired. Close and review again.</p>}
        {!safe && <p className="notice error">This plan cannot be confirmed. Resolve the listed issues and review again.</p>}
        <div className="modal-actions"><button disabled={apply.isPending} onClick={controller.closeReview}>Back</button><button className="primary" disabled={!safe || expired || apply.isPending} onClick={controller.confirm}>{apply.isPending ? <Busy>Confirming…</Busy> : 'Confirm installation'}</button></div>
      </>}
    </Modal>
  </>;
}
export { useInstallation };
