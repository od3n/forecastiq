"use client";

import styles from "./HorizonSelector.module.css"; // reuse segmented pill styles

const AGGREGATIONS = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
] as const;

export interface AggregationSelectorProps {
  selected: string;
  onChange: (aggregation: string) => void;
}

// Aggregation selector — segmented control (doc 02 §14.2). role="radiogroup".
export function AggregationSelector({ selected, onChange }: AggregationSelectorProps) {
  return (
    <div className={styles.group} role="radiogroup" aria-label="Time aggregation">
      {AGGREGATIONS.map((opt) => (
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
