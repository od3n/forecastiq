// Safe, non-enumerating auth error copy (S-08 behavioural states; SEC-09).
// Supabase AuthError carries an HTTP status; we map to generic messages that
// never reveal whether an account exists.
export function signInErrorMessage(status: number | undefined): string {
  if (status === 429) return "Too many attempts. Please try again later.";
  return "Invalid email or password.";
}

export function signUpErrorMessage(status: number | undefined): string {
  if (status === 429) return "Too many attempts. Please try again later.";
  return "Unable to create the account. Please check your details and try again.";
}

/** The reset flow always reports success regardless of account existence. */
export const RESET_SENT_MESSAGE =
  "If an account exists for that email, a password reset link has been sent.";

export const AUTH_NOT_CONFIGURED =
  "Sign-in is not configured in this environment. Set NEXT_PUBLIC_SUPABASE_URL and NEXT_PUBLIC_SUPABASE_ANON_KEY.";
