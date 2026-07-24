"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { getSupabaseClient } from "@/lib/auth/supabase";
import { RESET_SENT_MESSAGE, AUTH_NOT_CONFIGURED } from "@/lib/auth/messages";
import styles from "../auth.module.css";

// S-08 password reset. Calls the Supabase SDK, then shows a confirmation
// REGARDLESS of whether the email exists (no account enumeration, SEC-09).
// A 429 surfaces the rate-limit message instead.
export default function ResetPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const supabase = getSupabaseClient();
    if (!supabase) {
      setError(AUTH_NOT_CONFIGURED);
      return;
    }
    setSubmitting(true);
    const { error: err } = await supabase.auth.resetPasswordForEmail(email);
    setSubmitting(false);
    if (err?.status === 429) {
      setError("Too many attempts. Please try again later.");
      return;
    }
    // Any other outcome (success or unknown email) → identical confirmation.
    setSent(true);
  }

  return (
    <section className={styles.card} aria-labelledby="reset-title">
      <h1 id="reset-title" className={styles.title}>
        Reset password
      </h1>
      {error && (
        <div className={styles.errorSummary} role="alert">
          {error}
        </div>
      )}
      {sent ? (
        <div className={styles.notice} role="status">
          {RESET_SENT_MESSAGE}
        </div>
      ) : (
        <form className={styles.form} onSubmit={onSubmit} noValidate>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="email">
              Email
            </label>
            <input
              id="email"
              className={styles.input}
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <button className={styles.button} type="submit" disabled={submitting}>
            {submitting ? "Sending…" : "Send reset link"}
          </button>
        </form>
      )}
      <div className={styles.links}>
        <Link href="/auth/signin">Back to sign in</Link>
      </div>
    </section>
  );
}
