package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/identity"
)

// webhookMaxBody bounds the webhook request body read for signature + parse.
const webhookMaxBody = 1 << 20 // 1 MB

// webhookPayload is a permissive view of a Supabase auth webhook body. The
// event type and subject are read from the fields Supabase variants use; the
// exact provider mapping is adaptable without touching the security envelope.
type webhookPayload struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Subject string `json:"subject"`
	UserID  string `json:"user_id"`
	IP      string `json:"ip_address"`
	User    struct {
		ID string `json:"id"`
	} `json:"user"`
	Record struct {
		ID string `json:"id"`
	} `json:"record"`
}

// AuthWebhook receives signed Supabase auth webhooks and records `auth.*` audit
// events (audit-requirements §5). Signature verification is mandatory (HMAC-
// SHA256 over the raw body, timing-safe compare); ingestion is non-blocking — a
// valid signature always yields 204 even if the audit write fails, so the auth
// flow never depends on our audit availability. An invalid/missing signature is
// 401. The route is mounted only when a webhook secret is configured.
func (h *Handlers) AuthWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, webhookMaxBody))
	if err != nil {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	if !verifyWebhookSignature(h.WebhookSecret, c.GetHeader("X-Webhook-Signature"), body) {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	var p webhookPayload
	if jerr := json.Unmarshal(body, &p); jerr != nil {
		// Signature valid but body unparseable: acknowledge (non-blocking) + log.
		h.Logger.WarnContext(c.Request.Context(), "webhook.unparseable_body")
		c.Status(http.StatusNoContent)
		return
	}
	ev := identity.WebhookEvent{
		Type:    firstNonEmpty(p.Type, p.Event),
		Subject: firstNonEmpty(p.Subject, p.UserID, p.User.ID, p.Record.ID),
		IP:      firstNonEmpty(p.IP, c.ClientIP()),
	}
	if ierr := h.Webhook.Ingest(c.Request.Context(), ev); ierr != nil {
		h.Logger.ErrorContext(c.Request.Context(), "webhook.ingest_failed", slog.String("error", ierr.Error()))
	}
	c.Status(http.StatusNoContent)
}

// verifyWebhookSignature checks an HMAC-SHA256 signature over the raw body in
// constant time. The header carries "sha256=<hex>" (a bare hex value is also
// accepted). An empty secret or header fails closed.
func verifyWebhookSignature(secret, header string, body []byte) bool {
	if secret == "" || header == "" {
		return false
	}
	got := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(want))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
