# ⚡ gh-aca-utils — GitHub CLI Extension from the ACA Team

[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/greenstevester/gh-aca-utils)](https://github.com/greenstevester/gh-aca-utils/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/greenstevester/gh-aca-utils)](https://goreportcard.com/report/github.com/greenstevester/gh-aca-utils)
[![Build Status](https://github.com/greenstevester/gh-aca-utils/workflows/CI/badge.svg)](https://github.com/greenstevester/gh-aca-utils/actions)

A GitHub CLI extension for automating common aca-utils (Application Configuration Audit) tasks across repositories.

## ⚡ Quick Start (2 minutes)

1. **Install**: `gh extension install greenstevester/gh-aca-utils`
2. **Test**: `gh aca-utils ip-port --repo greenstevester/aca-example-repo --output table`
3. **Done!** You should see a table of any IP/port configurations found.

**Need help?** Run `gh aca-utils --help` to see all available commands.

## What is this?

It's a GitHub Command Line (gh cli) extension that provides two essential commands for managing tech-stack configurations:

- **`ip-port`** - Scans repositories to extract IP addresses and port configurations from config files
- **`flip-adapters`** - Toggles adapter settings (0↔1) in environment parameter files with optional Git workflow automation

![demo](docs/demo.gif)

## 🚀 Installation

### Simple Installation (Recommended)

```bash
gh extension install greenstevester/gh-aca-utils
```

<details>
<summary><strong>Manual Installation</strong></summary>

If the simple installation doesn't work, you can install manually:

```bash
# Clone and build from source
git clone https://github.com/greenstevester/gh-aca-utils.git
cd gh-aca-utils
go build -o gh-aca
gh extension install .
```

Or download a pre-built binary from the [releases page](https://github.com/greenstevester/gh-aca-utils/releases).

**Verify installation worked:**
```bash
gh aca --help
```

</details>

### Prerequisites

You must have these already installed:
- **GitHub CLI v2.0.0+** ([installation guide](https://github.com/cli/cli#installation))
- **Git authentication configured** (run `gh auth status` to verify)
- **Access to target repositories** (public repos work automatically)

## How to use it

NOTE: Once it's installed, you can run the gh "aca" extension commands ALWAYS WITH the "gh" prefix.

```bash
# Show available commands
gh aca-utils --help

# Get help for specific commands
gh aca-utils ip-port --help
gh aca-utils flip-adapters --help
gh aca-utils set-adapters --help
```

### IP/Port Extraction Command

Extract IP addresses and port configurations from a target repository across all branches:

```bash
# Scan a public repository for IP/port configurations
gh aca-utils ip-port --repo greenstevester/aca-example-repo --output table

# Scan ALL branches in the repository for comprehensive coverage
gh aca ip-port --repo myorg/microservice --all-branches --output table

# Scan with custom file patterns
gh aca-utils ip-port --repo greenstevester/aca-example-repo \
  --include "**/*.properties,**/*.yml,**/*.json" \
  --exclude "**/test/**,**/node_modules/**" \
  --output json

# Scan specific branch or tag
gh aca-utils ip-port --repo greenstevester/aca-example-repo --ref production --output csv

# Scan all branches with custom patterns and exclusions  
gh aca-utils ip-port --repo greenstevester/aca-example-repo --all-branches \
  --include "**/*.properties,**/*.env" \
  --exclude "**/test/**" --output json
```

**Supported file types**: `.properties`, `.yml`, `.yaml`, `.conf`, `.ini`, `.txt`, `.env`, `.json`

**Output formats**:
- `csv` (default) - Comma-separated values for spreadsheet import
- `table` - Human-readable formatted table
- `json` - Machine-readable JSON array

#### Example Output

```bash
$ gh aca-utils ip-port --repo greenstevester/aca-example-repo --output table

IP Key          IP Value      Port Key       Port Value  File Path                    Line
database.host   10.0.0.5      database.port  5432        config/app.properties        12
redis.host      172.16.0.10   redis.port     6379        config/cache.yml            8
api.host        192.168.1.100 api.port       8080        env/prod/service.properties  15
```

**With `--all-branches` flag:**
```bash
$ gh aca ip-port --repo myorg/config-repo --all-branches --output table

IP Key          IP Value      Port Key       Port Value  File Path                         Line
database.host   10.0.0.5      database.port  5432        [main] config/app.properties      12
redis.host      172.16.0.10   redis.port     6379        [main] config/cache.yml           8
api.host        192.168.1.100 api.port       8080        [main] env/prod/service.properties 15
test.host       127.0.0.1     test.port      9999        [dev] config/test.properties     5
staging.host    10.1.0.5      staging.port   8081        [staging] config/app.properties  20
```

### Adapter Management Commands

#### Set Adapters Command

Store frequently used adapter lists for reuse with the `flip-adapters` command:

```bash
# Store a list of adapters for easy reuse
gh aca set-adapters --adapters billing,payment,notifications,search

# List currently stored adapters
gh aca set-adapters --list

# Clear all stored adapters
gh aca set-adapters --clear
```

The adapters are stored in `~/.gh-aca-utils/adapters.txt` and can be automatically used by `flip-adapters` when `--adapters` is not specified.

#### Environment Adapter Toggle Command

Toggle adapter configurations in environment parameter files:

```bash
# Dry run (default) - show what would change
gh aca-utils flip-adapters --repo greenstevester/aca-example-repo \
  --env dev \
  --adapters billing,payment,notifications

# Apply changes and create commit + PR
gh aca-utils flip-adapters --repo greenstevester/aca-example-repo \
  --env production \
  --adapters search,analytics \
  --commit \
  --pr \
  --branch "toggle/prod-adapters"

# Apply changes only (no commit)
gh aca-utils flip-adapters --repo greenstevester/aca-example-repo \
  --env staging \
  --adapters crm,inventory \
  --dry-run=false

# Use stored adapters (no --adapters flag needed)
gh aca flip-adapters --repo myorg/service \
  --env production \
  --commit \
  --pr
```

**Required flags**:
- `--repo` - Target repository (format: `owner/repo`)  
- `--env` - Environment directory under `env/` (e.g., `dev`, `acc`, `prd`)

**Adapter specification** (one of these):
- `--adapters` - Comma-separated list of adapter keys to toggle
- Use stored adapters from `gh aca set-adapters` (when `--adapters` is omitted)

**Optional flags**:
- `--commit` - Create commit and push to new branch
- `--pr` - Create pull request (implies `--commit`)  
- `--branch` - Custom branch name (default: `toggle/adapters-{env}`)
- `--dry-run` - Show changes without applying (default: `true`)
- `--output` - Output format: `table` (default) or `json`

#### Example Output

```bash
$ gh aca-utils flip-adapters --repo greenstevester/aca-example-repo --env dev --adapters billing,search --output table

Adapter  Old  New  File
billing  0    1    env/dev/parameters.properties
search   1    0    env/dev/parameters.properties
```

### Expected File Structure in your repository for this feature to work

For the `flip-adapters` command, your repository should have this structure:

```
your-repo/
├── env/
│   ├── dev/
│   │   └── parameters.properties
│   ├── staging/
│   │   └── parameters.properties
│   └── production/
│       └── parameters.properties
```

Where `parameters.properties` contains adapter configurations:
```properties
# Adapter configurations
billing.adapter=0
search.adapter=1  
payment.adapter=1
crm.adapter=0
```

## Troubleshooting

### Authentication Issues
```bash
# Check GitHub CLI authentication
gh auth status

# Re-authenticate if needed
gh auth login
```

### Repository Access
```bash
# Verify you can access the repository
gh repo view owner/repo

# For private repos, ensure you have read access
gh repo clone owner/repo --depth=1
```

### Common Errors

**Error: `repo ORG/REPO is required`**
- Solution: Always specify the `--repo` flag with format `owner/repository`

**Error: `env is required`**  
- Solution: Specify the environment directory with `--env` (e.g., `--env dev`)

**Error: `adapter "xyz" not found`**
- Solution: Check the adapter name exists in your `env/{ENV}/parameters.properties` file

**Error: `failed to execute command: timeout`**
- Solution: Large repositories may timeout. Try scanning specific branches with `--ref`

## Advanced Examples

### Batch Processing Multiple Repos

```bash
# Create script to scan multiple repositories
#!/bin/bash
repos=("org/api-service" "org/web-app" "org/database")

for repo in "${repos[@]}"; do
  echo "Scanning $repo..."
  gh aca-utils ip-port --repo "$repo" --output csv >> all-configs.csv
done
```

### Integration with CI/CD

```yaml
# GitHub Actions workflow example
- name: Toggle staging adapters
  run: |
    gh aca-utils flip-adapters \
      --repo ${{ github.repository }} \
      --env staging \
      --adapters ${{ inputs.adapters }} \
      --commit \
      --pr
```

## System Requirements

- **Operating Systems**: Windows, macOS, Linux
- **GitHub CLI**: v2.0.0 or higher
- **Git**: Any recent version (for authentication)
- **Go**: Not required for users (only needed for development)

## Maintenance

### Upgrade
```bash
gh extension upgrade aca
```

### Uninstall
```bash
gh extension remove aca
```

### Configuration Files
- Stored adapters: `~/.gh-aca-utils/adapters.txt`
- Remove config directory: `rm -rf ~/.gh-aca-utils`

## Contributing

Found a bug or want to contribute? 
- **Report issues**: [GitHub Issues](https://github.com/greenstevester/gh-aca-utils/issues)
- **Feature requests**: [GitHub Discussions](https://github.com/greenstevester/gh-aca-utils/discussions)
- **Pull requests**: Welcome! Please read our contributing guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
