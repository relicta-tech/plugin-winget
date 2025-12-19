package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/relicta-tech/relicta-plugin-sdk/helpers"
	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

// Version is set at build time.
var Version = "0.1.0"

// Config represents WinGet plugin configuration.
type Config struct {
	PackageID   string            `json:"package_id"`
	GitHubToken string            `json:"github_token"`
	Installers  []InstallerConfig `json:"installers"`
	Metadata    MetadataConfig    `json:"metadata"`
	Locales     []LocaleConfig    `json:"locales"`
	PullRequest PRConfig          `json:"pull_request"`
	Validate    bool              `json:"validate"`
	TestInstall bool              `json:"test_install"`
	DryRun      bool              `json:"dry_run"`
}

// InstallerConfig defines installer settings.
type InstallerConfig struct {
	URL          string            `json:"url"`
	Architecture string            `json:"architecture"`
	Type         string            `json:"type"`
	Switches     map[string]string `json:"switches"`
	Scope        string            `json:"scope"`
	ProductCode  string            `json:"product_code"`
}

// MetadataConfig defines package metadata.
type MetadataConfig struct {
	Publisher           string   `json:"publisher"`
	PublisherURL        string   `json:"publisher_url"`
	PublisherSupportURL string   `json:"publisher_support_url"`
	Name                string   `json:"name"`
	ShortDescription    string   `json:"short_description"`
	License             string   `json:"license"`
	LicenseURL          string   `json:"license_url"`
	Copyright           string   `json:"copyright"`
	PackageURL          string   `json:"package_url"`
	Tags                []string `json:"tags"`
	Moniker             string   `json:"moniker"`
	ReleaseNotesURL     string   `json:"release_notes_url"`
}

// LocaleConfig defines locale-specific metadata.
type LocaleConfig struct {
	Locale      string `json:"locale"`
	Description string `json:"description"`
}

// PRConfig defines pull request settings.
type PRConfig struct {
	ForkOwner    string `json:"fork_owner"`
	BaseBranch   string `json:"base_branch"`
	Title        string `json:"title"`
	DeleteBranch bool   `json:"delete_branch"`
}

// WinGetPlugin implements the WinGet package manager plugin.
type WinGetPlugin struct{}

// GetInfo returns plugin metadata.
func (p *WinGetPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "winget",
		Version:     Version,
		Description: "Windows Package Manager (winget) manifest generation and PR submission",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
	}
}

// Validate validates plugin configuration.
func (p *WinGetPlugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	cfg := p.parseConfig(config)
	vb := helpers.NewValidationBuilder()

	// Validate package ID
	if !isValidPackageID(cfg.PackageID) {
		vb.AddError("package_id", "Package ID must be in format Publisher.PackageName")
	}

	// Check GitHub token
	if cfg.GitHubToken == "" {
		vb.AddError("github_token", "GitHub token is required")
	}

	// Validate installers
	if len(cfg.Installers) == 0 {
		vb.AddError("installers", "At least one installer is required")
	}

	for i, installer := range cfg.Installers {
		if installer.URL == "" {
			vb.AddError(fmt.Sprintf("installers[%d].url", i), "Installer URL is required")
		}
		if !isValidArchitecture(installer.Architecture) {
			vb.AddError(fmt.Sprintf("installers[%d].architecture", i),
				"Architecture must be x86, x64, arm, or arm64")
		}
	}

	// Validate metadata
	if cfg.Metadata.Publisher == "" {
		vb.AddError("metadata.publisher", "Publisher is required")
	}
	if cfg.Metadata.Name == "" {
		vb.AddError("metadata.name", "Package name is required")
	}
	if cfg.Metadata.ShortDescription == "" {
		vb.AddError("metadata.short_description", "Short description is required")
	} else if len(cfg.Metadata.ShortDescription) > 256 {
		vb.AddError("metadata.short_description", "Short description must be <= 256 characters")
	}
	if cfg.Metadata.License == "" {
		vb.AddError("metadata.license", "License is required")
	}

	return vb.Build(), nil
}

// Execute runs the plugin for a given hook.
func (p *WinGetPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)
	cfg.DryRun = cfg.DryRun || req.DryRun
	logger := slog.Default().With("plugin", "winget", "hook", req.Hook)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.executePostPublish(ctx, &req.Context, cfg, logger)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled by winget plugin", req.Hook),
		}, nil
	}
}

func (p *WinGetPlugin) executePostPublish(ctx context.Context, releaseCtx *plugin.ReleaseContext, cfg *Config, logger *slog.Logger) (*plugin.ExecuteResponse, error) {
	version := releaseCtx.Version
	logger = logger.With("version", version, "package_id", cfg.PackageID)

	// Calculate installer hashes
	logger.Info("Calculating installer hashes")
	var installers []Installer
	for i, installerCfg := range cfg.Installers {
		// Render URL with version
		url := renderTemplate(installerCfg.URL, map[string]string{
			"Version": version,
		})

		logger.Info("Processing installer",
			"index", i,
			"architecture", installerCfg.Architecture,
			"url", url)

		var hash string
		if cfg.DryRun {
			logger.Info("[DRY-RUN] Would download and hash installer")
			hash = "0000000000000000000000000000000000000000000000000000000000000000"
		} else {
			var err error
			hash, err = CalculateInstallerHash(ctx, url)
			if err != nil {
				return &plugin.ExecuteResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to calculate hash for installer %d: %v", i, err),
				}, nil
			}
		}

		installer := Installer{
			Architecture:    installerCfg.Architecture,
			InstallerType:   installerCfg.Type,
			InstallerURL:    url,
			InstallerSha256: hash,
			Scope:           installerCfg.Scope,
			ProductCode:     installerCfg.ProductCode,
		}

		if len(installerCfg.Switches) > 0 {
			installer.InstallerSwitches = installerCfg.Switches
		}

		installers = append(installers, installer)
	}

	// Generate manifests
	logger.Info("Generating manifests")
	manifests, err := GenerateManifests(cfg, version, installers)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to generate manifests: %v", err),
		}, nil
	}

	if cfg.DryRun {
		logger.Info("[DRY-RUN] Generated manifests",
			"path", manifests.Path,
			"installers", len(installers))

		// Log manifest content for dry-run
		versionYAML, _ := manifests.VersionYAML()
		installerYAML, _ := manifests.InstallerYAML()
		localeYAML, _ := manifests.LocaleYAML()

		logger.Info("[DRY-RUN] Version manifest", "content", versionYAML)
		logger.Info("[DRY-RUN] Installer manifest", "content", installerYAML)
		logger.Info("[DRY-RUN] Locale manifest", "content", localeYAML)

		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("[DRY-RUN] Would create PR for %s version %s", cfg.PackageID, version),
		}, nil
	}

	// Create pull request
	logger.Info("Creating pull request to winget-pkgs")
	ghClient := NewGitHubClient(cfg.GitHubToken, cfg.PullRequest.ForkOwner)

	// Ensure fork exists
	logger.Info("Ensuring fork of winget-pkgs exists")
	forkOwner, err := ghClient.EnsureFork(ctx)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to ensure fork: %v", err),
		}, nil
	}
	logger.Info("Using fork", "owner", forkOwner)

	// Create PR
	prURL, err := ghClient.CreatePR(ctx, manifests, cfg.PullRequest)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create PR: %v", err),
		}, nil
	}

	logger.Info("Pull request created", "url", prURL)
	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Created PR for %s version %s: %s", cfg.PackageID, version, prURL),
	}, nil
}

func (p *WinGetPlugin) parseConfig(raw map[string]any) *Config {
	parser := helpers.NewConfigParser(raw)

	// Parse installers
	var installers []InstallerConfig
	if installersRaw, ok := raw["installers"].([]any); ok {
		for _, item := range installersRaw {
			if m, ok := item.(map[string]any); ok {
				installer := InstallerConfig{}
				if url, ok := m["url"].(string); ok {
					installer.URL = url
				}
				if arch, ok := m["architecture"].(string); ok {
					installer.Architecture = arch
				}
				if t, ok := m["type"].(string); ok {
					installer.Type = t
				}
				if scope, ok := m["scope"].(string); ok {
					installer.Scope = scope
				}
				if productCode, ok := m["product_code"].(string); ok {
					installer.ProductCode = productCode
				}
				if switches, ok := m["switches"].(map[string]any); ok {
					installer.Switches = make(map[string]string)
					for k, v := range switches {
						if s, ok := v.(string); ok {
							installer.Switches[k] = s
						}
					}
				}
				installers = append(installers, installer)
			}
		}
	}

	// Parse metadata
	metadata := MetadataConfig{}
	if metaRaw, ok := raw["metadata"].(map[string]any); ok {
		if pub, ok := metaRaw["publisher"].(string); ok {
			metadata.Publisher = pub
		}
		if pubURL, ok := metaRaw["publisher_url"].(string); ok {
			metadata.PublisherURL = pubURL
		}
		if pubSupport, ok := metaRaw["publisher_support_url"].(string); ok {
			metadata.PublisherSupportURL = pubSupport
		}
		if name, ok := metaRaw["name"].(string); ok {
			metadata.Name = name
		}
		if desc, ok := metaRaw["short_description"].(string); ok {
			metadata.ShortDescription = desc
		}
		if lic, ok := metaRaw["license"].(string); ok {
			metadata.License = lic
		}
		if licURL, ok := metaRaw["license_url"].(string); ok {
			metadata.LicenseURL = licURL
		}
		if copyright, ok := metaRaw["copyright"].(string); ok {
			metadata.Copyright = copyright
		}
		if pkgURL, ok := metaRaw["package_url"].(string); ok {
			metadata.PackageURL = pkgURL
		}
		if moniker, ok := metaRaw["moniker"].(string); ok {
			metadata.Moniker = moniker
		}
		if releaseURL, ok := metaRaw["release_notes_url"].(string); ok {
			metadata.ReleaseNotesURL = releaseURL
		}
		if tags, ok := metaRaw["tags"].([]any); ok {
			for _, t := range tags {
				if s, ok := t.(string); ok {
					metadata.Tags = append(metadata.Tags, s)
				}
			}
		}
	}

	// Parse locales
	var locales []LocaleConfig
	if localesRaw, ok := raw["locales"].([]any); ok {
		for _, item := range localesRaw {
			if m, ok := item.(map[string]any); ok {
				locale := LocaleConfig{}
				if l, ok := m["locale"].(string); ok {
					locale.Locale = l
				}
				if d, ok := m["description"].(string); ok {
					locale.Description = d
				}
				locales = append(locales, locale)
			}
		}
	}

	// Parse PR config
	prConfig := PRConfig{
		BaseBranch:   "master",
		Title:        "New version: {{.PackageId}} version {{.Version}}",
		DeleteBranch: true,
	}
	if prRaw, ok := raw["pull_request"].(map[string]any); ok {
		if forkOwner, ok := prRaw["fork_owner"].(string); ok {
			prConfig.ForkOwner = forkOwner
		}
		if baseBranch, ok := prRaw["base_branch"].(string); ok {
			prConfig.BaseBranch = baseBranch
		}
		if title, ok := prRaw["title"].(string); ok {
			prConfig.Title = title
		}
		if deleteBranch, ok := prRaw["delete_branch"].(bool); ok {
			prConfig.DeleteBranch = deleteBranch
		}
	}

	return &Config{
		PackageID:   parser.GetString("package_id", "", ""),
		GitHubToken: parser.GetString("github_token", "GITHUB_TOKEN", ""),
		Installers:  installers,
		Metadata:    metadata,
		Locales:     locales,
		PullRequest: prConfig,
		Validate:    parser.GetBool("validate", true),
		TestInstall: parser.GetBool("test_install", false),
		DryRun:      parser.GetBool("dry_run", false),
	}
}

// isValidPackageID checks if a package ID is in valid format.
func isValidPackageID(id string) bool {
	if id == "" {
		return false
	}
	parts := strings.SplitN(id, ".", 2)
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[1] != ""
}

// isValidArchitecture checks if architecture is valid.
func isValidArchitecture(arch string) bool {
	switch arch {
	case "x86", "x64", "arm", "arm64":
		return true
	default:
		return false
	}
}

// renderTemplate renders a simple template with placeholders.
func renderTemplate(tmpl string, data map[string]string) string {
	result := tmpl
	for key, value := range data {
		result = strings.ReplaceAll(result, "{{."+key+"}}", value)
	}
	return result
}
