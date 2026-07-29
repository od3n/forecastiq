import styles from "./StatusBadge.module.css";

export type RankingStatus = "ranked" | "provisionally_ranked" | "unranked";

export interface StatusBadgeProps {
  status: RankingStatus;
  sampleCount?: number;
  minSampleCount?: number;
  coverage?: number | null;
}

// Coverage floor below which a provider is unranked (methodology §7.2,
// BR-RANK-04); mirrors coverageFloor in internal/analysis/domain/ranking.go.
const COVERAGE_FLOOR = 0.5;
// Per-variable pair minimum below which unranked is sample-driven (§7.2);
// mirrors provisionalMinPairs in the backend.
const PROVISIONAL_MIN_SAMPLES = 10;

const LABELS: Record<RankingStatus, string> = {
  ranked: "Ranked",
  provisionally_ranked: "Provisional",
  unranked: "Insufficient data",
};

// Ranking status badge (doc 02 §1.5). Text is always present (not color-only;
// a11y §3). Unranked shows the actual trigger: low coverage (BR-RANK-04) when
// the sample count is not the problem, otherwise the sample count + threshold.
export function StatusBadge({ status, sampleCount, minSampleCount = 30, coverage }: StatusBadgeProps) {
  const variant =
    status === "ranked" ? "ranked" : status === "provisionally_ranked" ? "provisional" : "unranked";

  const coverageTriggered =
    coverage != null &&
    coverage < COVERAGE_FLOOR &&
    (sampleCount === undefined || sampleCount >= PROVISIONAL_MIN_SAMPLES);

  let label = LABELS[status];
  if (status === "unranked" && coverageTriggered) {
    label = `Insufficient coverage (${Math.round(coverage * 100)}%)`;
  } else if (status === "unranked" && sampleCount !== undefined) {
    label = `Insufficient data (${sampleCount}/${minSampleCount})`;
  } else if (status === "provisionally_ranked" && sampleCount !== undefined) {
    label = `Provisional — ${sampleCount} samples`;
  }

  return <span className={`${styles.badge} ${styles[variant]}`}>{label}</span>;
}
