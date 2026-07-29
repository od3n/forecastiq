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
// a11y §3). Unranked shows the actual trigger: with ≥ 10 pairs the only
// backend trigger left is coverage (a value below the floor, or a missing
// value — the backend treats nil coverage as 0; BR-RANK-04), otherwise the
// sample count + threshold.
export function StatusBadge({ status, sampleCount, minSampleCount = 30, coverage }: StatusBadgeProps) {
  const variant =
    status === "ranked" ? "ranked" : status === "provisionally_ranked" ? "provisional" : "unranked";

  // Sample floor takes precedence: < 10 pairs is always sample-driven (§7.2).
  const samplesLow = sampleCount !== undefined && sampleCount < PROVISIONAL_MIN_SAMPLES;
  const coverageTriggered =
    !samplesLow &&
    (coverage != null
      ? coverage < COVERAGE_FLOOR
      : sampleCount !== undefined && sampleCount >= PROVISIONAL_MIN_SAMPLES);

  let label = LABELS[status];
  if (status === "unranked" && coverageTriggered) {
    // Floor, not round: 0.499 must read 49%, never a self-contradictory 50%.
    label =
      coverage != null
        ? `Insufficient coverage (${Math.floor(coverage * 100)}% / ${COVERAGE_FLOOR * 100}% required)`
        : "Insufficient coverage";
  } else if (status === "unranked" && sampleCount !== undefined) {
    label = `Insufficient data (${sampleCount}/${minSampleCount})`;
  } else if (status === "provisionally_ranked" && sampleCount !== undefined) {
    label = `Provisional — ${sampleCount} samples`;
  }

  return <span className={`${styles.badge} ${styles[variant]}`}>{label}</span>;
}
