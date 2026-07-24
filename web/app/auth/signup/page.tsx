"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { getSupabaseClient } from "@/lib/auth/supabase";
import { signUpErrorMessage, AUTH_NOT_CONFIGURED } from "@/lib/auth/messages";
import styles from "../auth.module.css";

// S-08 sign up. Registration via the Supabase SDK (email verification is
// mandatory and Supabase-managed, ADR-008). On success we route to the verify
// page. Min length 12 is enforced client-side as a hint; Supabase is authoritative.
export default function SignUpPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (password.length < 12) {
      setError("Password must be at least 12 characters.");
      return;
    }
    const supabase = getSupabaseClient();
    if (!supabase) {
      setError(AUTH_NOT_CONFIGURED);
      return;
    }
    setSubmitting(true);
    const { error: err } = await supabase.auth.signUp({ email, password });
    setSubmitting(false);
    if (err) {
      setError(signUpErrorMessage(err.status));
      return;
    }
    router.push("/auth/verify");
  }

  return (
    <section className={styles.card} aria-labelledby="signup-title">
      <h1 id="signup-title" className={styles.title}>
        Create account
      </h1>
      {error && (
        <div className={styles.errorSummary} role="alert">
          {error}
        </div>
      )}
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
        <div className={styles.field}>
          <label className={styles.label} htmlFor="password">
            Password (min 12 characters)
          </label>
          <input
            id="password"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            minLength={12}
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <button className={styles.button} type="submit" disabled={submitting}>
          {submitting ? "Creating…" : "Create account"}
        </button>
      </form>
      <div className={styles.links}>
        <Link href="/auth/signin">Already have an account? Sign in</Link>
      </div>
    </section>
  );
}
