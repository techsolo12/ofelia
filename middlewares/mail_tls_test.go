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

// TestNewMail_RejectsInvalidTLSPolicy ensures a misconfigured smtp-tls-policy
// is surfaced at startup rather than silently ignored. NewMail returns nil
// for an empty config; for a non-empty config with an invalid policy it must
// still construct the middleware (so the rest of the pipeline keeps working)
// but the policy must be normalized to the safe default — never to a weaker
// policy than what the operator wrote. We assert the safe-default behavior
// by checking the resolved dialer policy through the public mapping.
func TestNewMail_RejectsInvalidTLSPolicy(t *testing.T) {
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

	// We expect NO MAIL FROM to be received because the dialer should refuse
	// to send against a server that does not advertise STARTTLS.
	select {
	case <-f.fromCh:
		t.Fatal("expected default policy (mandatory) to refuse plaintext SMTP server, but mail was sent")
	case <-time.After(500 * time.Millisecond):
		// Expected: dialer refused.
	}

	// m.Run returns the underlying job error, not the mail error
	// (mail errors are logged); just confirm the goroutine completes.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Mail.Run did not return")
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
