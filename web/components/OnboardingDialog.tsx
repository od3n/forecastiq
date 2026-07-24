"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import styles from "./OnboardingDialog.module.css";

const STORAGE_KEY = "fiq_onboarding_dismissed";

export function isOnboardingDismissed(): boolean {
  if (typeof window === "undefined") return true;
  return localStorage.getItem(STORAGE_KEY) === "1";
}

export function dismissOnboarding(): void {
  localStorage.setItem(STORAGE_KEY, "1");
}

export function resetOnboarding(): void {
  localStorage.removeItem(STORAGE_KEY);
}

export interface OnboardingDialogProps {
  open: boolean;
  onDismiss: () => void;
}

// S-07 Onboarding dialog (doc 02 §screen-inventory §3). Shown once per account
// on first authenticated load; dismissible; re-openable from Settings. Focus-
// trapped (role="dialog", aria-modal, Escape dismisses, focus returns on close).
export function OnboardingDialog({ open, onDismiss }: OnboardingDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (open) {
      previousFocus.current = document.activeElement as HTMLElement;
      dialogRef.current?.focus();
    } else {
      previousFocus.current?.focus();
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onDismiss();
      // Focus trap: Tab cycles within dialog.
      if (e.key === "Tab" && dialogRef.current) {
        const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
          "a, button, input, select, textarea, [tabindex]:not([tabindex='-1'])",
        );
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open, onDismiss]);

  if (!open) return null;

  return (
    <div className={styles.overlay} onClick={onDismiss}>
      <div
        ref={dialogRef}
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="onboarding-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="onboarding-title" className={styles.title}>
          Welcome to ForecastIQ
        </h2>
        <div className={styles.body}>
          <p>
            <strong>What we measure:</strong> forecast accuracy — how close weather
            providers&apos; predictions are to observed reality, using standardized
            metrics (MAE, RMSE, Bias) with confidence intervals.
          </p>
          <p>
            <strong>What we don&apos;t deliver:</strong> weather forecasts. We compare
            them, not produce them. All data is attributed to its source provider.
          </p>
          <p style={{ marginTop: "var(--space-sm)" }}>
            Pick your default location and explore the{" "}
            <Link href="/methodology">Methodology</Link> to understand how scores
            are computed.
          </p>
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.primaryBtn} onClick={onDismiss}>
            Got it
          </button>
        </div>
      </div>
    </div>
  );
}
