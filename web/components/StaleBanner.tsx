import { absoluteLocal, relativeTime } from "@/lib/format";
import styles from "./StaleBanner.module.css";

export interface StaleBannerProps {
  lastUpdated: string;
}

// Persistent, non-dismissible stale banner shown WITH the data (doc 02 §13.5;
// BR-FRESH-01). role="status" + aria-live="polite" (a11y). Stale data is never
// served silently as current.
export function StaleBanner({ lastUpdated }: StaleBannerProps) {
  return (
    <div className={styles.banner} role="status" aria-live="polite">
      <span aria-hidden="true">⚠ </span>
      Data may be out of date — last updated {relativeTime(lastUpdated)} ({absoluteLocal(lastUpdated)}).
    </div>
  );
}
