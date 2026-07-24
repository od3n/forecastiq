"use client";

import styles from "./HorizonSelector.module.css"; // reuse pill styles

const VARIABLES = [
  { value: "temperature", label: "Temperature" },
  { value: "precipitation", label: "Precipitation" },
  { value: "wind_speed", label: "Wind Speed" },
  { value: "humidity", label: "Humidity" },
  { value: "pressure", label: "Pressure" },
] as const;

export interface VariableSelectorProps {
  selected: string;
  onChange: (variable: string) => void;
}

// Variable dropdown (doc 02 §1.5 Selectors). Maps to the `variable` URL param.
export function VariableSelector({ selected, onChange }: VariableSelectorProps) {
  return (
    <select
      value={selected}
      onChange={(e) => onChange(e.target.value)}
      aria-label="Weather variable"
      className={styles.pill}
      style={{ minHeight: 36, borderRadius: "var(--radius-md)", border: "1px solid var(--color-border)", padding: "var(--space-xs) var(--space-sm)", font: "inherit", fontSize: "var(--text-body-sm)" }}
    >
      {VARIABLES.map((v) => (
        <option key={v.value} value={v.value}>{v.label}</option>
      ))}
    </select>
  );
}
