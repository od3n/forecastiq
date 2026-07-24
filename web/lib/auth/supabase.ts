import { createClient, type SupabaseClient } from "@supabase/supabase-js";

// Browser Supabase client (S-08; ADR-008). The URL + anon key are browser-safe
// public config (secrets-management doc: anon key is RLS-scoped). Configured via
// NEXT_PUBLIC_SUPABASE_URL / NEXT_PUBLIC_SUPABASE_ANON_KEY. When unconfigured
// (local dev without a Supabase project), the client is null and the auth
// screens render a "not configured" notice instead of crashing.

let cached: SupabaseClient | null | undefined;

export function isAuthConfigured(): boolean {
  return Boolean(
    process.env.NEXT_PUBLIC_SUPABASE_URL && process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
  );
}

export function getSupabaseClient(): SupabaseClient | null {
  if (cached !== undefined) return cached;
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const anonKey = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
  if (!url || !anonKey) {
    cached = null;
    return null;
  }
  cached = createClient(url, anonKey, {
    auth: {
      persistSession: true,
      autoRefreshToken: true,
      // Handle the token in the URL after email verification / password recovery.
      detectSessionInUrl: true,
      flowType: "pkce",
    },
  });
  return cached;
}
