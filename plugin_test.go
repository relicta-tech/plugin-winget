package main

import (
	"testing"

	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &WinGetPlugin{}
	info := p.GetInfo()

	if info.Name != "winget" {
		t.Errorf("expected name 'winget', got '%s'", info.Name)
	}

	if info.Version != Version {
		t.Errorf("expected version '%s', got '%s'", Version, info.Version)
	}

	if len(info.Hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(info.Hooks))
	}

	if info.Hooks[0] != plugin.HookPostPublish {
		t.Error("expected PostPublish hook")
	}
}

func TestParseConfig(t *testing.T) {
	p := &WinGetPlugin{}

	tests := []struct {
		name     string
		raw      map[string]any
		validate func(t *testing.T, cfg *Config)
	}{
		{
			name: "basic config",
			raw: map[string]any{
				"package_id":   "MyOrg.MyApp",
				"github_token": "test-token",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.PackageID != "MyOrg.MyApp" {
					t.Errorf("expected package_id 'MyOrg.MyApp', got '%s'", cfg.PackageID)
				}
				if cfg.GitHubToken != "test-token" {
					t.Errorf("expected github_token 'test-token', got '%s'", cfg.GitHubToken)
				}
			},
		},
		{
			name: "with installers",
			raw: map[string]any{
				"package_id": "MyOrg.MyApp",
				"installers": []any{
					map[string]any{
						"url":          "https://example.com/app.msi",
						"architecture": "x64",
						"type":         "msi",
						"scope":        "machine",
					},
					map[string]any{
						"url":          "https://example.com/app-arm64.msi",
						"architecture": "arm64",
						"type":         "msi",
					},
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Installers) != 2 {
					t.Errorf("expected 2 installers, got %d", len(cfg.Installers))
				}
				if cfg.Installers[0].URL != "https://example.com/app.msi" {
					t.Errorf("wrong installer URL")
				}
				if cfg.Installers[0].Architecture != "x64" {
					t.Errorf("wrong architecture")
				}
				if cfg.Installers[0].Scope != "machine" {
					t.Errorf("wrong scope")
				}
			},
		},
		{
			name: "with metadata",
			raw: map[string]any{
				"package_id": "MyOrg.MyApp",
				"metadata": map[string]any{
					"publisher":         "My Org",
					"publisher_url":     "https://myorg.com",
					"name":              "My App",
					"short_description": "A test app",
					"license":           "MIT",
					"moniker":           "myapp",
					"tags":              []any{"utility", "tool"},
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Metadata.Publisher != "My Org" {
					t.Errorf("wrong publisher")
				}
				if cfg.Metadata.Name != "My App" {
					t.Errorf("wrong name")
				}
				if cfg.Metadata.Moniker != "myapp" {
					t.Errorf("wrong moniker")
				}
				if len(cfg.Metadata.Tags) != 2 {
					t.Errorf("expected 2 tags, got %d", len(cfg.Metadata.Tags))
				}
			},
		},
		{
			name: "with locales",
			raw: map[string]any{
				"package_id": "MyOrg.MyApp",
				"locales": []any{
					map[string]any{
						"locale":      "en-US",
						"description": "Full description here",
					},
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Locales) != 1 {
					t.Errorf("expected 1 locale, got %d", len(cfg.Locales))
				}
				if cfg.Locales[0].Locale != "en-US" {
					t.Errorf("wrong locale")
				}
				if cfg.Locales[0].Description != "Full description here" {
					t.Errorf("wrong description")
				}
			},
		},
		{
			name: "with PR config",
			raw: map[string]any{
				"package_id": "MyOrg.MyApp",
				"pull_request": map[string]any{
					"fork_owner":    "myuser",
					"base_branch":   "main",
					"title":         "Custom title: {{.PackageId}}",
					"delete_branch": false,
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.PullRequest.ForkOwner != "myuser" {
					t.Errorf("wrong fork_owner")
				}
				if cfg.PullRequest.BaseBranch != "main" {
					t.Errorf("wrong base_branch")
				}
				if cfg.PullRequest.Title != "Custom title: {{.PackageId}}" {
					t.Errorf("wrong title")
				}
				if cfg.PullRequest.DeleteBranch {
					t.Errorf("delete_branch should be false")
				}
			},
		},
		{
			name: "default PR config",
			raw: map[string]any{
				"package_id": "MyOrg.MyApp",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.PullRequest.BaseBranch != "master" {
					t.Errorf("expected default base_branch 'master', got '%s'", cfg.PullRequest.BaseBranch)
				}
				if !cfg.PullRequest.DeleteBranch {
					t.Errorf("delete_branch should default to true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.raw)
			tt.validate(t, cfg)
		})
	}
}

func TestIsValidPackageID(t *testing.T) {
	tests := []struct {
		id       string
		expected bool
	}{
		{"MyOrg.MyApp", true},
		{"Microsoft.VisualStudioCode", true},
		{"Publisher.Package", true},
		{"InvalidPackageID", false},
		{"", false},
		{".Package", false},
		{"Publisher.", false},
		{"Publisher.Sub.Package", true}, // This actually splits on first dot
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := isValidPackageID(tt.id)
			if result != tt.expected {
				t.Errorf("expected %v for '%s', got %v", tt.expected, tt.id, result)
			}
		})
	}
}

func TestIsValidArchitecture(t *testing.T) {
	tests := []struct {
		arch     string
		expected bool
	}{
		{"x86", true},
		{"x64", true},
		{"arm", true},
		{"arm64", true},
		{"", false},
		{"amd64", false},
		{"i386", false},
	}

	for _, tt := range tests {
		t.Run(tt.arch, func(t *testing.T) {
			result := isValidArchitecture(tt.arch)
			if result != tt.expected {
				t.Errorf("expected %v for '%s', got %v", tt.expected, tt.arch, result)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     map[string]string
		expected string
	}{
		{
			name:     "simple version",
			tmpl:     "https://example.com/app-{{.Version}}.msi",
			data:     map[string]string{"Version": "1.0.0"},
			expected: "https://example.com/app-1.0.0.msi",
		},
		{
			name:     "multiple placeholders",
			tmpl:     "{{.PackageId}} version {{.Version}}",
			data:     map[string]string{"PackageId": "MyOrg.MyApp", "Version": "2.0.0"},
			expected: "MyOrg.MyApp version 2.0.0",
		},
		{
			name:     "no placeholders",
			tmpl:     "https://example.com/app.msi",
			data:     map[string]string{"Version": "1.0.0"},
			expected: "https://example.com/app.msi",
		},
		{
			name:     "missing placeholder",
			tmpl:     "{{.Name}} {{.Missing}}",
			data:     map[string]string{"Name": "Test"},
			expected: "Test {{.Missing}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTemplate(tt.tmpl, tt.data)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
