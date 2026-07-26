"use client";

import { useEffect, useState } from "react";
import { getSupabaseClient, isAuthConfigured } from "./supabase";

// Session token source for API calls (S-08; ADR-008). When Supabase is
// configured, the access token comes from the SDK session (persisted +
// auto-refreshed). Otherwise (local dev without a Supabase project) a dev
// bearer token — entered once on the sign-in screen and verified against the
// API — is kept in localStorage and presented to the Go devauth verifier
// (token format "<subject>" or "<subject>:<email>"). There is no silent
// env-injected token: every session goes through /auth/signin.

const DEV_TOKEN_KEY = "fiq.dev.token";

/** getDevToken reads the dev-mode bearer token (browser only; SSR-safe null). */
export function getDevToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(DEV_TOKEN_KEY);
  } catch {
    return null;
  }
}

/** setDevToken stores the verified dev-mode bearer token. */
export function setDevToken(token: string): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(DEV_TOKEN_KEY, token);
  } catch {
    /* storage unavailable (private mode) — session lasts the page only */
  }
}

/** clearDevToken removes the dev-mode bearer token. */
export function clearDevToken(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(DEV_TOKEN_KEY);
  } catch {
    /* ignore */
  }
}

/**
 * getAccessToken resolves the bearer token for the current session, or null
 * when signed out. Supabase session token when configured; dev token otherwise.
 */
export async function getAccessToken(): Promise<string | null> {
  const supabase = getSupabaseClient();
  if (supabase) {
    const { data } = await supabase.auth.getSession();
    return data.session?.access_token ?? null;
  }
  return getDevToken();
}

/**
 * authHeaders builds JSON mutation headers with the session bearer token
 * attached (shared by the admin pages' fetch calls).
 */
export async function authHeaders(): Promise<Record<string, string>> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = await getAccessToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

/** signOut ends the session (Supabase SDK sign-out and/or dev token removal). */
export async function signOut(): Promise<void> {
  const supabase = getSupabaseClient();
  if (supabase) {
    await supabase.auth.signOut();
  }
  clearDevToken();
}

/**
 * safeReturnPath sanitizes a post-signin redirect target: only same-origin
 * absolute paths are honoured (no protocol-relative "//" or external URLs),
 * anything else falls back to the Overview.
 */
export function safeReturnPath(raw: string | null | undefined): string {
  if (!raw || !raw.startsWith("/") || raw.startsWith("//")) return "/";
  return raw;
}

// devEmail mirrors the devauth adapter's email derivation so the header can
// label the dev session ("<subject>:<email>" or "<subject>@dev.local").
function devEmail(token: string): string {
  const [subject, email] = token.split(":", 2);
  return email?.trim() || `${subject.trim()}@dev.local`;
}

export type SessionStatus = "loading" | "signed-in" | "signed-out";

export interface SessionState {
  status: SessionStatus;
  /** Best-effort account label; null while loading or signed out. */
  email: string | null;
}

/**
 * useSession exposes the client session state for session-aware chrome (the
 * header account area, admin link gating). Resolution is deferred to an
 * effect so SSR/static HTML stays deterministic (status "loading").
 */
export function useSession(): SessionState {
  const [state, setState] = useState<SessionState>({ status: "loading", email: null });

  useEffect(() => {
    if (isAuthConfigured()) {
      const supabase = getSupabaseClient();
      if (!supabase) return;
      let cancelled = false;
      supabase.auth.getSession().then(({ data }) => {
        if (cancelled) return;
        const email = data.session?.user?.email ?? null;
        setState(data.session ? { status: "signed-in", email } : { status: "signed-out", email: null });
      });
      const { data: sub } = supabase.auth.onAuthStateChange((_event, session) => {
        setState(
          session
            ? { status: "signed-in", email: session.user?.email ?? null }
            : { status: "signed-out", email: null },
        );
      });
      return () => {
        cancelled = true;
        sub.subscription.unsubscribe();
      };
    }
    const token = getDevToken();
    setState(
      token
        ? { status: "signed-in", email: devEmail(token) }
        : { status: "signed-out", email: null },
    );
    return undefined;
  }, []);

  return state;
}
