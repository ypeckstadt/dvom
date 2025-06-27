# Installation Guide

## Build Requirements

- Go 1.23 or higher
- Make (optional, for using Makefile)
- Docker daemon access

## Installation Methods

### 1. Homebrew (macOS/Linux) - Recommended

```bash
# Add the custom tap
brew tap ypeckstadt/homebrew-tap

# Install dvom
brew install dvom

# Or in one command
brew install ypeckstadt/homebrew-tap/dvom
```

### 2. Pre-built Binaries

Download the latest release from the [releases page](https://github.com/ypeckstadt/dvom/releases).

Available for:
- Linux (amd64, arm64)
- macOS (amd64, arm64) 
- Windows (amd64)

### 3. Go Install

```bash
go install github.com/ypeckstadt/dvom/cmd/dvom@latest
```

### 4. Build from Source

```bash
# Clone the repository
git clone https://github.com/ypeckstadt/dvom.git
cd dvom

# Build the binary
make build

# Or build manually
go build -o bin/dvom ./cmd/dvom

# Install to GOPATH/bin
make install
```

### 5. Docker

```bash
# Pull the latest image
docker pull ghcr.io/ypeckstadt/dvom:latest

# Run a backup
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/backups:/backups \
  ghcr.io/ypeckstadt/dvom:latest \
  backup --volume=pgdata --name=backup-name
```

## Verification

Verify your installation:

```bash
dvom --version
dvom --help
```

## Next Steps

- [Configuration Guide](configuration.md) - Set up storage backends
- [Usage Guide](usage.md) - Learn the commands
- [Quick Examples](examples.md) - Start backing up volumes