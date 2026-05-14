// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSMTPTLSPolicy_Valid pins the accepted values for the smtp-tls-policy
// INI key. Defensive enum handling per CLAUDE.md: only the documented set is
// valid, the empty string is valid (means "use default = mandatory"), and
// unknown values are rejected so a typo cannot silently weaken transport
// security. See https://github.com/netresearch/ofelia/issues/653.
func TestSMTPTLSPolicy_Valid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value SMTPTLSPolicy
		want  bool
	}{
		{name: "empty (default)", value: SMTPTLSPolicy(""), want: true},
		{name: "mandatory", value: SMTPTLSPolicyMandatory, want: true},
		{name: "opportunistic", value: SMTPTLSPolicyOpportunistic, want: true},
		{name: "none", value: SMTPTLSPolicyNone, want: true},
		{name: "typo: required", value: SMTPTLSPolicy("required"), want: false},
		{name: "case wrong: Mandatory", value: SMTPTLSPolicy("Mandatory"), want: false},
		{name: "garbage", value: SMTPTLSPolicy("xyz"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.value.Valid())
		})
	}
}

// TestResolveSMTPTLSPolicy_UnknownFallsBackToMandatory ensures a misconfigured
// smtp-tls-policy normalizes to the safe default rather than silently weakening
// transport security. The resolver is the choke point that NewMail / sendMail
// flow through; we exercise it directly because the fail-closed contract is
// the resolver's, not NewMail's. (The test name was tightened per #660 review:
// the previous name implied NewMail itself rejected the policy, but NewMail
// happily constructs a middleware with any string — the fail-closed step is
// here.)
func TestResolveSMTPTLSPolicy_UnknownFallsBackToMandatory(t *testing.T) {
	t.Parallel()

	// Unknown policy must NOT silently degrade to NoStartTLS or Opportunistic.
	// The policy resolver maps unknown -> mandatory (the safe default).
	policy := resolveSMTPTLSPolicy(SMTPTLSPolicy("required-pls"))
	assert.Equal(t, SMTPTLSPolicyMandatory, policy,
		"unknown policy must map to the safe default (mandatory), never weaker")
}

// TestMail_DefaultPolicyIsMandatory pins the default behavior change from
// go-mail's OpportunisticStartTLS (silently sends cleartext if STARTTLS is
// not advertised) to MandatoryStartTLS — the upstream's own recommendation
// for any modern SMTP server. See https://github.com/netresearch/ofelia/issues/653.
//
// We verify the default by hitting a plaintext SMTP server (the existing
// test fixture does not advertise STARTTLS); under MandatoryStartTLS the
// send must fail with an error that mentions TLS, NOT silently succeed.
func TestMail_DefaultPolicyIsMandatory(t *testing.T) {
	t.Parallel()
	f := setupMailTest(t)

	f.ctx.Start()
	f.ctx.Stop(nil)

	// No SMTPTLSPolicy set → should resolve to mandatory.
	m := NewMail(&MailConfig{
		SMTPHost:  f.smtpdHost,
		SMTPPort:  f.smtpdPort,
		EmailTo:   "foo@foo.com",
		EmailFrom: "qux@qux.com",
	})
	require.NotNil(t, m)

	done := make(chan error, 1)
	go func() {
		done <- m.Run(f.ctx)
	}()

	// Wait for Run to complete first (it's the deterministic signal — the
	// dialer either errored on the missing STARTTLS or completed). Then
	// inspect fromCh non-blockingly to confirm no MAIL FROM was ever
	// received. Previously this test gated on a 500ms timeout to assert
	// "no mail was sent", which is flaky on slow CI runners (Copilot
	// review of #660). The new shape: the dialer's actual completion is
	// the upper bound — by the time Run returns, any MAIL FROM that was
	// going to arrive has arrived.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Mail.Run did not return within 10s")
	}

	select {
	case <-f.fromCh:
		t.Fatal("expected default policy (mandatory) to refuse plaintext SMTP server, but mail was sent")
	default:
		// Expected: dialer refused before reaching MAIL FROM.
	}
}

// TestMail_PolicyNoneAllowsPlaintext is the opt-in counterpart: setting
// smtp-tls-policy = none restores the legacy ability to send to plain SMTP
// servers (local relays, MailHog, dev fixtures). This is the migration
// escape hatch for operators who can't enable STARTTLS upstream.
func TestMail_PolicyNoneAllowsPlaintext(t *testing.T) {
	t.Parallel()
	f := setupMailTest(t)

	f.ctx.Start()
	f.ctx.Stop(nil)

	m := NewMail(&MailConfig{
		SMTPHost:      f.smtpdHost,
		SMTPPort:      f.smtpdPort,
		EmailTo:       "foo@foo.com",
		EmailFrom:     "qux@qux.com",
		SMTPTLSPolicy: SMTPTLSPolicyNone,
	})
	require.NotNil(t, m)

	done := make(chan struct{})
	var runErr error
	go func() {
		runErr = m.Run(f.ctx)
		close(done)
	}()

	select {
	case from := <-f.fromCh:
		assert.Equal(t, "qux@qux.com", from)
	case <-time.After(3 * time.Second):
		t.Fatal("expected SMTP send with policy=none against plain server")
	}

	<-done
	require.NoError(t, runErr)
}

// TestSMTPTLSPolicy_Validate covers the hard-fail validation surface
// (Validate returns ErrInvalidSMTPTLSPolicy for unknown values; nil for the
// documented set). Production config-load paths can branch on this to
// reject typos at startup rather than soft-falling-back via
// resolveSMTPTLSPolicy at runtime.
func TestSMTPTLSPolicy_Validate(t *testing.T) {
	t.Parallel()
	require.NoError(t, SMTPTLSPolicy("").Validate())
	require.NoError(t, SMTPTLSPolicyMandatory.Validate())
	require.NoError(t, SMTPTLSPolicyOpportunistic.Validate())
	require.NoError(t, SMTPTLSPolicyNone.Validate())

	err := SMTPTLSPolicy("required").Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidSMTPTLSPolicy)
	assert.Contains(t, err.Error(), `"required"`,
		"validation error should include the offending value for diagnostics")
}
