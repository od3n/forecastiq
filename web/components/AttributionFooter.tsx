import styles from "./AttributionFooter.module.css";

/** One provider's attribution, configured server-side (never hardcoded; BR-ATTR-01). */
export interface ProviderAttribution {
  provider: string;
  text: string;
  url: string;
}

export interface AttributionFooterProps {
  /** Provider attributions from the response envelope (`attribution[]`). */
  providers?: ProviderAttribution[];
  /** Observation provenance summary line (e.g. "Open-Meteo Historical (reanalysis blend)"). */
  observations?: string;
  /** Methodology version from `metadata.methodology_version`. */
  methodologyVersion?: string;
}

// Attribution footer on every data-bearing page (doc 02 §3.4; BR-ATTR-01).
// Attribution text/url are configured per provider (from the API envelope),
// never hardcoded; the non-promise disclaimer (NP-01) is always shown. The
// foundation renders the static disclaimer; screens pass envelope attribution.
export function AttributionFooter({
  providers,
  observations,
  methodologyVersion,
}: AttributionFooterProps) {
  return (
    <footer className={styles.footer}>
      <div className={styles.inner}>
        {providers && providers.length > 0 && (
          <p className={styles.sources}>
            Data sources:{" "}
            {providers.map((p, i) => (
              <span key={p.provider}>
                {i > 0 && " · "}
                {p.provider} ({p.text}){" "}
                <a className={styles.link} href={p.url} target="_blank" rel="noreferrer noopener">
                  link
                </a>
              </span>
            ))}
          </p>
        )}
        {(observations || methodologyVersion) && (
          <p>
            {observations && <>Observations: {observations}</>}
            {observations && methodologyVersion && " · "}
            {methodologyVersion && <>Methodology {methodologyVersion}</>}
          </p>
        )}
        <p>All times UTC unless labeled.</p>
        <p>ForecastIQ measures forecast accuracy. We don&apos;t deliver weather forecasts.</p>
      </div>
    </footer>
  );
}
