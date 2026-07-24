"use client";

import { useEffect, useRef } from "react";
import styles from "./ErrorPanel.module.css";

export interface ErrorPanelProps {
  title?: string;
  message: string;
  /** Correlation id surfaced for support (S-15); never a stack trace. */
  requestId?: string;
  /** When provided, renders a Retry button and moves focus to it (a11y). */
  onRetry?: () => void;
}

// Error panel (doc 02 §13.4; S-15). role="alert"; focus moves to the primary
// action. Displays the request_id for support correlation without leaking
// internals (no stack traces, no provider error detail).
export function ErrorPanel({ title = "Unable to load data", message, requestId, onRetry }: ErrorPanelProps) {
  const retryRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    retryRef.current?.focus();
  }, []);
  return (
    <div className={styles.panel} role="alert">
      <p className={styles.title}>{title}</p>
      <p className={styles.detail}>{message}</p>
      {requestId && <p className={styles.requestId}>Reference: {requestId}</p>}
      {onRetry && (
        <button ref={retryRef} type="button" className={styles.retry} onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}
