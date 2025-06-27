# DVOM - Docker Volume Manager

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

DVOM (Docker Volume Manager) is a powerful command-line tool for backing up and restoring individual Docker volumes with support for multiple storage backends, encryption, and progress tracking.

## ✨ Key Features

- **🔐 AES-256 Encryption** - Optional encryption for secure backups
- **📊 Progress Bars** - Real-time progress with speed and ETA
- **☁️ Multi-Storage** - Local, Google Cloud Storage, and AWS S3 support
- **🔄 Automatic Versioning** - Timestamp-based backup versions
- **🐳 Container Management** - Auto stop/restart containers during operations
- **📦 Individual Volumes** - Backup specific volumes, not entire containers

## 🚀 Quick Start

### Installation

#### Homebrew (macOS/Linux)
```bash
# Add the custom tap
brew tap ypeckstadt/homebrew-tap

# Install dvom
brew install dvom
```

Or in one command:
```bash
brew install ypeckstadt/homebrew-tap/dvom
```

#### Go Install
```bash
go install github.com/ypeckstadt/dvom/cmd/dvom@latest
```

#### Pre-built Binaries
Download from the [releases page](https://github.com/ypeckstadt/dvom/releases).

### Basic Usage

```bash
# Backup a volume
dvom backup --volume=pgdata --name=prod-backup

# Backup with encryption
dvom backup --volume=pgdata --name=secure-backup --encrypt

# List backups (shows encryption status)
dvom list

# Restore a backup
dvom restore --snapshot=prod-backup --target-volume=pgdata

# Restore encrypted backup
dvom restore --snapshot=secure-backup --target-volume=pgdata --password=secret

# View backup details
dvom info prod-backup
```

## 📖 Documentation

- **[Installation Guide](docs/installation.md)** - Detailed installation instructions
- **[Configuration](docs/configuration.md)** - Storage backends and settings
- **[Usage Guide](docs/usage.md)** - Complete command reference
- **[Encryption](docs/encryption.md)** - Security features and best practices
- **[Examples](docs/examples.md)** - Real-world usage scenarios
- **[Architecture](docs/architecture.md)** - How DVOM works internally

## 🔐 Security Features

- **AES-256-GCM Encryption** with PBKDF2 key derivation
- **Secure password prompting** with terminal input
- **Integrity verification** and tamper detection
- **Backward compatibility** with non-encrypted backups

## ☁️ Storage Backends

| Backend | Use Case | Features |
|---------|----------|----------|
| **Local** | Development, simple setups | Fast, no dependencies |
| **Google Cloud Storage** | GCP environments | Global access, lifecycle management |
| **AWS S3** | AWS environments | S3-compatible services supported |

## 🎯 Quick Examples

```bash
# Database backup with container management
dvom backup --volume=pgdata --name=db-backup --stop-containers=postgres

# Encrypted backup to cloud storage
dvom backup --volume=appdata --name=secure-backup \
  --encrypt --storage=s3 --s3-bucket=my-backups

# List all backups with encryption status
dvom list

# Restore with automatic decryption
dvom restore --snapshot=secure-backup --target-volume=appdata --force
```

## 🛠️ Development

```bash
# Clone and build
git clone https://github.com/ypeckstadt/dvom.git
cd dvom
make build

# Run tests
make test

# Run linter and security checks
make lint
make security
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🤝 Contributing

Contributions are welcome! Please read our [contributing guidelines](CONTRIBUTING.md) and submit pull requests to help improve DVOM.

---

**Need help?** Check out the [documentation](docs/) or open an [issue](https://github.com/ypeckstadt/dvom/issues).