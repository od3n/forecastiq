import type { ReactNode } from "react";
import styles from "./EmptyState.module.css";

/** Empty-state variants (doc 02 §13.3; state contracts §1). */
export type EmptyStateVariant =
  | "no-locations"
  | "no-data"
  | "insufficient"
  | "observation-unavailable";

export interface EmptyStateProps {
  variant: EmptyStateVariant;
  title: string;
  description: string;
  /** Optional action (e.g. an admin "Add Location" CTA). */
  action?: ReactNode;
}

// Generic empty-state panel. Never renders broken charts, empty axes, or
// "undefined" labels (doc 02 §13.3 — the UI always explains the absence).
export function EmptyState({ variant, title, description, action }: EmptyStateProps) {
  return (
    <div className={styles.panel} data-variant={variant}>
      <p className={styles.title}>{title}</p>
      <p className={styles.body}>{description}</p>
      {action}
    </div>
  );
}
