"use client";

import styles from "./HorizonSelector.module.css"; // reuse pill styles

const PRESETS = [
  { value: "7d", label: "7 days" },
  { value: "30d", label: "30 days" },
  { value: "90d", label: "90 days" },
] as const;

export interface DateRangePickerProps {
  selected: string;
  onChange: (period: string) => void;
}

// Date range preset selector (doc 02 §1.5). Segmented preset pills (7d/30d/90d).
// Custom date range input is a follow-on.
export function DateRangePicker({ selected, onChange }: DateRangePickerProps) {
  return (
    <div className={styles.group} role="radiogroup" aria-label="Date range">
      {PRESETS.map((opt) => (
        <button
          key={opt.value}
          type="button"
          role="radio"
          aria-checked={selected === opt.value}
          className={`${styles.pill} ${selected === opt.value ? styles.active : ""}`}
          onClick={() => onChange(opt.value)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
