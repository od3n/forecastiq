import type { Warning } from "@/lib/api/types";
import styles from "./PartialWarnings.module.css";

export interface PartialWarningsProps {
  warnings: Warning[];
}

// Partial-result banner shown above content when some providers are
// unavailable/stale (doc 02 §13.4; state contracts §4). Unaffected providers
// render normally; affected ones are omitted from data and listed here.
// role="note" feeds the message strings to assistive tech.
export function PartialWarnings({ warnings }: PartialWarningsProps) {
  if (!warnings || warnings.length === 0) return null;
  return (
    <div className={styles.banner} role="note">
      Some data is temporarily unavailable.
      <ul className={styles.list}>
        {warnings.map((w, i) => (
          <li key={w.provider_id ?? `${w.code}-${i}`}>{w.message}</li>
        ))}
      </ul>
    </div>
  );
}
