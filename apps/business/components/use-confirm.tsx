"use client";
import { useCallback, useRef, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { useDialogFocusTrap } from "./use-dialog-focus-trap";

export type ConfirmRequest = {
  title: string;
  description?: string;
  /** Wording of the button that goes ahead, e.g. "Удалить". */
  confirmLabel: string;
};

type Pending = ConfirmRequest & { settle: (confirmed: boolean) => void };

/**
 * Replacement for window.confirm. The native dialog is unstyled, blocks the
 * page, and browsers let people silence it after the second one — none of which
 * is acceptable for an action that deletes a record.
 *
 * Render the returned `dialog` inside the component, and await `ask` where the
 * old code called confirm().
 */
export function useConfirm() {
  const [pending, setPending] = useState<Pending | null>(null);
  const settleRef = useRef<((confirmed: boolean) => void) | null>(null);

  const close = useCallback((confirmed: boolean) => {
    settleRef.current?.(confirmed);
    settleRef.current = null;
    setPending(null);
  }, []);

  const dialogRef = useDialogFocusTrap<HTMLDivElement>(pending !== null, () => close(false));

  const ask = useCallback(
    (request: ConfirmRequest) =>
      new Promise<boolean>(resolve => {
        settleRef.current = resolve;
        setPending({ ...request, settle: resolve });
      }),
    [],
  );

  const dialog = pending ? (
    <div className="confirm-backdrop" role="presentation" onMouseDown={() => close(false)}>
      <div
        ref={dialogRef}
        className="confirm-dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby={pending.description ? "confirm-description" : undefined}
        onMouseDown={event => event.stopPropagation()}
      >
        <span className="confirm-icon">
          <AlertTriangle />
        </span>
        <h2 id="confirm-title">{pending.title}</h2>
        {pending.description && <p id="confirm-description">{pending.description}</p>}
        <div className="confirm-actions">
          <button type="button" onClick={() => close(false)}>
            Отмена
          </button>
          <button type="button" className="confirm-proceed" onClick={() => close(true)}>
            {pending.confirmLabel}
          </button>
        </div>
      </div>
    </div>
  ) : null;

  return { ask, dialog };
}
