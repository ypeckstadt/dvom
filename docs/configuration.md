# Configuration Guide

DVOM supports multiple storage backends and can be configured through command-line flags, environment variables, or configuration files.

## Storage Backends

### Local Storage (Default)

No additional configuration required. Backups are stored in the specified directory.

```bash
# Use default backup directory (./backups)
dvom backup --volume=pgdata --name=my-backup

# Use custom directory
dvom backup --volume=pgdata --name=my-backup --backup-dir=/my-backups
```

### Google Cloud Storage (GCS)

```bash
# Set up authentication via environment
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials.json"

# Backup to GCS
dvom backup --volume=pgdata --name=my-backup \
  --storage=gcs \
  --gcs-bucket=my-backups \
  --gcs-project=my-project-id

# Or use credentials file flag
dvom backup --volume=pgdata --name=my-backup \
  --storage=gcs \
  --gcs-bucket=my-backups \
  --gcs-creds=/path/to/creds.json
```

### AWS S3

```bash
# Using AWS credentials (recommended)
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-east-1"

# Backup to S3
dvom backup --volume=pgdata --name=my-backup \
  --storage=s3 \
  --s3-bucket=my-backups

# Or use flags
dvom backup --volume=pgdata --name=my-backup \
  --storage=s3 \
  --s3-bucket=my-backups \
  --s3-access-key=KEY \
  --s3-secret-key=SECRET \
  --s3-region=us-west-2
```

### S3-Compatible Services

DVOM works with MinIO, DigitalOcean Spaces, and other S3-compatible services:

```bash
# MinIO example
dvom backup --volume=pgdata --name=my-backup \
  --storage=s3 \
  --s3-bucket=my-backups \
  --s3-endpoint=http://minio.example.com:9000 \
  --s3-access-key=minioadmin \
  --s3-secret-key=minioadmin
```

## Global Flags

### Storage Configuration
```bash
--storage string         Storage backend type (local, gcs, s3) (default "local")
--backup-dir string      Directory for local storage (default "./backups")
```

### GCS Flags
```bash
--gcs-bucket string      GCS bucket name
--gcs-project string     GCS project ID  
--gcs-creds string       Path to GCS credentials file
```

### S3 Flags
```bash
--s3-bucket string       S3 bucket name
--s3-region string       S3 region (default "us-east-1")
--s3-endpoint string     S3 endpoint (for S3-compatible services)
--s3-access-key string   S3 access key
--s3-secret-key string   S3 secret key
```

### Output Control
```bash
--verbose, -v           Verbose output
--quiet, -q             Quiet output (no progress bars)
```

### Encryption
```bash
--encrypt               Encrypt the backup with AES-256
--password string       Password for encryption/decryption
```

## Environment Variables

You can set default values using environment variables:

```bash
export DVOM_STORAGE=s3
export DVOM_S3_BUCKET=my-default-bucket
export DVOM_S3_REGION=us-west-2
export DVOM_BACKUP_DIR=/default/backup/path
```

## Best Practices

### Security
- Never pass passwords via command line in production
- Use environment variables or interactive prompts for credentials
- Enable encryption for sensitive data
- Use IAM roles when possible (AWS/GCS)

### Performance
- Use `--quiet` flag in scripts to disable progress bars
- Choose storage regions close to your Docker host
- Consider network bandwidth for large volumes

### Organization
- Use consistent naming conventions for backups
- Implement retention policies for old backups
- Use separate buckets/directories for different environments

## Next Steps

- [Usage Guide](usage.md) - Learn the commands
- [Encryption Guide](encryption.md) - Secure your backups
- [Examples](examples.md) - Real-world scenarios