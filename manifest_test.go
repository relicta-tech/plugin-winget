package main

import (
	"strings"
	"testing"
)

func TestGenerateManifests(t *testing.T) {
	cfg := &Config{
		PackageID: "MyOrg.MyApp",
		Metadata: MetadataConfig{
			Publisher:        "My Organization",
			PublisherURL:     "https://myorg.com",
			Name:             "My Application",
			ShortDescription: "A useful application",
			License:          "MIT",
			LicenseURL:       "https://github.com/myorg/myapp/LICENSE",
			Moniker:          "myapp",
			Tags:             []string{"utility", "productivity"},
		},
		Locales: []LocaleConfig{
			{
				Locale:      "en-US",
				Description: "A full description of the application",
			},
		},
	}

	installers := []Installer{
		{
			Architecture:    "x64",
			InstallerType:   "msi",
			InstallerURL:    "https://example.com/myapp-1.0.0-x64.msi",
			InstallerSha256: "ABC123",
		},
	}

	manifests, err := GenerateManifests(cfg, "1.0.0", installers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check version manifest
	if manifests.Version.PackageIdentifier != "MyOrg.MyApp" {
		t.Errorf("expected PackageIdentifier 'MyOrg.MyApp', got '%s'", manifests.Version.PackageIdentifier)
	}
	if manifests.Version.PackageVersion != "1.0.0" {
		t.Errorf("expected PackageVersion '1.0.0', got '%s'", manifests.Version.PackageVersion)
	}
	if manifests.Version.ManifestType != "version" {
		t.Errorf("expected ManifestType 'version', got '%s'", manifests.Version.ManifestType)
	}

	// Check installer manifest
	if len(manifests.Installer.Installers) != 1 {
		t.Errorf("expected 1 installer, got %d", len(manifests.Installer.Installers))
	}
	if manifests.Installer.ManifestType != "installer" {
		t.Errorf("expected ManifestType 'installer', got '%s'", manifests.Installer.ManifestType)
	}

	// Check locale manifest
	if manifests.Locale.Publisher != "My Organization" {
		t.Errorf("expected Publisher 'My Organization', got '%s'", manifests.Locale.Publisher)
	}
	if manifests.Locale.ShortDescription != "A useful application" {
		t.Errorf("expected ShortDescription 'A useful application', got '%s'", manifests.Locale.ShortDescription)
	}
	if manifests.Locale.Description != "A full description of the application" {
		t.Errorf("expected Description from locale, got '%s'", manifests.Locale.Description)
	}
	if manifests.Locale.ManifestType != "defaultLocale" {
		t.Errorf("expected ManifestType 'defaultLocale', got '%s'", manifests.Locale.ManifestType)
	}

	// Check path
	expectedPath := "manifests/m/MyOrg.MyApp/1.0.0"
	if manifests.Path != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, manifests.Path)
	}
}

func TestGenerateManifestsInvalidPackageID(t *testing.T) {
	cfg := &Config{
		PackageID: "InvalidPackageID",
	}

	_, err := GenerateManifests(cfg, "1.0.0", nil)
	if err == nil {
		t.Error("expected error for invalid package ID")
	}
}

func TestManifestSetYAML(t *testing.T) {
	manifests := &ManifestSet{
		Version: &VersionManifest{
			PackageIdentifier: "MyOrg.MyApp",
			PackageVersion:    "1.0.0",
			DefaultLocale:     "en-US",
			ManifestType:      "version",
			ManifestVersion:   ManifestVersion,
		},
		Installer: &InstallerManifest{
			PackageIdentifier: "MyOrg.MyApp",
			PackageVersion:    "1.0.0",
			Installers: []Installer{
				{
					Architecture:    "x64",
					InstallerType:   "msi",
					InstallerURL:    "https://example.com/app.msi",
					InstallerSha256: "ABC123",
				},
			},
			ManifestType:    "installer",
			ManifestVersion: ManifestVersion,
		},
		Locale: &LocaleManifest{
			PackageIdentifier: "MyOrg.MyApp",
			PackageVersion:    "1.0.0",
			PackageLocale:     "en-US",
			Publisher:         "My Org",
			PackageName:       "My App",
			License:           "MIT",
			ShortDescription:  "A test app",
			ManifestType:      "defaultLocale",
			ManifestVersion:   ManifestVersion,
		},
		Path: "manifests/m/MyOrg.MyApp/1.0.0",
	}

	// Test version YAML
	versionYAML, err := manifests.VersionYAML()
	if err != nil {
		t.Fatalf("failed to generate version YAML: %v", err)
	}
	if !strings.Contains(versionYAML, "PackageIdentifier: MyOrg.MyApp") {
		t.Error("version YAML missing PackageIdentifier")
	}

	// Test installer YAML
	installerYAML, err := manifests.InstallerYAML()
	if err != nil {
		t.Fatalf("failed to generate installer YAML: %v", err)
	}
	if !strings.Contains(installerYAML, "InstallerUrl: https://example.com/app.msi") {
		t.Error("installer YAML missing InstallerUrl")
	}

	// Test locale YAML
	localeYAML, err := manifests.LocaleYAML()
	if err != nil {
		t.Fatalf("failed to generate locale YAML: %v", err)
	}
	if !strings.Contains(localeYAML, "Publisher: My Org") {
		t.Error("locale YAML missing Publisher")
	}
}

func TestManifestSetGetFiles(t *testing.T) {
	manifests := &ManifestSet{
		Version: &VersionManifest{
			PackageIdentifier: "MyOrg.MyApp",
			PackageVersion:    "1.0.0",
			DefaultLocale:     "en-US",
			ManifestType:      "version",
			ManifestVersion:   ManifestVersion,
		},
		Installer: &InstallerManifest{
			PackageIdentifier: "MyOrg.MyApp",
			PackageVersion:    "1.0.0",
			Installers:        []Installer{},
			ManifestType:      "installer",
			ManifestVersion:   ManifestVersion,
		},
		Locale: &LocaleManifest{
			PackageIdentifier: "MyOrg.MyApp",
			PackageVersion:    "1.0.0",
			PackageLocale:     "en-US",
			Publisher:         "My Org",
			PackageName:       "My App",
			License:           "MIT",
			ShortDescription:  "A test app",
			ManifestType:      "defaultLocale",
			ManifestVersion:   ManifestVersion,
		},
		Path: "manifests/m/MyOrg.MyApp/1.0.0",
	}

	files, err := manifests.GetFiles()
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	expectedFiles := []string{
		"manifests/m/MyOrg.MyApp/1.0.0/MyOrg.MyApp.yaml",
		"manifests/m/MyOrg.MyApp/1.0.0/MyOrg.MyApp.installer.yaml",
		"manifests/m/MyOrg.MyApp/1.0.0/MyOrg.MyApp.locale.en-US.yaml",
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("expected %d files, got %d", len(expectedFiles), len(files))
	}

	for _, path := range expectedFiles {
		if _, ok := files[path]; !ok {
			t.Errorf("missing file: %s", path)
		}
	}

	// Check that files have YAML header
	for path, content := range files {
		if !strings.HasPrefix(content, "# Created using Relicta") {
			t.Errorf("file %s missing YAML header", path)
		}
	}
}

func TestAddYAMLHeader(t *testing.T) {
	content := "PackageIdentifier: Test.App"
	result := addYAMLHeader(content)

	if !strings.HasPrefix(result, "# Created using Relicta") {
		t.Error("missing Relicta header")
	}
	if !strings.Contains(result, content) {
		t.Error("original content missing")
	}
}
