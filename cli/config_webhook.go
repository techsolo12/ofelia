// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	ini "gopkg.in/ini.v1"

	"github.com/netresearch/ofelia/middlewares"
)

const webhookSection = "webhook"

// Canonical webhook-* global config key names — single source of truth shared
// by the INI side (mapstructure tags on middlewares.WebhookGlobalConfig), the
// Docker label allow-list, and the label-to-config reader. Keeping these as
// named constants prevents drift like the issue #620 part B regression where
// label keys were renamed on the INI side but not the label side.
const (
	webhookGlobalKeyWebhooks             = "webhook-webhooks"
	webhookGlobalKeyAllowRemotePresets   = "webhook-allow-remote-presets"
	webhookGlobalKeyTrustedPresetSources = "webhook-trusted-preset-sources"
	webhookGlobalKeyPresetCacheTTL       = "webhook-preset-cache-ttl"
	webhookGlobalKeyPresetCacheDir       = "webhook-preset-cache-dir"
	webhookGlobalKeyAllowedHosts         = "webhook-allowed-hosts"

	// Legacy unprefixed Docker label key names — deprecated, see #620.
	legacyLabelKeyWebhooks             = "webhooks"
	legacyLabelKeyAllowRemotePresets   = "allow-remote-presets"
	legacyLabelKeyTrustedPresetSources = "trusted-preset-sources"
	legacyLabelKeyPresetCacheTTL       = "preset-cache-ttl"
	legacyLabelKeyPresetCacheDir       = "preset-cache-dir"
)

// WebhookConfigs holds all parsed webhook configurations
type WebhookConfigs struct {
	Global          *middlewares.WebhookGlobalConfig
	Webhooks        map[string]*middlewares.WebhookConfig
	Manager         *middlewares.WebhookManager
	iniWebhookNames map[string]struct{} // tracks names defined in INI (protected from label overwrite)
}

// NewWebhookConfigs creates a new WebhookConfigs with defaults
func NewWebhookConfigs() *WebhookConfigs {
	return &WebhookConfigs{
		Global:   middlewares.DefaultWebhookGlobalConfig(),
		Webhooks: make(map[string]*middlewares.WebhookConfig),
	}
}

// InitManager initializes the webhook manager with the parsed configurations
func (wc *WebhookConfigs) InitManager() error {
	wc.Manager = middlewares.NewWebhookManager(wc.Global)

	for name, config := range wc.Webhooks {
		config.Name = name
		if err := wc.Manager.Register(config); err != nil {
			return fmt.Errorf("register webhook %q: %w", name, err)
		}
	}

	return nil
}

// parseWebhookSections parses [webhook "name"] sections from INI config
func parseWebhookSections(cfg *ini.File, c *Config) error {
	if c.WebhookConfigs == nil {
		c.WebhookConfigs = NewWebhookConfigs()
	}

	for _, section := range cfg.Sections() {
		name := strings.TrimSpace(section.Name())

		// Parse [webhook "name"] sections
		if strings.HasPrefix(name, webhookSection) {
			webhookName := parseWebhookName(name)
			if webhookName == "" {
				return fmt.Errorf("webhook section must have a name: [webhook \"name\"]")
			}

			config := middlewares.DefaultWebhookConfig()
			config.Name = webhookName

			if err := parseWebhookConfig(section, config); err != nil {
				return fmt.Errorf("parse webhook %q: %w", webhookName, err)
			}

			c.WebhookConfigs.Webhooks[webhookName] = config
			if c.WebhookConfigs.iniWebhookNames == nil {
				c.WebhookConfigs.iniWebhookNames = make(map[string]struct{})
			}
			c.WebhookConfigs.iniWebhookNames[webhookName] = struct{}{}
		}
	}

	return nil
}

// parseWebhookName extracts the webhook name from section name
// e.g., "webhook \"slack-alerts\"" -> "slack-alerts"
func parseWebhookName(sectionName string) string {
	// Format: webhook "name" or webhook 'name'
	sectionName = strings.TrimPrefix(sectionName, webhookSection)
	sectionName = strings.TrimSpace(sectionName)

	// Remove quotes
	if len(sectionName) >= 2 {
		if (sectionName[0] == '"' && sectionName[len(sectionName)-1] == '"') ||
			(sectionName[0] == '\'' && sectionName[len(sectionName)-1] == '\'') {
			return sectionName[1 : len(sectionName)-1]
		}
	}

	return sectionName
}

// parseWebhookConfig parses webhook configuration from an INI section.
// Currently always returns nil as all fields are optional with defaults.
//
//nolint:unparam // error return kept for future validation additions
func parseWebhookConfig(section *ini.Section, config *middlewares.WebhookConfig) error {
	if key, err := section.GetKey("preset"); err == nil {
		config.Preset = ExpandEnvVars(key.String())
	}

	if key, err := section.GetKey("id"); err == nil {
		config.ID = ExpandEnvVars(key.String())
	}

	if key, err := section.GetKey("secret"); err == nil {
		config.Secret = ExpandEnvVars(key.String())
	}

	if key, err := section.GetKey("url"); err == nil {
		config.URL = ExpandEnvVars(key.String())
	}

	if key, err := section.GetKey("trigger"); err == nil {
		config.Trigger = middlewares.TriggerType(ExpandEnvVars(key.String()))
	}

	if key, err := section.GetKey("timeout"); err == nil {
		if d, err := key.Duration(); err == nil {
			config.Timeout = d
		}
	}

	if key, err := section.GetKey("retry-count"); err == nil {
		if n, err := key.Int(); err == nil {
			config.RetryCount = n
		}
	}

	if key, err := section.GetKey("retry-delay"); err == nil {
		if d, err := key.Duration(); err == nil {
			config.RetryDelay = d
		}
	}

	if key, err := section.GetKey("link"); err == nil {
		config.Link = ExpandEnvVars(key.String())
	}

	if key, err := section.GetKey("link-text"); err == nil {
		config.LinkText = ExpandEnvVars(key.String())
	}

	return nil
}

// JobWebhookConfig holds per-job webhook configuration
type JobWebhookConfig struct {
	// Webhooks is a comma-separated list of webhook names for this job
	Webhooks string `gcfg:"webhooks" mapstructure:"webhooks"`
}

// GetWebhookNames returns the list of webhook names for a job
func (c *JobWebhookConfig) GetWebhookNames() []string {
	return middlewares.ParseWebhookNames(c.Webhooks)
}

// syncWebhookConfigs detects changes in label-defined webhooks and re-initializes
// the webhook manager if needed (called during container update events).
func (c *Config) syncWebhookConfigs(parsed *WebhookConfigs) {
	if parsed == nil {
		return
	}

	if !c.applyWebhookChanges(parsed) {
		return
	}

	// Re-initialize webhook manager with updated configs
	if err := c.WebhookConfigs.InitManager(); err != nil {
		c.logger.Error(fmt.Sprintf("Failed to re-initialize webhook manager: %v", err))
		return
	}
	c.logger.Info("Webhook configuration updated from container labels", "count", len(c.WebhookConfigs.Webhooks))

	c.rebuildAllMiddlewares()
}

// applyWebhookChanges merges parsed webhook configs into the current config,
// returning true if any changes were detected. INI-defined webhooks are never
// overwritten by labels (security: prevents container label hijacking).
func (c *Config) applyWebhookChanges(parsed *WebhookConfigs) bool {
	changed := false

	for name, wh := range parsed.Webhooks {
		// Never overwrite INI-defined webhooks from labels
		if _, isINI := c.WebhookConfigs.iniWebhookNames[name]; isINI {
			continue
		}
		existing, exists := c.WebhookConfigs.Webhooks[name]
		if !exists {
			c.WebhookConfigs.Webhooks[name] = wh
			changed = true
			continue
		}
		if webhookConfigChanged(existing, wh) {
			c.WebhookConfigs.Webhooks[name] = wh
			changed = true
		}
	}

	// Remove label-defined webhooks no longer present in parsed labels
	for name := range c.WebhookConfigs.Webhooks {
		if _, isINI := c.WebhookConfigs.iniWebhookNames[name]; isINI {
			continue
		}
		if _, stillPresent := parsed.Webhooks[name]; !stillPresent {
			delete(c.WebhookConfigs.Webhooks, name)
			changed = true
		}
	}

	return changed
}

// webhookConfigChanged returns true if any configurable field differs between two webhook configs.
func webhookConfigChanged(a, b *middlewares.WebhookConfig) bool {
	return a.Preset != b.Preset || a.URL != b.URL ||
		a.ID != b.ID || a.Secret != b.Secret ||
		a.Trigger != b.Trigger || a.Timeout != b.Timeout ||
		a.RetryCount != b.RetryCount || a.RetryDelay != b.RetryDelay ||
		a.Link != b.Link || a.LinkText != b.LinkText
}

// rebuildAllMiddlewares resets and rebuilds scheduler and job middlewares.
// Used after webhook config changes to re-attach updated webhook middlewares.
func (c *Config) rebuildAllMiddlewares() {
	c.sh.ResetMiddlewares()
	c.buildSchedulerMiddlewares(c.sh)
	wm := c.getWebhookManager()
	// All jobs (including disabled/paused) remain in Jobs, so one loop suffices.
	for _, j := range c.sh.Jobs {
		if jc, ok := j.(jobConfig); ok {
			jc.ResetMiddlewares()
			jc.buildMiddlewares(wm)
			j.Use(c.sh.Middlewares()...)
		}
	}
}

// mergeWebhookConfigs merges label-defined webhooks into the main config.
// INI-defined webhooks take precedence: label webhooks are added only if no
// INI webhook with the same name exists.
func mergeWebhookConfigs(c *Config, parsed *WebhookConfigs) {
	if parsed == nil || len(parsed.Webhooks) == 0 {
		return
	}
	if c.WebhookConfigs == nil {
		c.WebhookConfigs = NewWebhookConfigs()
	}

	for name, wh := range parsed.Webhooks {
		if _, exists := c.WebhookConfigs.Webhooks[name]; exists {
			c.logger.Warn(fmt.Sprintf("ignoring label-defined webhook %q because an INI webhook with the same name exists", name))
			continue
		}
		c.WebhookConfigs.Webhooks[name] = wh
	}

	// Merge global webhook settings from labels if not already set from INI
	if c.WebhookConfigs.Global.Webhooks == "" && parsed.Global.Webhooks != "" {
		c.WebhookConfigs.Global.Webhooks = parsed.Global.Webhooks
	}
	if c.WebhookConfigs.Global.AllowedHosts == "*" && parsed.Global.AllowedHosts != "*" {
		c.WebhookConfigs.Global.AllowedHosts = parsed.Global.AllowedHosts
	}
}

// buildWebhookConfigsFromLabels creates WebhookConfig objects from label-parsed webhook params.
func buildWebhookConfigsFromLabels(c *Config, webhookLabels map[string]map[string]string) {
	if len(webhookLabels) == 0 {
		return
	}
	if c.WebhookConfigs == nil {
		c.WebhookConfigs = NewWebhookConfigs()
	}
	for name, params := range webhookLabels {
		wh := middlewares.DefaultWebhookConfig()
		wh.Name = name
		applyWebhookLabelParams(wh, params)
		c.WebhookConfigs.Webhooks[name] = wh
	}
}

// applyWebhookLabelParams applies flat label params to a WebhookConfig.
// This mirrors parseWebhookConfig but works from a string map (Docker labels)
// instead of an INI section.
func applyWebhookLabelParams(config *middlewares.WebhookConfig, params map[string]string) {
	for key, val := range params {
		switch strings.ToLower(key) {
		case "preset":
			config.Preset = val
		case "id":
			config.ID = val
		case "secret": //nolint:goconst // matches gcfg:"secret" struct tag — Go syntax requires literal in tag
			config.Secret = val
		case "url":
			config.URL = val
		case "trigger":
			config.Trigger = middlewares.TriggerType(val)
		case "timeout":
			if d, err := time.ParseDuration(val); err == nil {
				config.Timeout = d
			}
		case "retry-count":
			if n, err := strconv.Atoi(val); err == nil {
				config.RetryCount = n
			}
		case "retry-delay":
			if d, err := time.ParseDuration(val); err == nil {
				config.RetryDelay = d
			}
		case "link":
			config.Link = val
		case "link-text":
			config.LinkText = val
		}
	}
}

// legacyWebhookLabelAliases maps the OLD unprefixed Docker label keys (left
// behind by PR #618 when it renamed the INI side to webhook-*) to the
// canonical webhook-* form. A user copying their INI keys verbatim into Docker
// labels (a reasonable workflow) previously hit "Unknown global label keys"
// and silently lost the values. For one release we still accept the legacy
// forms but emit a deprecation warning so operators have time to migrate
// (#620 part B).
var legacyWebhookLabelAliases = map[string]string{
	legacyLabelKeyWebhooks:             webhookGlobalKeyWebhooks,
	legacyLabelKeyAllowRemotePresets:   webhookGlobalKeyAllowRemotePresets,
	legacyLabelKeyTrustedPresetSources: webhookGlobalKeyTrustedPresetSources,
	legacyLabelKeyPresetCacheTTL:       webhookGlobalKeyPresetCacheTTL,
	legacyLabelKeyPresetCacheDir:       webhookGlobalKeyPresetCacheDir,
}

// loggedDeprecatedLabel guards the per-key deprecation log so reconcile-driven
// re-invocations of applyGlobalWebhookLabels don't spam the log on every
// container event. Each old name fires at most once for the lifetime of the
// process.
var loggedDeprecatedLabel sync.Map // map[string]struct{}

// pickWebhookLabel returns the value for the canonical (webhook-prefixed) key
// if present; otherwise falls back to the legacy unprefixed form (logging a
// one-shot deprecation warning). The new form takes precedence so operators
// mid-migration don't see the legacy key clobber their explicit new-style
// value.
func pickWebhookLabel(globals map[string]any, canonical string, logger *slog.Logger) (string, bool) {
	if v, ok := globals[canonical]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
		return "", false
	}
	for legacy, target := range legacyWebhookLabelAliases {
		if target != canonical {
			continue
		}
		v, ok := globals[legacy]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return "", false
		}
		if _, loaded := loggedDeprecatedLabel.LoadOrStore(legacy, struct{}{}); !loaded && logger != nil {
			logger.Warn(fmt.Sprintf(
				"DEPRECATED Docker label key %q — use %q instead. "+
					"The legacy form will be removed in a future release. "+
					"See: https://github.com/netresearch/ofelia/issues/620",
				"ofelia."+legacy, "ofelia."+canonical,
			))
		}
		return s, true
	}
	return "", false
}

// applyGlobalWebhookLabels extracts webhook-specific keys from the globals map
// (populated from service container labels) into the Config's WebhookConfigs.Global.
// Accepts both the canonical webhook-* form (matching INI naming) and the
// legacy unprefixed form left behind by #618 (with a one-shot deprecation
// warning per legacy key).
func applyGlobalWebhookLabels(c *Config, globals map[string]any) {
	if c.WebhookConfigs == nil {
		c.WebhookConfigs = NewWebhookConfigs()
	}

	if s, ok := pickWebhookLabel(globals, webhookGlobalKeyWebhooks, c.logger); ok {
		c.WebhookConfigs.Global.Webhooks = s
	}
	if s, ok := pickWebhookLabel(globals, webhookGlobalKeyAllowRemotePresets, c.logger); ok {
		c.WebhookConfigs.Global.AllowRemotePresets, _ = strconv.ParseBool(s)
	}
	if s, ok := pickWebhookLabel(globals, webhookGlobalKeyTrustedPresetSources, c.logger); ok {
		c.WebhookConfigs.Global.TrustedPresetSources = s
	}
	if s, ok := pickWebhookLabel(globals, webhookGlobalKeyPresetCacheTTL, c.logger); ok {
		if d, err := time.ParseDuration(s); err == nil {
			c.WebhookConfigs.Global.PresetCacheTTL = d
		}
	}
	if s, ok := pickWebhookLabel(globals, webhookGlobalKeyPresetCacheDir, c.logger); ok {
		c.WebhookConfigs.Global.PresetCacheDir = s
	}
	// webhook-allowed-hosts has no legacy alias — it always used the prefixed form.
	if v, ok := globals[webhookGlobalKeyAllowedHosts]; ok {
		if s, ok := v.(string); ok {
			c.WebhookConfigs.Global.AllowedHosts = s
		}
	}
}
