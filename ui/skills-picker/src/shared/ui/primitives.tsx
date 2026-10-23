import { Dialog } from '@base-ui/react/dialog';
import { Checkbox } from '@base-ui/react/checkbox';
import { Check, Minus, X, LoaderCircle } from './icons';
import { useRef, type ReactNode } from 'react';

export function Tick({ checked, mixed = false, disabled, label, onChange }: {
  checked: boolean; mixed?: boolean; disabled?: boolean; label: string; onChange: (checked: boolean) => void;
}) {
  return <Checkbox.Root className="checkbox" aria-label={label} checked={checked} indeterminate={mixed}
    disabled={disabled} onCheckedChange={onChange}>
    <Checkbox.Indicator>{mixed ? <Minus size={13}/> : <Check size={13}/>}</Checkbox.Indicator>
  </Checkbox.Root>;
}

export function Modal({ open, onOpenChange, title, children, returnFocus }: {
  open: boolean; onOpenChange: (open: boolean) => void; title: string; children: ReactNode; returnFocus?: () => HTMLElement | null;
}) {
  const origin = useRef<HTMLElement | null>(null);
  const wasOpen = useRef(false);
  if (open && !wasOpen.current) origin.current = document.activeElement instanceof HTMLElement && document.activeElement !== document.body ? document.activeElement : null;
  wasOpen.current = open;
  return <Dialog.Root open={open} onOpenChange={onOpenChange}>
    <Dialog.Portal><Dialog.Backdrop className="backdrop"/><Dialog.Popup className="modal"
      finalFocus={() => {
        const trigger = returnFocus ? returnFocus() : origin.current;
        if (trigger?.isConnected && !trigger.matches(':disabled, [aria-disabled="true"]') && trigger.getClientRects().length > 0 && getComputedStyle(trigger).visibility === 'visible') return trigger;
        return document.querySelector<HTMLInputElement>('#skill-search');
      }}>
      <div className="panel-heading"><Dialog.Title>{title}</Dialog.Title>
        <Dialog.Close className="icon-button" aria-label="Close"><X size={18}/></Dialog.Close></div>
      {children}
    </Dialog.Popup></Dialog.Portal>
  </Dialog.Root>;
}

export function Busy({ children }: { children: ReactNode }) {
  return <span className="busy"><LoaderCircle size={16} className="spin"/>{children}</span>;
}
export function Problem({ error, retry }: { error: unknown; retry?: () => void }) {
  return <div role="alert" className="notice error"><p>{error instanceof Error ? error.message : 'Something went wrong.'}</p>
    {retry && <button onClick={retry}>Try again</button>}</div>;
}
