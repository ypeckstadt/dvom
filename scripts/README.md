# DVOM Release Scripts

This directory contains scripts to help with version management and releases.

## Scripts

### 🚀 release.sh

**Full-featured interactive release management script**

```bash
# Interactive release with all features
./scripts/release.sh

# Preview what would happen
./scripts/release.sh --dry-run

# Show help
./scripts/release.sh --help
```

**Features:**
- ✅ Shows current version and recent commits
- ✅ Interactive version type selection (patch/minor/major)
- ✅ Pre-release checks (build, lint)
- ✅ Custom release messages
- ✅ Git working directory validation
- ✅ Confirmation prompts
- ✅ Automatic tag creation and pushing
- ✅ Colored output and progress indicators

### ⚡ quick-release.sh

**Simple one-command release script**

```bash
# Quick patch release
./scripts/quick-release.sh patch

# Quick minor release  
./scripts/quick-release.sh minor

# Quick major release
./scripts/quick-release.sh major

# Show current version
./scripts/quick-release.sh
```

**Features:**
- ✅ Fast command-line version bumping
- ✅ Automatic tag creation and pushing
- ✅ No prompts or confirmations
- ✅ Perfect for CI/CD or quick releases

## Usage Examples

### Interactive Release (Recommended)

```bash
# Start interactive release process
./scripts/release.sh
```

**What it does:**
1. Shows current version (e.g., v0.2.3)
2. Displays recent commits since last tag
3. Shows **clear, detailed** version increment options:
   ```
   🚀 Select the type of release:
   
     1) 🐛 Patch Release → v0.2.4
        └─ Bug fixes, security patches, documentation updates
        └─ No new features or breaking changes
   
     2) ✨ Minor Release → v0.3.0  
        └─ New features, improvements, backward compatible changes
        └─ What you want for: new encryption, progress bars, etc.
   
     3) 💥 Major Release → v1.0.0
        └─ Breaking changes, major API changes
        └─ Incompatible with previous versions
   ```
4. Runs pre-release checks
5. Creates and pushes tag
6. Triggers GitHub Actions release workflow

### Quick Release

```bash
# Quick patch release (0.2.3 → 0.2.4)
./scripts/quick-release.sh patch

# Quick minor release (0.2.3 → 0.3.0)
./scripts/quick-release.sh minor

# Quick major release (0.2.3 → 1.0.0)
./scripts/quick-release.sh major
```

## Version Types

| Type | When to Use | Example |
|------|-------------|---------|
| **Patch** | Bug fixes, security patches, documentation | v1.2.3 → v1.2.4 |
| **Minor** | New features, backward compatible changes | v1.2.3 → v1.3.0 |
| **Major** | Breaking changes, major rewrites | v1.2.3 → v2.0.0 |

## Prerequisites

- Git repository with existing tags (or starts at v0.0.0)
- Write access to push tags
- Optional: Make available for pre-release checks

## Integration with GitHub Actions

Both scripts create git tags that trigger the GitHub Actions release workflow:

1. **Tag created** → GitHub Actions triggered
2. **Build binaries** for multiple platforms  
3. **Create GitHub release** with binaries
4. **Update Homebrew tap** automatically
5. **Publish Docker images** to registry

## Troubleshooting

### No existing tags
If no tags exist, scripts start from v0.0.0:
```bash
# First release will be v0.1.0 (minor) or v0.0.1 (patch)
./scripts/release.sh
```

### Working directory not clean
The full script checks for uncommitted changes:
```bash
# Commit or stash changes first
git add . && git commit -m "Prepare for release"
./scripts/release.sh
```

### Pre-release checks fail
```bash
# Fix issues first
make lint
make build

# Then try release again
./scripts/release.sh
```

### Tag already exists
```bash
# Delete local tag
git tag -d v1.2.3

# Delete remote tag  
git push origin :refs/tags/v1.2.3

# Try release again
```

## Customization

### Add custom checks to release.sh

Edit the `run_pre_release_checks()` function:

```bash
run_pre_release_checks() {
    # Add your custom checks here
    print_info "Running tests..."
    make test
    
    print_info "Running security scan..."
    make security
    
    # etc...
}
```

### Modify release message format

Edit the `get_release_message()` function to change default message format.

## Safety Features

### Full Script (release.sh)
- ✅ Working directory cleanliness check
- ✅ Multiple confirmation prompts
- ✅ Pre-release validation
- ✅ Dry-run mode for testing
- ✅ Graceful error handling

### Quick Script (quick-release.sh)
- ⚠️  Minimal safety checks
- ⚠️  No confirmation prompts
- ⚠️  Use with caution in production

## Examples in CI/CD

### GitHub Actions
```yaml
- name: Create Release
  run: |
    # Use quick script for automated releases
    ./scripts/quick-release.sh patch
```

### Manual Release Process
```bash
# 1. Finish feature work
git add . && git commit -m "Add encryption feature"

# 2. Run interactive release
./scripts/release.sh

# 3. Choose minor (new feature)
# 4. Let GitHub Actions handle the rest
```

Both scripts integrate seamlessly with the existing GitHub Actions workflow for automated binary building and release publishing!