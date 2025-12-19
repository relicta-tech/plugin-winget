package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestVersion is the current winget manifest schema version.
const ManifestVersion = "1.6.0"

// VersionManifest represents the version manifest file.
type VersionManifest struct {
	PackageIdentifier string `yaml:"PackageIdentifier"`
	PackageVersion    string `yaml:"PackageVersion"`
	DefaultLocale     string `yaml:"DefaultLocale"`
	ManifestType      string `yaml:"ManifestType"`
	ManifestVersion   string `yaml:"ManifestVersion"`
}

// InstallerManifest represents the installer manifest file.
type InstallerManifest struct {
	PackageIdentifier string      `yaml:"PackageIdentifier"`
	PackageVersion    string      `yaml:"PackageVersion"`
	Installers        []Installer `yaml:"Installers"`
	ManifestType      string      `yaml:"ManifestType"`
	ManifestVersion   string      `yaml:"ManifestVersion"`
}

// Installer represents a single installer entry.
type Installer struct {
	Architecture      string            `yaml:"Architecture"`
	InstallerType     string            `yaml:"InstallerType"`
	InstallerURL      string            `yaml:"InstallerUrl"`
	InstallerSha256   string            `yaml:"InstallerSha256"`
	Scope             string            `yaml:"Scope,omitempty"`
	InstallerSwitches map[string]string `yaml:"InstallerSwitches,omitempty"`
	ProductCode       string            `yaml:"ProductCode,omitempty"`
}

// LocaleManifest represents the locale manifest file.
type LocaleManifest struct {
	PackageIdentifier   string   `yaml:"PackageIdentifier"`
	PackageVersion      string   `yaml:"PackageVersion"`
	PackageLocale       string   `yaml:"PackageLocale"`
	Publisher           string   `yaml:"Publisher"`
	PublisherURL        string   `yaml:"PublisherUrl,omitempty"`
	PublisherSupportURL string   `yaml:"PublisherSupportUrl,omitempty"`
	PackageName         string   `yaml:"PackageName"`
	License             string   `yaml:"License"`
	LicenseURL          string   `yaml:"LicenseUrl,omitempty"`
	Copyright           string   `yaml:"Copyright,omitempty"`
	ShortDescription    string   `yaml:"ShortDescription"`
	Description         string   `yaml:"Description,omitempty"`
	Moniker             string   `yaml:"Moniker,omitempty"`
	Tags                []string `yaml:"Tags,omitempty"`
	PackageURL          string   `yaml:"PackageUrl,omitempty"`
	ReleaseNotesURL     string   `yaml:"ReleaseNotesUrl,omitempty"`
	ManifestType        string   `yaml:"ManifestType"`
	ManifestVersion     string   `yaml:"ManifestVersion"`
}

// ManifestSet contains all generated manifest files.
type ManifestSet struct {
	Version   *VersionManifest
	Installer *InstallerManifest
	Locale    *LocaleManifest
	Path      string
}

// GenerateManifests generates all winget manifest files.
func GenerateManifests(cfg *Config, version string, installers []Installer) (*ManifestSet, error) {
	// Parse package ID
	parts := strings.SplitN(cfg.PackageID, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid package ID format: %s", cfg.PackageID)
	}
	publisher := parts[0]

	// Version manifest
	versionManifest := &VersionManifest{
		PackageIdentifier: cfg.PackageID,
		PackageVersion:    version,
		DefaultLocale:     "en-US",
		ManifestType:      "version",
		ManifestVersion:   ManifestVersion,
	}

	// Installer manifest
	installerManifest := &InstallerManifest{
		PackageIdentifier: cfg.PackageID,
		PackageVersion:    version,
		Installers:        installers,
		ManifestType:      "installer",
		ManifestVersion:   ManifestVersion,
	}

	// Locale manifest
	localeManifest := &LocaleManifest{
		PackageIdentifier:   cfg.PackageID,
		PackageVersion:      version,
		PackageLocale:       "en-US",
		Publisher:           cfg.Metadata.Publisher,
		PublisherURL:        cfg.Metadata.PublisherURL,
		PublisherSupportURL: cfg.Metadata.PublisherSupportURL,
		PackageName:         cfg.Metadata.Name,
		License:             cfg.Metadata.License,
		LicenseURL:          cfg.Metadata.LicenseURL,
		Copyright:           cfg.Metadata.Copyright,
		ShortDescription:    cfg.Metadata.ShortDescription,
		Moniker:             cfg.Metadata.Moniker,
		Tags:                cfg.Metadata.Tags,
		PackageURL:          cfg.Metadata.PackageURL,
		ReleaseNotesURL:     cfg.Metadata.ReleaseNotesURL,
		ManifestType:        "defaultLocale",
		ManifestVersion:     ManifestVersion,
	}

	// Add description from locales
	for _, locale := range cfg.Locales {
		if locale.Locale == "en-US" {
			localeManifest.Description = locale.Description
			break
		}
	}

	// Build path: manifests/p/Publisher/PackageName/version
	firstLetter := strings.ToLower(publisher[:1])
	path := fmt.Sprintf("manifests/%s/%s/%s", firstLetter, cfg.PackageID, version)

	return &ManifestSet{
		Version:   versionManifest,
		Installer: installerManifest,
		Locale:    localeManifest,
		Path:      path,
	}, nil
}

// VersionYAML returns the version manifest as YAML.
func (m *ManifestSet) VersionYAML() (string, error) {
	return toYAML(m.Version)
}

// InstallerYAML returns the installer manifest as YAML.
func (m *ManifestSet) InstallerYAML() (string, error) {
	return toYAML(m.Installer)
}

// LocaleYAML returns the locale manifest as YAML.
func (m *ManifestSet) LocaleYAML() (string, error) {
	return toYAML(m.Locale)
}

// GetFiles returns a map of file paths to content for committing.
func (m *ManifestSet) GetFiles() (map[string]string, error) {
	files := make(map[string]string)

	versionYAML, err := m.VersionYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to generate version manifest: %w", err)
	}
	files[fmt.Sprintf("%s/%s.yaml", m.Path, m.Version.PackageIdentifier)] = addYAMLHeader(versionYAML)

	installerYAML, err := m.InstallerYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to generate installer manifest: %w", err)
	}
	files[fmt.Sprintf("%s/%s.installer.yaml", m.Path, m.Installer.PackageIdentifier)] = addYAMLHeader(installerYAML)

	localeYAML, err := m.LocaleYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to generate locale manifest: %w", err)
	}
	files[fmt.Sprintf("%s/%s.locale.en-US.yaml", m.Path, m.Locale.PackageIdentifier)] = addYAMLHeader(localeYAML)

	return files, nil
}

// toYAML converts a struct to YAML string.
func toYAML(v any) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// addYAMLHeader adds the winget manifest YAML header comment.
func addYAMLHeader(content string) string {
	header := "# Created using Relicta\n# yaml-language-server: $schema=https://aka.ms/winget-manifest.version.1.6.0.schema.json\n\n"
	return header + content
}
