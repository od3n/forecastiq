package supabaseadmin

import "context"

// Noop is the dev/test SupabaseAdmin: account-lifecycle changes are local-only
// (there is no managed provider to call). It is selected when running in auth
// dev-mode or when no service-role key is configured. It implements
// ports.SupabaseAdmin and always succeeds.
type Noop struct{}

// NewNoop returns a no-op admin propagator.
func NewNoop() Noop { return Noop{} }

// SetBanned is a no-op.
func (Noop) SetBanned(context.Context, string, bool) error { return nil }

// DeleteUser is a no-op.
func (Noop) DeleteUser(context.Context, string) error { return nil }
