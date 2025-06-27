# Encryption Guide

DVOM supports **optional AES-256-GCM encryption** for secure backup storage. This guide covers encryption features, usage, and best practices.

## 🔐 Encryption Features

- **AES-256-GCM Encryption** - Industry standard authenticated encryption
- **PBKDF2 Key Derivation** - 100,000 iterations with SHA-256
- **Salt & Nonce Generation** - Cryptographically secure random values
- **Backward Compatibility** - Works with both encrypted and non-encrypted backups
- **Automatic Detection** - Auto-detects encrypted backups during restore
- **Progress Tracking** - Progress bars work seamlessly with encryption

## 🚀 Basic Usage

### Creating Encrypted Backups

```bash
# Interactive password prompt (recommended)
dvom backup --volume=pgdata --name=secure-backup --encrypt

# With password flag (less secure)
dvom backup --volume=pgdata --name=secure-backup --encrypt --password=mypass123
```

### Restoring Encrypted Backups

```bash
# Interactive password prompt
dvom restore --snapshot=secure-backup --target-volume=pgdata --force

# With password flag
dvom restore --snapshot=secure-backup --target-volume=pgdata --password=mypass123 --force
```

### Viewing Encryption Status

```bash
# List all backups (shows ENCRYPTED column)
dvom list

# Get detailed backup information
dvom info secure-backup
```

## 🔒 Security Details

### Encryption Algorithm
- **Cipher**: AES-256-GCM (Galois/Counter Mode)
- **Key Size**: 256 bits
- **Authentication**: Built-in authentication with GCM
- **Nonce**: 96-bit unique nonce per encryption block

### Key Derivation
- **Algorithm**: PBKDF2 with SHA-256
- **Iterations**: 100,000 (recommended by OWASP)
- **Salt Size**: 256 bits (32 bytes)
- **Output**: 256-bit encryption key

### File Format
```
[Magic Header: "DVOM-ENC"] [Version: 1] [Salt: 32 bytes] [Nonce: 12 bytes] [Encrypted Data...]
```

## 📊 Example Output

### List Command with Encryption Status
```
Volume Backups:

BACKUP NAME                    LATEST VERSION       SIZE      VERSIONS  ENCRYPTED  VOLUME
------------------------------  --------------------  ----------  ----------  ----------  --------------------
secure-backup                  2024-06-27 14:30:25  45.2 MB   1         Yes        pgdata
regular-backup                 2024-06-27 14:25:10  42.1 MB   1         No         pgdata
```

### Info Command for Encrypted Backup
```
Snapshot: secure-backup
Created: 2024-06-27 14:30:25
Size: 45.2 MB
Type: direct-volume-backup
Encrypted: true
Volumes: 1
  - pgdata
```

## 🛡️ Best Practices

### Password Security
```bash
# ✅ Good: Interactive prompt (most secure)
dvom backup --volume=pgdata --name=secure --encrypt

# ✅ Good: Environment variable
export DVOM_PASSWORD="your-secure-password"
dvom backup --volume=pgdata --name=secure --encrypt

# ❌ Avoid: Command line password (visible in process list)
dvom backup --volume=pgdata --name=secure --encrypt --password=secret123
```

### Password Management
- Use strong, unique passwords (minimum 12 characters)
- Store passwords in secure password managers
- Consider key files for automated scenarios
- Use different passwords for different backup sets

### Operational Security
```bash
# Backup with encryption and container management
dvom backup --volume=pgdata --name=prod-backup \
  --encrypt \
  --stop-containers=postgres \
  --storage=s3 \
  --s3-bucket=secure-backups

# Verify backup encryption status
dvom info prod-backup

# Test restore to ensure password works
dvom restore --snapshot=prod-backup --target-volume=test-restore --dry-run
```

## 🔄 Migration Scenarios

### Encrypting Existing Backups
```bash
# Create encrypted copy of existing backup
dvom restore --snapshot=old-backup --target-volume=temp-volume --force
dvom backup --volume=temp-volume --name=old-backup-encrypted --encrypt
dvom delete old-backup --force
docker volume rm temp-volume
```

### Changing Encryption Password
```bash
# Restore with old password, backup with new password
dvom restore --snapshot=old-encrypted --target-volume=temp --password=oldpass --force
dvom backup --volume=temp --name=new-encrypted --encrypt --password=newpass
dvom delete old-encrypted --force
docker volume rm temp
```

## ⚠️ Important Notes

### Compatibility
- **Backward Compatible**: Non-encrypted backups continue to work normally
- **Forward Compatible**: Encrypted backups include version information
- **Mixed Mode**: Can have both encrypted and non-encrypted backups

### Password Recovery
- **No Password Recovery**: Lost passwords mean lost data
- **No Backdoors**: DVOM cannot recover encrypted data without the password
- **Test Restores**: Regularly verify passwords work correctly

### Performance Impact
- **Minimal Overhead**: Streaming encryption with negligible performance impact
- **Progress Tracking**: Real-time progress bars during encryption/decryption
- **Memory Efficient**: Uses constant memory regardless of backup size

## 🧪 Testing Encryption

```bash
# Create test volume with data
docker volume create test-encrypt
docker run --rm -v test-encrypt:/data alpine sh -c "echo 'secret data' > /data/test.txt"

# Create encrypted backup
dvom backup --volume=test-encrypt --name=test-backup --encrypt --password=testpass

# Verify encryption status
dvom list
dvom info test-backup

# Test restore
docker volume create test-restore
dvom restore --snapshot=test-backup --target-volume=test-restore --password=testpass --force

# Verify data integrity
docker run --rm -v test-restore:/data alpine cat /data/test.txt

# Cleanup
dvom delete test-backup --force
docker volume rm test-encrypt test-restore
```

## 🔧 Troubleshooting

### Common Issues

**Wrong Password Error**
```
Error: decryption failed: cipher: message authentication failed
```
- Verify password is correct
- Check for typos in password
- Ensure backup is actually encrypted

**Corruption Detection**
```
Error: backup marked as encrypted but no encryption header found
```
- Backup file may be corrupted
- Try restoring from a different version

**Performance Issues**
```bash
# Use quiet mode for better performance in scripts
dvom backup --volume=large-volume --name=big-backup --encrypt --quiet
```

## 📚 Advanced Usage

### Automated Backups with Encryption
```bash
#!/bin/bash
# automated-backup.sh

# Set password via environment (more secure than command line)
export DVOM_PASSWORD="$(cat /secure/backup-password.txt)"

# Create encrypted backup
dvom backup --volume=production-data --name="auto-backup-$(date +%Y%m%d)" \
  --encrypt \
  --storage=s3 \
  --s3-bucket=secure-backups \
  --quiet

# Cleanup old backups (keep last 7 days)
# Implementation depends on your retention policy
```

## 📖 Related Documentation

- [Configuration Guide](configuration.md) - Storage backend setup
- [Usage Guide](usage.md) - Complete command reference
- [Examples](examples.md) - Real-world scenarios
- [Architecture](architecture.md) - How encryption works internally