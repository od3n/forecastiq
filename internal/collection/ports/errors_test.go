package ports_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

func TestProviderError_OutcomeMapping(t *testing.T) {
	cases := []struct {
		code ports.ErrorCode
		want ports.Outcome
	}{
		{ports.ErrTimeout, ports.OutcomeTimeout},
		{ports.ErrRateLimited, ports.OutcomeRateLimited},
		{ports.ErrInvalidCredentials, ports.OutcomeAuthFailed},
		{ports.ErrProvider5xx, ports.OutcomeFailed},
		{ports.ErrNetworkLocal, ports.OutcomeFailed},
		{ports.ErrSchemaDrift, ports.OutcomeFailed},
	}
	for _, tc := range cases {
		pe := ports.NewProviderError(tc.code, 0, false, nil)
		assert.Equalf(t, tc.want, pe.Outcome(), "code %s", tc.code)
	}
}

func TestProviderError_ErrorIsLogSafe(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.1:443: secret detail")
	pe := ports.NewProviderError(ports.ErrProvider5xx, 503, true, cause)
	msg := pe.Error()
	assert.Contains(t, msg, "provider_5xx")
	assert.Contains(t, msg, "503")
	assert.NotContains(t, msg, "secret detail", "wrapped cause must not render in the message")
	// Unwrap still exposes the cause for errors.Is/As.
	assert.ErrorIs(t, pe, cause)
}

func TestProviderError_NoStatus(t *testing.T) {
	pe := ports.NewProviderError(ports.ErrTimeout, 0, true, nil)
	assert.Equal(t, "provider error: timeout", pe.Error())
}
