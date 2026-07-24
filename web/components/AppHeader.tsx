import Link from "next/link";
import styles from "./AppHeader.module.css";

// Global header shell (doc 02 §3.1). Role-gated items (Admin, Settings) and the
// global location/horizon selectors are wired with auth + screens in WP-21;
// the foundation ships the public nav + account entry point.
export function AppHeader() {
  return (
    <header className={styles.header}>
      <div className={styles.inner}>
        <Link href="/" className={styles.logo}>
          ForecastIQ
        </Link>
        <nav className={styles.nav} aria-label="Primary">
          <Link href="/" className={styles.navLink}>
            Overview
          </Link>
          <Link href="/trends" className={styles.navLink}>
            Trends
          </Link>
          <Link href="/methodology" className={styles.navLink}>
            Methodology
          </Link>
        </nav>
        <span className={styles.spacer} />
        <Link href="/auth/signin" className={styles.account}>
          Sign in
        </Link>
      </div>
    </header>
  );
}
