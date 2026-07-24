import styles from "./StatusBadge.module.css";

export type RankingStatus = "ranked" | "provisionally_ranked" | "unranked";

export interface StatusBadgeProps {
  status: RankingStatus;
  sampleCount?: number;
  minSampleCount?: number;
}

const LABELS: Record<RankingStatus, string> = {
  ranked: "Ranked",
  provisionally_ranked: "Provisional",
  unranked: "Insufficient data",
};

// Ranking status badge (doc 02 §1.5). Text is always present (not color-only;
// a11y §3). Unranked shows the sample count + threshold.
export function StatusBadge({ status, sampleCount, minSampleCount = 30 }: StatusBadgeProps) {
  const variant =
    status === "ranked" ? "ranked" : status === "provisionally_ranked" ? "provisional" : "unranked";

  let label = LABELS[status];
  if (status === "unranked" && sampleCount !== undefined) {
    label = `Insufficient data (${sampleCount}/${minSampleCount})`;
  } else if (status === "provisionally_ranked" && sampleCount !== undefined) {
    label = `Provisional — ${sampleCount} samples`;
  }

  return <span className={`${styles.badge} ${styles[variant]}`}>{label}</span>;
}
