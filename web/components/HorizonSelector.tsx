"use client";

import { HORIZON_OPTIONS } from "@/lib/state/use-global-params";
import styles from "./HorizonSelector.module.css";

export interface HorizonSelectorProps {
  selected: number;
  onChange: (minutes: number) => void;
  /** When set, only these horizons are selectable; the rest render disabled. */
  allowedMinutes?: number[];
}

// Segmented pill control for the forecast horizon (doc 02 §3.1). Global
// control; persisted in the URL. role="radiogroup" for a11y (doc 07 §2 S-04).
export function HorizonSelector({ selected, onChange, allowedMinutes }: HorizonSelectorProps) {
  return (
    <div className={styles.group} role="radiogroup" aria-label="Forecast horizon">
      {HORIZON_OPTIONS.map((opt) => {
        const disabled = allowedMinutes !== undefined && !allowedMinutes.includes(opt.minutes);
        return (
          <button
            key={opt.minutes}
            type="button"
            role="radio"
            aria-checked={selected === opt.minutes}
            disabled={disabled}
            title={disabled ? "This view always shows the full 24-hour day" : undefined}
            className={`${styles.pill} ${selected === opt.minutes ? styles.active : ""}`}
            style={disabled ? { opacity: 0.4, cursor: "not-allowed" } : undefined}
            onClick={() => onChange(opt.minutes)}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
