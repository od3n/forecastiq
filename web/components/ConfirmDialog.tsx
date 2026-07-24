"use client";

import { useState } from "react";
import styles from "./ConfirmDialog.module.css";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  /** The exact text the user must type to confirm (motor-error guard). */
  confirmText: string;
  description: string;
  actionLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
}

// Typed-confirmation dialog for destructive actions (doc 02 §14.3). The user
// must type the exact confirmText before the action button enables (motor-error
// guard per security reconciliation SEC-10).
export function ConfirmDialog({ open, title, confirmText, description, actionLabel, onConfirm, onCancel }: ConfirmDialogProps) {
  const [input, setInput] = useState("");

  if (!open) return null;

  return (
    <div className={styles.overlay} onClick={onCancel}>
      <div className={styles.dialog} role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-desc" onClick={(e) => e.stopPropagation()}>
        <h2 id="confirm-title" className={styles.title}>{title}</h2>
        <p id="confirm-desc" style={{ color: "var(--color-text-secondary)" }}>{description}</p>
        <label style={{ fontSize: "var(--text-label)", color: "var(--color-text-secondary)" }}>
          Type <strong>{confirmText}</strong> to confirm:
          <input
            className={styles.input}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            autoFocus
            style={{ marginTop: "var(--space-xs)" }}
          />
        </label>
        <div className={styles.actions}>
          <button type="button" className={styles.cancelBtn} onClick={onCancel}>Cancel</button>
          <button type="button" className={styles.dangerBtn} disabled={input !== confirmText} onClick={() => { onConfirm(); setInput(""); }}>
            {actionLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
