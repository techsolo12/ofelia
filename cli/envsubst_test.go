// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
)

func TestExpandEnvVars(t *testing.T) {
	// Not parallel — mutates environment variables.

	tests := []struct {
		name   string
		input  string
		env    map[string]string
		want   string
	}{
		{
			name:  "defined var",
			input: "image = ${FOO}",
			env:   map[string]string{"FOO": "bar"},
			want:  "image = bar",
		},
		{
			name:  "undefined stays literal",
			input: "image = ${UNDEF_VAR_XYZ}",
			want:  "image = ${UNDEF_VAR_XYZ}",
		},
		{
			name:  "empty var stays literal",
			input: "image = ${EMPTY_VAR}",
			env:   map[string]string{"EMPTY_VAR": ""},
			want:  "image = ${EMPTY_VAR}",
		},
		{
			name:  "default for undefined",
			input: "image = ${UNDEF_VAR_XYZ:-alpine:latest}",
			want:  "image = alpine:latest",
		},
		{
			name:  "default for empty",
			input: "image = ${EMPTY_VAR:-alpine:latest}",
			env:   map[string]string{"EMPTY_VAR": ""},
			want:  "image = alpine:latest",
		},
		{
			name:  "defined ignores default",
			input: "image = ${FOO:-fallback}",
			env:   map[string]string{"FOO": "bar"},
			want:  "image = bar",
		},
		{
			name:  "explicit empty default",
			input: "pass = ${UNDEF_VAR_XYZ:-}",
			want:  "pass = ",
		},
		{
			name:  "default with special chars",
			input: "url = ${DB:-postgres://user:pass@host:5432/db}",
			want:  "url = postgres://user:pass@host:5432/db",
		},
		{
			name:  "dollar without braces not substituted",
			input: "cmd = echo $FOO",
			env:   map[string]string{"FOO": "bar"},
			want:  "cmd = echo $FOO",
		},
		{
			name:  "dollar-number in command safe",
			input: "command = sh -c 'echo $1'",
			want:  "command = sh -c 'echo $1'",
		},
		{
			name:  "multiple vars one line",
			input: "env = ${HOST}:${PORT}",
			env:   map[string]string{"HOST": "localhost", "PORT": "8080"},
			want:  "env = localhost:8080",
		},
		{
			name:  "var in section header",
			input: `[job-exec "${JOB_NAME}"]`,
			env:   map[string]string{"JOB_NAME": "backup"},
			want:  `[job-exec "backup"]`,
		},
		{
			name:  "no vars passthrough",
			input: "schedule = @daily",
			want:  "schedule = @daily",
		},
		{
			name:  "invalid var name not matched",
			input: "x = ${123BAD}",
			want:  "x = ${123BAD}",
		},
		{
			name:  "no nesting",
			input: "x = ${FOO${BAR}}",
			env:   map[string]string{"FOO": "a", "BAR": "b"},
			want:  "x = ${FOOb}",
		},
		{
			name:  "backslash before var still substitutes",
			input: `x = \${FOO}`,
			env:   map[string]string{"FOO": "bar"},
			want:  `x = \bar`,
		},
		{
			name: "multiline config",
			input: `[global]
smtp-host = ${SMTP_HOST:-mail.example.com}
smtp-password = ${SMTP_PASS}

[job-exec "test"]
schedule = @every 5s
container = ${CONTAINER}
command = echo $HOME`,
			env: map[string]string{
				"SMTP_PASS": "s3cret",
				"CONTAINER": "my-app",
			},
			want: `[global]
smtp-host = mail.example.com
smtp-password = s3cret

[job-exec "test"]
schedule = @every 5s
container = my-app
command = echo $HOME`,
		},
		{
			name:  "cron expression untouched",
			input: "schedule = */5 * * * *",
			want:  "schedule = */5 * * * *",
		},
		{
			name:  "password with special chars in value",
			input: "smtp-password = ${PASS}",
			env:   map[string]string{"PASS": "p@ss=w0rd!#$%"},
			want:  "smtp-password = p@ss=w0rd!#$%",
		},
		{
			name:  "default with colon in image tag",
			input: "image = ${IMG:-nginx:1.25-alpine}",
			want:  "image = nginx:1.25-alpine",
		},
		{
			name:  "underscore in var name",
			input: "x = ${MY_LONG_VAR_NAME}",
			env:   map[string]string{"MY_LONG_VAR_NAME": "val"},
			want:  "x = val",
		},
		{
			name:  "var at start of value",
			input: "${PREFIX}/path",
			env:   map[string]string{"PREFIX": "/opt"},
			want:  "/opt/path",
		},
		{
			name:  "default with closing brace in middle",
			input: "x = ${UNDEF_VAR_XYZ:-hello}world",
			want:  "x = helloworld",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set env vars for this test
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			// Unset any vars that should be undefined
			// (t.Setenv handles cleanup automatically)

			got := expandEnvVars(tc.input)
			if got != tc.want {
				t.Errorf("expandEnvVars(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}
