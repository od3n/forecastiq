import type { FreshnessState } from "@/lib/api/types";
import { relativeTime } from "@/lib/format";
import styles from "./FreshnessBadge.module.css";

export interface FreshnessBadgeProps {
  state: FreshnessState;
  lastUpdated?: string;
}

const WORD: Record<FreshnessState, string> = {
  fresh: "Fresh",
  delayed: "Delayed",
  stale: "Stale",
  unavailable: "Unavailable",
};

// Freshness indicator (doc 02 §2.4/§1.5). Color is never the only channel: a
// text label is always rendered alongside the dot/pill (a11y §3, A-02). `fresh`
// is a green dot + "Updated {relative}"; the other states are solid pills.
export function FreshnessBadge({ state, lastUpdated }: FreshnessBadgeProps) {
  const ariaLabel = `Data freshness: ${WORD[state]}`;
  if (state === "fresh") {
    return (
      <span className={`${styles.badge} ${styles.fresh}`} aria-label={ariaLabel}>
        <span className={styles.dot} aria-hidden="true" />
        <span className={styles.label}>
          {lastUpdated ? `Updated ${relativeTime(lastUpdated)}` : "Fresh"}
        </span>
      </span>
    );
  }
  return (
    <span className={styles.badge} aria-label={ariaLabel}>
      <span className={`${styles.pill} ${styles[state]}`}>{WORD[state]}</span>
    </span>
  );
}
