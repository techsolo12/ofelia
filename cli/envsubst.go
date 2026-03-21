// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"regexp"
	"strings"
)

// envVarPattern matches ${VAR} and ${VAR:-default} syntax.
// Only matches valid variable names: letter or underscore, then alphanumeric/underscore.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-[^}]*)?\}`)

// ExpandEnvVars substitutes ${VAR} and ${VAR:-default} in the input string
// with environment variable values.
//
// Behavior:
//   - ${VAR}: replaced with env value if defined and non-empty; kept literal otherwise
//   - ${VAR:-default}: replaced with env value if defined and non-empty; uses default otherwise
//   - $VAR (without braces): not substituted — safe for cron expressions and shell commands
func ExpandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		groups := envVarPattern.FindStringSubmatch(match)
		varName := groups[1]
		defaultPart := groups[2] // "" if no :- present, ":-..." if present

		value, exists := os.LookupEnv(varName)
		if exists && value != "" {
			return value
		}

		// Check if a default was specified (:-default syntax)
		if strings.HasPrefix(defaultPart, ":-") {
			return defaultPart[2:] // strip the :- prefix
		}

		// Undefined without default — keep literal
		return match
	})
}
