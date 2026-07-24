// Package supabaseadmin implements the identity module's SupabaseAdmin port
// against the Supabase Auth (GoTrue) Admin API (ADR-008 §6): it bans/unbans and
// deletes users so local account-lifecycle changes propagate to the managed
// auth provider. The service-role key is env-only and never client-exposed. A
// no-op implementation (see noop.go) is selected in dev/test where no managed
// project exists.
package supabaseadmin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config configures the Supabase Admin API client.
type Config struct {
	ProjectURL     string // e.g. https://<ref>.supabase.co
	ServiceRoleKey string // service-role key (env-only; never in a client bundle)
	HTTPClient     *http.Client
}

// Client calls the Supabase Auth Admin API to propagate account-lifecycle
// changes. It implements ports.SupabaseAdmin.
type Client struct {
	base       string
	key        string
	httpClient *http.Client
}

// New builds a Client.
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{base: strings.TrimRight(cfg.ProjectURL, "/"), key: cfg.ServiceRoleKey, httpClient: cfg.HTTPClient}
}

// SetBanned bans (or clears the ban on) a user via the admin update endpoint.
// A long ban_duration approximates a permanent ban; "none" clears it.
func (c *Client) SetBanned(ctx context.Context, authSubject string, banned bool) error {
	dur := "none"
	if banned {
		dur = "876000h" // ~100 years
	}
	return c.do(ctx, http.MethodPut, "/auth/v1/admin/users/"+url.PathEscape(authSubject), fmt.Sprintf(`{"ban_duration":%q}`, dur))
}

// DeleteUser deletes a user via the admin endpoint.
func (c *Client) DeleteUser(ctx context.Context, authSubject string) error {
	return c.do(ctx, http.MethodDelete, "/auth/v1/admin/users/"+url.PathEscape(authSubject), "")
}

func (c *Client) do(ctx context.Context, method, path, body string) error {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("supabase admin %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("supabase admin %s returned %d", path, resp.StatusCode)
	}
	return nil
}
