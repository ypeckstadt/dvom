# Examples and Use Cases

Real-world examples of using DVOM for various backup scenarios.

## 🗄️ Database Backups

### PostgreSQL Database

```bash
# Stop database, backup volume, restart database
dvom backup --volume=pgdata --name="postgres-$(date +%Y%m%d)" \
  --stop-containers=postgres \
  --encrypt

# Restore for testing
docker volume create pgdata-test
dvom restore --snapshot=postgres-20240627 --target-volume=pgdata-test --force

# Start test instance
docker run -d --name postgres-test -v pgdata-test:/var/lib/postgresql/data postgres:13
```

### MySQL Database

```bash
# Backup MySQL data with proper shutdown
dvom backup --volume=mysql-data --name="mysql-backup-$(date +%Y%m%d-%H%M)" \
  --stop-containers=mysql-server \
  --storage=s3 \
  --s3-bucket=database-backups \
  --encrypt

# List all MySQL backups
dvom list --storage=s3 --s3-bucket=database-backups | grep mysql
```

### MongoDB Database

```bash
# Backup MongoDB with replica set coordination
dvom backup --volume=mongodb-data --name="mongo-$(date +%Y%m%d)" \
  --stop-containers=mongo-primary,mongo-secondary \
  --storage=gcs \
  --gcs-bucket=mongodb-backups
```

## 🌐 Web Application Backups

### WordPress Site

```bash
# Backup both database and uploads
dvom backup --volume=wp-database --name="wp-db-$(date +%Y%m%d)" \
  --stop-containers=wordpress-db --encrypt

dvom backup --volume=wp-uploads --name="wp-files-$(date +%Y%m%d)" \
  --storage=s3 --s3-bucket=wp-backups

# Full site restore
dvom restore --snapshot=wp-db-20240627 --target-volume=wp-database --force
dvom restore --snapshot=wp-files-20240627 --target-volume=wp-uploads \
  --storage=s3 --s3-bucket=wp-backups --force
```

### Node.js Application

```bash
# Backup application data and user uploads
dvom backup --volume=app-data --name="app-$(date +%Y%m%d-%H%M)" \
  --stop-containers=node-app,redis-cache \
  --storage=gcs \
  --gcs-bucket=app-backups \
  --encrypt
```

## 📊 Development Workflows

### Environment Cloning

```bash
# Backup production data
dvom backup --volume=prod-database --name=prod-clone \
  --storage=s3 --s3-bucket=shared-data \
  --encrypt --password="$SHARED_PASSWORD"

# Restore to development
docker volume create dev-database
dvom restore --snapshot=prod-clone --target-volume=dev-database \
  --storage=s3 --s3-bucket=shared-data \
  --password="$SHARED_PASSWORD" --force

# Start dev environment with production data
docker-compose -f docker-compose.dev.yml up -d
```

### CI/CD Integration

```bash
#!/bin/bash
# .github/workflows/backup.yml equivalent

# Pre-deployment backup
dvom backup --volume=app-data --name="pre-deploy-$(git rev-parse --short HEAD)" \
  --storage=s3 --s3-bucket=ci-backups \
  --quiet

# Deploy application
docker-compose up -d

# Health check - rollback if needed
if ! curl -f http://localhost:8080/health; then
    echo "Deployment failed, rolling back..."
    dvom restore --snapshot="pre-deploy-$(git rev-parse --short HEAD)" \
      --target-volume=app-data \
      --storage=s3 --s3-bucket=ci-backups --force
    docker-compose restart
fi
```

## 🔄 Migration Scenarios

### Docker Host Migration

```bash
# Source host: Backup all volumes
for volume in $(docker volume ls -q); do
    dvom backup --volume="$volume" --name="migration-$volume" \
      --storage=s3 --s3-bucket=migration-bucket \
      --encrypt --password="$MIGRATION_PASSWORD"
done

# Target host: Restore all volumes
dvom list --storage=s3 --s3-bucket=migration-bucket | grep migration- | \
while read backup rest; do
    volume_name=${backup#migration-}
    docker volume create "$volume_name"
    dvom restore --snapshot="$backup" --target-volume="$volume_name" \
      --storage=s3 --s3-bucket=migration-bucket \
      --password="$MIGRATION_PASSWORD" --force
done
```

### Cloud Provider Migration

```bash
# Migrate from AWS to GCP
# Step 1: Backup to S3
dvom backup --volume=app-data --name=cloud-migration \
  --storage=s3 --s3-bucket=aws-migration \
  --encrypt

# Step 2: Download backup locally
mkdir -p ./migration-backups
dvom restore --snapshot=cloud-migration --target-volume=temp-volume \
  --storage=s3 --s3-bucket=aws-migration --force

# Step 3: Re-upload to GCS
dvom backup --volume=temp-volume --name=cloud-migration \
  --storage=gcs --gcs-bucket=gcp-migration \
  --encrypt

# Step 4: Restore on new GCP instance
dvom restore --snapshot=cloud-migration --target-volume=app-data \
  --storage=gcs --gcs-bucket=gcp-migration --force
```

## 📅 Automated Backup Strategies

### Hourly Backups with Retention

```bash
#!/bin/bash
# hourly-backup.sh

VOLUME="critical-data"
HOUR=$(date +%H)
BACKUP_NAME="hourly-$HOUR"

# Create hourly backup (overwrites previous hour)
dvom backup --volume="$VOLUME" --name="$BACKUP_NAME" \
  --encrypt --quiet

echo "✅ Hourly backup completed: $BACKUP_NAME"
```

### Daily Backups with Weekly Retention

```bash
#!/bin/bash
# daily-backup.sh

VOLUME="app-database"
DATE=$(date +%Y%m%d)
DAY_OF_WEEK=$(date +%u)  # 1-7 (Monday-Sunday)
BACKUP_NAME="daily-$DATE"

# Daily backup
dvom backup --volume="$VOLUME" --name="$BACKUP_NAME" \
  --storage=gcs --gcs-bucket=daily-backups \
  --encrypt --quiet

# Keep weekly snapshots (Sunday backups)
if [ "$DAY_OF_WEEK" -eq 7 ]; then
    WEEK_NUM=$(date +%U)
    dvom backup --volume="$VOLUME" --name="weekly-$WEEK_NUM" \
      --storage=gcs --gcs-bucket=weekly-backups \
      --encrypt --quiet
fi

# Cleanup old daily backups (keep 7 days)
OLD_DATE=$(date -d '7 days ago' +%Y%m%d)
dvom delete "daily-$OLD_DATE" \
  --storage=gcs --gcs-bucket=daily-backups --force 2>/dev/null || true
```

### Multi-Tier Backup Strategy

```bash
#!/bin/bash
# comprehensive-backup.sh

VOLUMES=("database" "uploads" "config")
TIMESTAMP=$(date +%Y%m%d-%H%M)

for volume in "${VOLUMES[@]}"; do
    # Hot backup (frequent, short retention)
    dvom backup --volume="$volume" --name="hot-$volume-$(date +%H%M)" \
      --backup-dir="/fast-storage/hot-backups" --quiet
    
    # Daily backup (encrypted, cloud storage)
    if [ "$(date +%H)" -eq 2 ]; then  # 2 AM daily
        dvom backup --volume="$volume" --name="daily-$volume-$(date +%Y%m%d)" \
          --storage=s3 --s3-bucket=daily-backups \
          --encrypt --quiet
    fi
    
    # Weekly backup (long-term retention)
    if [ "$(date +%u)" -eq 7 ] && [ "$(date +%H)" -eq 3 ]; then  # Sunday 3 AM
        dvom backup --volume="$volume" --name="weekly-$volume-$(date +%Y-%U)" \
          --storage=gcs --gcs-bucket=weekly-backups \
          --encrypt --quiet
    fi
done
```

## 🧪 Testing and Validation

### Backup Integrity Testing

```bash
#!/bin/bash
# test-backup-integrity.sh

BACKUP_NAME="$1"
TEST_VOLUME="test-restore-$(date +%s)"

echo "🧪 Testing backup integrity: $BACKUP_NAME"

# Create test volume and restore
docker volume create "$TEST_VOLUME"
if dvom restore --snapshot="$BACKUP_NAME" --target-volume="$TEST_VOLUME" --force --quiet; then
    echo "✅ Backup restore successful"
    
    # Additional integrity checks could go here
    # e.g., database consistency checks, file verification, etc.
    
    # Cleanup
    docker volume rm "$TEST_VOLUME"
    echo "✅ Backup integrity test passed"
else
    echo "❌ Backup restore failed"
    docker volume rm "$TEST_VOLUME" 2>/dev/null || true
    exit 1
fi
```

### Disaster Recovery Drill

```bash
#!/bin/bash
# disaster-recovery-drill.sh

echo "🚨 Starting disaster recovery drill..."

# Simulate disaster by stopping all services
docker-compose down

# List available backups
echo "📋 Available backups:"
dvom list --storage=s3 --s3-bucket=disaster-recovery

# Restore from latest backup
LATEST_BACKUP=$(dvom list --storage=s3 --s3-bucket=disaster-recovery | \
                grep -v "BACKUP NAME" | head -1 | awk '{print $1}')

echo "🔄 Restoring from: $LATEST_BACKUP"
dvom restore --snapshot="$LATEST_BACKUP" --target-volume=app-data \
  --storage=s3 --s3-bucket=disaster-recovery --force

# Restart services
docker-compose up -d

# Verify service health
sleep 30
if curl -f http://localhost:8080/health; then
    echo "✅ Disaster recovery drill successful"
else
    echo "❌ Disaster recovery drill failed"
    exit 1
fi
```

## 🔧 Troubleshooting Scenarios

### Large Volume Backup

```bash
# For very large volumes, use local storage first, then sync
dvom backup --volume=huge-dataset --name=large-backup \
  --backup-dir="/fast-local/backups" \
  --quiet

# Then sync to cloud storage manually
aws s3 cp "/fast-local/backups/large-backup@$(date +%Y%m%d-%H%M%S).tar.gz" \
  s3://archive-bucket/
```

### Network-Interrupted Restore

```bash
# Use local staging for unreliable networks
dvom restore --snapshot=cloud-backup --target-volume=staging-volume \
  --storage=s3 --s3-bucket=remote-backups --force

# Verify restore completed successfully
if docker run --rm -v staging-volume:/data alpine test -f /data/expected-file; then
    # Copy to final destination
    docker run --rm -v staging-volume:/src -v final-volume:/dst alpine \
      cp -r /src/. /dst/
    docker volume rm staging-volume
else
    echo "Restore verification failed"
    exit 1
fi
```

### Cross-Platform Backup

```bash
# Backup from Linux host
dvom backup --volume=shared-data --name=cross-platform \
  --storage=s3 --s3-bucket=cross-platform-backups \
  --encrypt

# Restore on Windows host (using WSL2)
dvom restore --snapshot=cross-platform --target-volume=shared-data \
  --storage=s3 --s3-bucket=cross-platform-backups \
  --password="$SHARED_PASSWORD" --force
```

## 📊 Monitoring and Alerting

### Backup Success Monitoring

```bash
#!/bin/bash
# backup-with-monitoring.sh

VOLUME="$1"
BACKUP_NAME="$2"
WEBHOOK_URL="$3"

# Perform backup
if dvom backup --volume="$VOLUME" --name="$BACKUP_NAME" --encrypt --quiet; then
    # Success notification
    curl -X POST "$WEBHOOK_URL" -d "{\"status\":\"success\",\"backup\":\"$BACKUP_NAME\"}"
    echo "✅ Backup successful: $BACKUP_NAME"
else
    # Failure notification
    curl -X POST "$WEBHOOK_URL" -d "{\"status\":\"failure\",\"backup\":\"$BACKUP_NAME\"}"
    echo "❌ Backup failed: $BACKUP_NAME"
    exit 1
fi
```

### Backup Size Monitoring

```bash
#!/bin/bash
# monitor-backup-sizes.sh

# Check backup sizes and alert if significant change
dvom list --storage=s3 --s3-bucket=monitored-backups | \
while read name version size rest; do
    if [ "$name" != "BACKUP" ]; then  # Skip header
        size_mb=$(echo "$size" | sed 's/[^0-9.]//g')
        if (( $(echo "$size_mb > 1000" | bc -l) )); then
            echo "⚠️  Large backup detected: $name ($size)"
        fi
    fi
done
```

## 📚 Integration Examples

### Docker Compose Integration

```yaml
# docker-compose.backup.yml
version: '3.8'
services:
  backup:
    image: ghcr.io/ypeckstadt/dvom:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - app-data:/source-data:ro
    environment:
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - BACKUP_PASSWORD=${BACKUP_PASSWORD}
    command: >
      backup --volume=app-data --name=scheduled-backup
      --storage=s3 --s3-bucket=app-backups
      --encrypt --password=${BACKUP_PASSWORD}
      --quiet

volumes:
  app-data:
    external: true
```

### Kubernetes CronJob

```yaml
# backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: volume-backup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: dvom
            image: ghcr.io/ypeckstadt/dvom:latest
            command:
            - dvom
            - backup
            - --volume=persistent-data
            - --name=k8s-backup-$(date +%Y%m%d)
            - --storage=gcs
            - --gcs-bucket=k8s-backups
            - --encrypt
            - --quiet
            env:
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: /credentials/gcs-key.json
            - name: BACKUP_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: backup-secrets
                  key: password
            volumeMounts:
            - name: docker-socket
              mountPath: /var/run/docker.sock
            - name: gcs-credentials
              mountPath: /credentials
            - name: data-volume
              mountPath: /data
          volumes:
          - name: docker-socket
            hostPath:
              path: /var/run/docker.sock
          - name: gcs-credentials
            secret:
              secretName: gcs-credentials
          - name: data-volume
            persistentVolumeClaim:
              claimName: app-data-pvc
          restartPolicy: OnFailure
```

These examples cover common real-world scenarios and can be adapted to your specific needs. Remember to always test your backup and restore procedures in a non-production environment first!