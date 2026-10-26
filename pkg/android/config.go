// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alca-lang/alca/pkg/apm"
)

// Config represents Android-specific project configuration.
type Config struct {
	PackageName         string   `json:"package"`
	Label               string   `json:"label"`
	VersionCode         int      `json:"version_code"`
	VersionName         string   `json:"version_name"`
	MinSDK              int      `json:"min_sdk"`
	TargetSDK           int      `json:"target_sdk"`
	Permissions         []string `json:"permissions"`
	PermissionsExplicit bool     `json:"-"`
	Activity            string   `json:"activity"`
	Orientation         string   `json:"orientation"`
	Theme               string   `json:"theme"`
}

// DefaultConfig returns default Android settings for a given project name.
func DefaultConfig(name string) *Config {
	cleanName := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	cleanName = strings.ReplaceAll(cleanName, " ", "_")
	if cleanName == "" {
		cleanName = "myapp"
	}
	label := strings.Title(strings.ReplaceAll(name, "_", " "))
	if label == "" {
		label = "My App"
	}
	return &Config{
		PackageName: fmt.Sprintf("com.alca.%s", cleanName),
		Label:       label,
		VersionCode: 1,
		VersionName: "1.0.0",
		MinSDK:      21,
		TargetSDK:   34, // Modern target SDK (Android 14)
		Permissions: []string{
			"android.permission.INTERNET",
			"android.permission.ACCESS_NETWORK_STATE",
			"android.permission.VIBRATE",
		},
		Activity:    "alca.runtime.MainActivity",
		Orientation: "unspecified",
		Theme:       "@android:style/Theme.NoTitleBar.Fullscreen",
	}
}

// LoadConfig reads android settings from alca.toml.
func LoadConfig(dir string) (*Config, error) {
	fallback := DefaultConfig(filepath.Base(dir))
	manifestPath := filepath.Join(dir, "alca.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return nil, fmt.Errorf("reading alca.toml: %w", err)
	}

	manifest, err := apm.ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("parsing alca.toml: %w", err)
	}

	cfg := DefaultConfig(manifest.Package.Name)
	if manifest.Package.Version != "" {
		cfg.VersionName = manifest.Package.Version
	}
	if manifest.Package.Description != "" {
		cfg.Label = manifest.Package.Description
	}

	// Read [android] section if present in Extra
	if androidSec, ok := manifest.Extra["android"]; ok {
		if v, ok := androidSec["package"]; ok && strings.TrimSpace(v) != "" {
			cfg.PackageName = strings.TrimSpace(v)
		}
		if v, ok := androidSec["label"]; ok && strings.TrimSpace(v) != "" {
			cfg.Label = strings.TrimSpace(v)
		}
		if v, ok := androidSec["activity"]; ok && strings.TrimSpace(v) != "" {
			cfg.Activity = strings.TrimSpace(v)
		}
		if v, ok := androidSec["theme"]; ok && strings.TrimSpace(v) != "" {
			cfg.Theme = strings.TrimSpace(v)
		}
		if v, ok := androidSec["orientation"]; ok && strings.TrimSpace(v) != "" {
			cfg.Orientation = strings.TrimSpace(v)
		}
		if v, ok := androidSec["version_code"]; ok && strings.TrimSpace(v) != "" {
			if code, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && code > 0 {
				cfg.VersionCode = code
			}
		}
		if v, ok := androidSec["version_name"]; ok && strings.TrimSpace(v) != "" {
			cfg.VersionName = strings.TrimSpace(v)
		}
		if v, ok := androidSec["min_sdk"]; ok && strings.TrimSpace(v) != "" {
			if sdk, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && sdk > 0 {
				cfg.MinSDK = sdk
			}
		}
		if v, ok := androidSec["target_sdk"]; ok && strings.TrimSpace(v) != "" {
			if sdk, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && sdk > 0 {
				cfg.TargetSDK = sdk
			}
		}
		if v, ok := androidSec["permissions"]; ok {
			cfg.PermissionsExplicit = true
			trimmed := strings.TrimSpace(v)
			if trimmed == "" || trimmed == "[]" {
				cfg.Permissions = []string{}
			} else {
				// Parse JSON array or comma separated
				var perms []string
				if err := json.Unmarshal([]byte(trimmed), &perms); err == nil {
					cfg.Permissions = perms
				} else {
					// Comma-separated fallback
					parts := strings.Split(trimmed, ",")
					var cleaned []string
					for _, p := range parts {
						p = strings.Trim(strings.TrimSpace(p), "[]\"' ")
						if p != "" {
							cleaned = append(cleaned, p)
						}
					}
					cfg.Permissions = cleaned
				}
			}
		}
	}

	return cfg, nil
}
