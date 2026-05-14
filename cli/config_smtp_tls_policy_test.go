// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/middlewares"
	"github.com/netresearch/ofelia/test"
)

// TestSMTPTLSPolicy_ParsedFromINI verifies the gcfg parser correctly
// hydrates the new `smtp-tls-policy` INI key onto MailConfig.SMTPTLSPolicy
// (a typed string alias). Without this test, a future change to gcfg or
// the field tag could silently leave the value as the empty string and
// the runtime would soft-fall back to mandatory — the operator would
// still get safe behavior, but their explicit configuration would be
// silently ignored. See https://github.com/netresearch/ofelia/issues/653.
func TestSMTPTLSPolicy_ParsedFromINI(t *testing.T) {
	cases := []struct {
		name string
		ini  string
		want middlewares.SMTPTLSPolicy
	}{
		{
			name: "mandatory",
			want: middlewares.SMTPTLSPolicyMandatory,
			ini: `
[global]
smtp-host = mail.example.com
smtp-port = 587
smtp-tls-policy = mandatory

[job-exec "t"]
schedule = @daily
container = app
command = echo ok
`,
		},
		{
			name: "opportunistic",
			want: middlewares.SMTPTLSPolicyOpportunistic,
			ini: `
[global]
smtp-host = mail.example.com
smtp-port = 587
smtp-tls-policy = opportunistic

[job-exec "t"]
schedule = @daily
container = app
command = echo ok
`,
		},
		{
			name: "none",
			want: middlewares.SMTPTLSPolicyNone,
			ini: `
[global]
smtp-host = mail.example.com
smtp-port = 587
smtp-tls-policy = none

[job-exec "t"]
schedule = @daily
container = app
command = echo ok
`,
		},
		{
			name: "absent (default empty, runtime treats as mandatory)",
			want: middlewares.SMTPTLSPolicy(""),
			ini: `
[global]
smtp-host = mail.example.com
smtp-port = 587

[job-exec "t"]
schedule = @daily
container = app
command = echo ok
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := BuildFromString(tc.ini, test.NewTestLogger())
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Global.SMTPTLSPolicy,
				"smtp-tls-policy must hydrate into Global.SMTPTLSPolicy")
		})
	}
}

// TestSMTPTLSPolicy_JobInheritsFromGlobal pins the inheritance behavior
// added in cli/config.go: a job that omits smtp-tls-policy inherits the
// global value; a job that sets it (including to "none") keeps its own.
// Tracks the same operator-intent rule already documented for
// SMTPTLSSkipVerify: explicit per-job security settings win.
func TestSMTPTLSPolicy_JobInheritsFromGlobal(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.Global.MailConfig.SMTPTLSPolicy = middlewares.SMTPTLSPolicyOpportunistic
	job := &middlewares.MailConfig{}

	cfg.mergeMailDefaults(job)

	assert.Equal(t, middlewares.SMTPTLSPolicyOpportunistic, job.SMTPTLSPolicy,
		"job with empty SMTPTLSPolicy should inherit from global")
}

// TestSMTPTLSPolicy_JobOverridesGlobal pins the negative case: a job that
// has set its own policy must NOT be overwritten by global.
func TestSMTPTLSPolicy_JobOverridesGlobal(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.Global.MailConfig.SMTPTLSPolicy = middlewares.SMTPTLSPolicyMandatory
	job := &middlewares.MailConfig{SMTPTLSPolicy: middlewares.SMTPTLSPolicyNone}

	cfg.mergeMailDefaults(job)

	assert.Equal(t, middlewares.SMTPTLSPolicyNone, job.SMTPTLSPolicy,
		"job with explicit SMTPTLSPolicy must not be overwritten by global")
}
