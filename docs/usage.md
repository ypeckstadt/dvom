# Usage Guide

Complete command reference for DVOM (Docker Volume Manager).

## Commands Overview

| Command | Description |
|---------|-------------|
| `backup` | Create a backup of a volume |
| `restore` | Restore a volume backup |
| `list` | List available backups |
| `info` | Show detailed backup information |
| `versions` | List all versions of a backup |
| `delete` | Delete volume backups |
| `volumes` | List all Docker volumes |

## Global Flags

```bash
--storage string         Storage backend (local, gcs, s3) (default "local")
--backup-dir string      Local storage directory (default "./backups")
--verbose, -v           Verbose output
--quiet, -q             Quiet output (no progress bars)

# GCS flags
--gcs-bucket string      GCS bucket name
--gcs-project string     GCS project ID
--gcs-creds string       GCS credentials file path

# S3 flags  
--s3-bucket string       S3 bucket name
--s3-region string       S3 region (default "us-east-1")
--s3-endpoint string     S3 endpoint URL
--s3-access-key string   S3 access key
--s3-secret-key string   S3 secret key

# Encryption flags
--encrypt               Enable AES-256 encryption
--password string       Encryption/decryption password
```

## backup

Create a backup of a Docker volume.

### Syntax
```bash
dvom backup --volume=<volume-name> --name=<backup-name> [flags]
```

### Required Flags
```bash
--volume string          Volume name to backup
-n, --name string        Name for the volume backup
```

### Optional Flags
```bash
--stop-containers strings   Container names/IDs to stop during backup
--encrypt                   Encrypt the backup with AES-256
--password string           Password for encryption
```

### Examples
```bash
# Basic backup
dvom backup --volume=pgdata --name=prod-backup

# Backup with container management
dvom backup --volume=pgdata --name=db-backup --stop-containers=postgres

# Encrypted backup
dvom backup --volume=pgdata --name=secure-backup --encrypt

# Backup to cloud storage
dvom backup --volume=pgdata --name=cloud-backup \
  --storage=s3 --s3-bucket=my-backups

# Multiple containers stopped
dvom backup --volume=appdata --name=app-backup \
  --stop-containers=web-server,redis,worker
```

## restore

Restore a volume backup to a Docker volume.

### Syntax
```bash
dvom restore --snapshot=<backup-name> --target-volume=<volume-name> [flags]
```

### Required Flags
```bash
-s, --snapshot string        Name of the backup to restore
--target-volume string       Target volume name
```

### Optional Flags
```bash
--version string            Specific version to restore (YYYYMMDD-HHMMSS)
--password string           Password for decryption
--dry-run                   Show what would be restored
--force                     Skip confirmation prompts
--stop-containers strings   Container names/IDs to stop during restore
```

### Examples
```bash
# Basic restore
dvom restore --snapshot=prod-backup --target-volume=pgdata

# Restore specific version
dvom restore --snapshot=prod-backup --version=20240627-143052 \
  --target-volume=pgdata

# Restore encrypted backup
dvom restore --snapshot=secure-backup --target-volume=pgdata --password=secret

# Dry run to preview
dvom restore --snapshot=prod-backup --target-volume=pgdata --dry-run

# Force restore without confirmation
dvom restore --snapshot=prod-backup --target-volume=pgdata --force

# Restore with container management
dvom restore --snapshot=db-backup --target-volume=pgdata \
  --stop-containers=postgres --force
```

## list

List all available volume backups.

### Syntax
```bash
dvom list [flags]
```

### Output Format
```
BACKUP NAME                    LATEST VERSION       SIZE      VERSIONS  ENCRYPTED  VOLUME
------------------------------  --------------------  ----------  ----------  ----------  --------------------
prod-backup                    2024-06-27 14:30:25  45.2 MB   3         No         pgdata
secure-backup                  2024-06-27 14:25:10  42.1 MB   1         Yes        pgdata
```

### Examples
```bash
# List all backups
dvom list

# List backups in specific storage
dvom list --storage=gcs --gcs-bucket=my-backups

# Verbose listing
dvom list --verbose
```

## info

Show detailed information about a specific backup.

### Syntax
```bash
dvom info <backup-name> [flags]
```

### Examples
```bash
# Show backup details
dvom info prod-backup

# Show info from cloud storage
dvom info prod-backup --storage=s3 --s3-bucket=my-backups
```

### Output Example
```
Snapshot: prod-backup
Created: 2024-06-27 14:30:25
Size: 45.2 MB
Type: direct-volume-backup
Encrypted: false
Volumes: 1
  - pgdata
Description: Direct volume backup of pgdata
```

## versions

List all versions of a specific backup.

### Syntax
```bash
dvom versions <backup-name> [flags]
```

### Examples
```bash
# List all versions
dvom versions prod-backup

# List versions from cloud storage
dvom versions prod-backup --storage=gcs --gcs-bucket=my-backups
```

### Output Example
```
VERSION              CREATED              SIZE       DESCRIPTION
20240627-143052      2024-06-27 14:30:52  45.2 MB    Direct volume backup of pgdata
20240627-120000      2024-06-27 12:00:00  44.8 MB    Direct volume backup of pgdata
20240626-180000      2024-06-26 18:00:00  44.1 MB    Direct volume backup of pgdata
```

## delete

Delete volume backups by name or specific version.

### Syntax
```bash
dvom delete <backup-name> [flags]
```

### Optional Flags
```bash
--version string   Specific version to delete (YYYYMMDD-HHMMSS)
--force           Skip confirmation prompts
```

### Examples
```bash
# Delete all versions of a backup
dvom delete prod-backup

# Delete specific version
dvom delete prod-backup --version=20240627-143052

# Force delete without confirmation
dvom delete prod-backup --force

# Delete from cloud storage
dvom delete prod-backup --storage=s3 --s3-bucket=my-backups --force
```

## volumes

List all Docker volumes on the system.

### Syntax
```bash
dvom volumes [flags]
```

### Examples
```bash
# List all Docker volumes
dvom volumes

# Verbose volume listing
dvom volumes --verbose
```

### Output Example
```
Docker Volumes:

VOLUME NAME                    DRIVER          CREATED              MOUNTPOINT
------------------------------  ---------------  --------------------  --------------------
pgdata                         local           2024-06-27T10:30:25Z  /var/lib/docker/volumes/pgdata/_data
redis-data                     local           2024-06-27T09:15:10Z  /var/lib/docker/volumes/redis-data/_data
app-uploads                    local           2024-06-26T16:45:30Z  /var/lib/docker/volumes/app-uploads/_data
```

## Advanced Usage Patterns

### Automated Backup Scripts
```bash
#!/bin/bash
# daily-backup.sh

DATE=$(date +%Y%m%d)
BACKUP_NAME="daily-backup-$DATE"

# Backup with error handling
if dvom backup --volume=pgdata --name="$BACKUP_NAME" --encrypt --quiet; then
    echo "✅ Backup successful: $BACKUP_NAME"
    
    # Cleanup old backups (keep last 7 days)
    WEEK_AGO=$(date -d '7 days ago' +%Y%m%d)
    dvom delete "daily-backup-$WEEK_AGO" --force 2>/dev/null || true
else
    echo "❌ Backup failed"
    exit 1
fi
```

### Backup Rotation
```bash
# Keep different retention periods
dvom backup --volume=pgdata --name="hourly-$(date +%H)"     # 24 hours
dvom backup --volume=pgdata --name="daily-$(date +%u)"      # 7 days  
dvom backup --volume=pgdata --name="weekly-$(date +%U)"     # 52 weeks
dvom backup --volume=pgdata --name="monthly-$(date +%m)"    # 12 months
```

### Cross-Environment Restore
```bash
# Backup from production
dvom backup --volume=prod-data --name=prod-snapshot \
  --storage=s3 --s3-bucket=backups

# Restore to staging
dvom restore --snapshot=prod-snapshot --target-volume=staging-data \
  --storage=s3 --s3-bucket=backups --force
```

### Encrypted Cloud Backups
```bash
# Daily encrypted backup to cloud
dvom backup --volume=sensitive-data --name="encrypted-$(date +%Y%m%d)" \
  --encrypt \
  --storage=gcs \
  --gcs-bucket=secure-backups \
  --stop-containers=app,database \
  --quiet
```

## Error Handling

### Common Exit Codes
- `0` - Success
- `1` - General error
- `2` - Invalid arguments
- `3` - Docker connection error
- `4` - Storage backend error
- `5` - Encryption/decryption error

### Example Error Handling
```bash
#!/bin/bash
dvom backup --volume=pgdata --name=backup
case $? in
    0) echo "Backup successful" ;;
    1) echo "General error occurred" ;;
    3) echo "Docker connection failed" ;;
    4) echo "Storage backend error" ;;
    *) echo "Unknown error" ;;
esac
```

## Performance Tips

### For Large Volumes
```bash
# Use quiet mode to reduce overhead
dvom backup --volume=large-data --name=big-backup --quiet

# Use local storage for faster operations
dvom backup --volume=large-data --name=local-backup \
  --backup-dir=/fast-storage/backups
```

### For Automated Scripts
```bash
# Combine flags to minimize overhead
dvom backup --volume=auto-data --name="auto-$(date +%s)" \
  --encrypt --quiet --force \
  --storage=s3 --s3-bucket=automated-backups
```

## 📖 Related Documentation

- [Installation Guide](installation.md) - Setup and installation
- [Configuration Guide](configuration.md) - Storage backends
- [Encryption Guide](encryption.md) - Security features
- [Examples](examples.md) - Real-world scenarios