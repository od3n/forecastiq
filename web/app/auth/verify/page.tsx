import Link from "next/link";
import styles from "../auth.module.css";

// S-08 verify. Informational page shown after sign-up: email verification is
// mandatory and Supabase-managed (ADR-008); the verification link returns the
// user to the app.
export default function VerifyPage() {
  return (
    <section className={styles.card} aria-labelledby="verify-title">
      <h1 id="verify-title" className={styles.title}>
        Check your email
      </h1>
      <div className={styles.notice} role="status">
        We&apos;ve sent a verification link to your email address. Open it to finish
        setting up your account, then sign in.
      </div>
      <div className={styles.links}>
        <Link href="/auth/signin">Back to sign in</Link>
      </div>
    </section>
  );
}
