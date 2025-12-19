# plugin-winget

Relicta plugin for publishing packages to Windows Package Manager (winget).

## Features

- Generate winget manifest files (version, installer, locale)
- Calculate SHA256 hashes for installers
- Support multiple architectures (x86, x64, arm, arm64)
- Create pull requests to winget-pkgs repository
- Support for MSI, MSIX, EXE, and other installer types
- Automatic fork management

## Installation

```bash
relicta plugin install winget
```

## Configuration

```yaml
plugins:
  - name: winget
    enabled: true
    hooks:
      - PostPublish
    config:
      # Package identifier (required)
      package_id: "MyOrg.MyApp"

      # GitHub token for PR creation
      github_token: ${GITHUB_TOKEN}

      # Installer configuration
      installers:
        - url: "https://github.com/myorg/myapp/releases/download/v{{.Version}}/myapp-{{.Version}}-x64.msi"
          architecture: "x64"
          type: "msi"
          scope: "machine"

        - url: "https://github.com/myorg/myapp/releases/download/v{{.Version}}/myapp-{{.Version}}-arm64.msi"
          architecture: "arm64"
          type: "msi"

      # Package metadata
      metadata:
        publisher: "My Organization"
        publisher_url: "https://myorg.com"
        name: "My Application"
        short_description: "A useful application"
        license: "MIT"
        license_url: "https://github.com/myorg/myapp/blob/main/LICENSE"
        tags:
          - "utility"
          - "productivity"
        moniker: "myapp"

      # Locale configuration
      locales:
        - locale: "en-US"
          description: "Full description of the application..."

      # PR settings
      pull_request:
        base_branch: "master"
        title: "New version: {{.PackageId}} version {{.Version}}"
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub token with repo scope |
| `WINGET_PKGS_FORK` | Fork repository (owner/repo) |

## Manifest Generation

The plugin generates three manifest files:

1. **Version manifest** (`Publisher.PackageName.yaml`)
   - Package identifier and version
   - Default locale

2. **Installer manifest** (`Publisher.PackageName.installer.yaml`)
   - Installer URLs and SHA256 hashes
   - Architecture and installer type
   - Installation switches

3. **Locale manifest** (`Publisher.PackageName.locale.en-US.yaml`)
   - Publisher and package metadata
   - License information
   - Tags and descriptions

## Supported Installer Types

- `msi` - Windows Installer
- `msix` - MSIX packages
- `exe` - Executable installers
- `zip` - ZIP archives
- `inno` - Inno Setup
- `nullsoft` - NSIS installers
- `wix` - WiX Toolset
- `burn` - WiX Burn bundles

## Dry Run

Test the plugin without creating a PR:

```bash
relicta publish --dry-run
```

## Requirements

- GitHub token with `public_repo` scope
- Installer files must be publicly accessible

## Development

```bash
# Build
go build -o plugin-winget

# Test
go test -v ./...

# Lint
golangci-lint run
```

## License

MIT License - see [LICENSE](LICENSE) for details.
