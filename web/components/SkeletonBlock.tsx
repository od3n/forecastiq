import styles from "./SkeletonBlock.module.css";

export type SkeletonVariant = "card" | "row" | "chart";

export interface SkeletonBlockProps {
  variant: SkeletonVariant;
  count?: number;
}

// Skeleton loading block (doc 02 §13.2). Matches the final layout shape
// (CLS < 0.1); pulse animation disabled under prefers-reduced-motion.
export function SkeletonBlock({ variant, count = 1 }: SkeletonBlockProps) {
  return (
    <div aria-busy="true" aria-label="Loading data" style={{ display: "flex", flexDirection: "column", gap: "var(--space-sm)" }}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className={`${styles.skeleton} ${styles[variant]}`} />
      ))}
    </div>
  );
}
