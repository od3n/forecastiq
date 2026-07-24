"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { getSupabaseClient } from "@/lib/auth/supabase";
import { signInErrorMessage, AUTH_NOT_CONFIGURED } from "@/lib/auth/messages";
import styles from "../auth.module.css";

// S-08 sign in. Custom form calling the Supabase JS SDK (C-17); no app-managed
// password handling. On success the SDK stores the session; we redirect to the
// Overview. Failures show a generic message (no account enumeration, SEC-09).
export default function SignInPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
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
    const { error: err } = await supabase.auth.signInWithPassword({ email, password });
    setSubmitting(false);
    if (err) {
      setError(signInErrorMessage(err.status));
      return;
    }
    router.push("/");
  }

  return (
    <section className={styles.card} aria-labelledby="signin-title">
      <h1 id="signin-title" className={styles.title}>
        Sign in
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
            Password
          </label>
          <input
            id="password"
            className={styles.input}
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <button className={styles.button} type="submit" disabled={submitting}>
          {submitting ? "Signing in…" : "Sign in"}
        </button>
      </form>
      <div className={styles.links}>
        <Link href="/auth/reset">Forgot password?</Link>
        <Link href="/auth/signup">Create account</Link>
      </div>
    </section>
  );
}
