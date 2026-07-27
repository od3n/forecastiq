"use client";

import { Suspense, useState, type FormEvent } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { getSupabaseClient, isAuthConfigured } from "@/lib/auth/supabase";
import { signInErrorMessage } from "@/lib/auth/messages";
import { safeReturnPath, setDevToken } from "@/lib/auth/session";
import { apiGet } from "@/lib/api/client";
import styles from "../auth.module.css";

// S-08 sign in. With Supabase configured: custom form calling the Supabase JS
// SDK (C-17); no app-managed password handling; on success the SDK stores the
// session. Without Supabase (local dev): a dev-token form verified against the
// API (GET /me through the Go devauth verifier) before the token is stored, so
// the admin surface is only reachable through a real sign-in in every
// environment. Failures show generic messages (no account enumeration, SEC-09).
// `?return=` redirects are sanitized to same-origin paths.

function SupabaseSignInForm({ returnPath }: { returnPath: string }) {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const supabase = getSupabaseClient();
    if (!supabase) return;
    setSubmitting(true);
    const { error: err } = await supabase.auth.signInWithPassword({ email, password });
    setSubmitting(false);
    if (err) {
      setError(signInErrorMessage(err.status));
      return;
    }
    router.push(returnPath);
  }

  return (
    <>
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
    </>
  );
}

// Dev-mode sign in (Supabase not configured). The token is only stored after
// the API accepts it, so a typo cannot leave a broken "session" behind. The
// redirect is a full navigation so all SWR caches restart authenticated.
function DevSignInForm({ returnPath }: { returnPath: string }) {
  const [token, setToken] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const trimmed = token.trim();
    if (!trimmed) {
      setError("Enter a dev token.");
      return;
    }
    setSubmitting(true);
    try {
      await apiGet("/me", { token: trimmed });
      setDevToken(trimmed);
      window.location.assign(returnPath);
    } catch {
      setSubmitting(false);
      setError("Sign-in failed. Check the dev token and that the API is running.");
    }
  }

  return (
    <>
      <p className={styles.notice}>
        Development mode — Supabase is not configured. Sign in with a dev token
        (format <code>subject</code> or <code>subject:email</code>).
      </p>
      {error && (
        <div className={styles.errorSummary} role="alert">
          {error}
        </div>
      )}
      <form className={styles.form} onSubmit={onSubmit} noValidate>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="dev-token">
            Dev token
          </label>
          <input
            id="dev-token"
            className={styles.input}
            type="text"
            autoComplete="off"
            spellCheck={false}
            required
            value={token}
            onChange={(e) => setToken(e.target.value)}
          />
        </div>
        <button className={styles.button} type="submit" disabled={submitting}>
          {submitting ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </>
  );
}

// Inner content (needs a Suspense boundary for useSearchParams under static export).
function SignInContent() {
  const searchParams = useSearchParams();
  const returnPath = safeReturnPath(searchParams.get("return"));

  return (
    <section className={styles.card} aria-labelledby="signin-title">
      <h1 id="signin-title" className={styles.title}>
        Sign in
      </h1>
      {isAuthConfigured() ? (
        <SupabaseSignInForm returnPath={returnPath} />
      ) : (
        <DevSignInForm returnPath={returnPath} />
      )}
    </section>
  );
}

export default function SignInPage() {
  return (
    <Suspense fallback={null}>
      <SignInContent />
    </Suspense>
  );
}
