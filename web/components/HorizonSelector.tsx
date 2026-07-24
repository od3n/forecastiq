"use client";

import { HORIZON_OPTIONS } from "@/lib/state/use-global-params";
import styles from "./HorizonSelector.module.css";

export interface HorizonSelectorProps {
  selected: number;
  onChange: (minutes: number) => void;
}

// Segmented pill control for the forecast horizon (doc 02 §3.1). Global
// control; persisted in the URL. role="radiogroup" for a11y (doc 07 §2 S-04).
export function HorizonSelector({ selected, onChange }: HorizonSelectorProps) {
  return (
    <div className={styles.group} role="radiogroup" aria-label="Forecast horizon">
      {HORIZON_OPTIONS.map((opt) => (
        <button
          key={opt.minutes}
          type="button"
          role="radio"
          aria-checked={selected === opt.minutes}
          className={`${styles.pill} ${selected === opt.minutes ? styles.active : ""}`}
          onClick={() => onChange(opt.minutes)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
