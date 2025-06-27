package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SnapshotStorage provides volume-centric storage operations
type SnapshotStorage struct {
	backend Backend
}

// NewSnapshotStorage creates a volume-centric storage layer
func NewSnapshotStorage(backend Backend) *SnapshotStorage {
	return &SnapshotStorage{
		backend: backend,
	}
}

// StoreSnapshot stores a volume snapshot with automatic versioning
func (s *SnapshotStorage) StoreSnapshot(ctx context.Context, name string, backup *Backup) error {
	if name == "" {
		return fmt.Errorf("snapshot name is required")
	}

	// Clean snapshot name
	name = cleanSnapshotName(name)

	// Create versioned snapshot ID
	timestamp := time.Now().Format("20060102-150405")
	versionedID := fmt.Sprintf("%s@%s", name, timestamp)

	// Update metadata with snapshot info
	backup.Metadata.Name = name
	backup.Metadata.Type = "volume-snapshot"
	backup.Metadata.CreatedAt = time.Now()
	backup.Metadata.Version = timestamp

	// Store with versioned ID
	snapshotBackup := &Backup{
		ID:         versionedID,
		Metadata:   backup.Metadata,
		DataReader: backup.DataReader,
	}

	// Update metadata ID to match the versioned ID
	snapshotBackup.Metadata.ID = versionedID

	return s.backend.Store(ctx, snapshotBackup)
}

// GetSnapshot retrieves a volume snapshot by name (latest version) or name@version
func (s *SnapshotStorage) GetSnapshot(ctx context.Context, nameOrVersioned string) (*Backup, error) {
	nameOrVersioned = cleanSnapshotName(nameOrVersioned)

	// Check if version is specified (name@version format)
	if strings.Contains(nameOrVersioned, "@") {
		// Direct versioned lookup
		return s.backend.Retrieve(ctx, nameOrVersioned)
	}

	// Find latest version for this name
	latestVersion, err := s.GetLatestVersion(ctx, nameOrVersioned)
	if err != nil {
		return nil, err
	}

	versionedID := fmt.Sprintf("%s@%s", nameOrVersioned, latestVersion)
	return s.backend.Retrieve(ctx, versionedID)
}

// ListSnapshots returns all volume snapshots grouped by name with version info
func (s *SnapshotStorage) ListSnapshots(ctx context.Context) ([]SnapshotInfo, error) {
	backups, err := s.backend.List(ctx)
	if err != nil {
		return nil, err
	}

	// Group by snapshot name
	snapshotGroups := make(map[string][]BackupMetadata)
	for _, backup := range backups {
		// Only include volume snapshots with @ versioning
		if !strings.Contains(backup.ID, "@") {
			continue
		}

		// Extract name from versioned ID (name@version)
		parts := strings.SplitN(backup.ID, "@", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]

		snapshotGroups[name] = append(snapshotGroups[name], backup)
	}

	var snapshots []SnapshotInfo
	for name, versions := range snapshotGroups {
		// Sort versions by creation time (newest first)
		latestBackup := versions[0]
		for _, v := range versions {
			if v.CreatedAt.After(latestBackup.CreatedAt) {
				latestBackup = v
			}
		}

		snapshot := SnapshotInfo{
			Name:         name,
			Size:         latestBackup.Size,
			CreatedAt:    latestBackup.CreatedAt,
			Description:  latestBackup.Description,
			Version:      latestBackup.Version,
			VersionCount: len(versions),
			Encrypted:    latestBackup.Encrypted,
		}

		// Extract volume info if available
		if latestBackup.VolumeName != "" {
			snapshot.Volumes = strings.Split(latestBackup.VolumeName, ",")
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// DeleteSnapshot removes volume snapshots by name (all versions) or name@version (specific version)
func (s *SnapshotStorage) DeleteSnapshot(ctx context.Context, nameOrVersioned string) error {
	nameOrVersioned = cleanSnapshotName(nameOrVersioned)

	// Check if version is specified
	if strings.Contains(nameOrVersioned, "@") {
		// Delete specific version
		return s.backend.Delete(ctx, nameOrVersioned)
	}

	// Delete all versions of this snapshot name
	versions, err := s.ListVersions(ctx, nameOrVersioned)
	if err != nil {
		return fmt.Errorf("failed to list versions for deletion: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("no snapshots found with name '%s'", nameOrVersioned)
	}

	// Delete each version
	for _, version := range versions {
		versionedID := fmt.Sprintf("%s@%s", nameOrVersioned, version.Version)
		if err := s.backend.Delete(ctx, versionedID); err != nil {
			return fmt.Errorf("failed to delete version %s: %w", version.Version, err)
		}
	}

	return nil
}

// SnapshotExists checks if a snapshot exists
func (s *SnapshotStorage) SnapshotExists(ctx context.Context, nameOrVersioned string) (bool, error) {
	nameOrVersioned = cleanSnapshotName(nameOrVersioned)

	// Check if version is specified
	if strings.Contains(nameOrVersioned, "@") {
		// Check specific version
		return s.backend.Exists(ctx, nameOrVersioned)
	}

	// Check if any version exists for this name
	versions, err := s.ListVersions(ctx, nameOrVersioned)
	if err != nil {
		return false, err
	}

	return len(versions) > 0, nil
}

// SnapshotInfo contains information about a volume snapshot
type SnapshotInfo struct {
	Name            string    `json:"name"`
	Size            int64     `json:"size"`
	CreatedAt       time.Time `json:"created_at"`
	Description     string    `json:"description,omitempty"`
	Volumes         []string  `json:"volumes,omitempty"`
	SourceContainer string    `json:"source_container,omitempty"`
	Version         string    `json:"version,omitempty"`
	VersionCount    int       `json:"version_count,omitempty"`
	Encrypted       bool      `json:"encrypted,omitempty"`
}

// VersionInfo contains information about a specific version of a snapshot
type VersionInfo struct {
	Version     string    `json:"version"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

// ListVersions returns all versions of a snapshot name
func (s *SnapshotStorage) ListVersions(ctx context.Context, name string) ([]VersionInfo, error) {
	name = cleanSnapshotName(name)

	backups, err := s.backend.List(ctx)
	if err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for _, backup := range backups {
		// Check if this backup matches our snapshot name
		if strings.HasPrefix(backup.ID, name+"@") {
			parts := strings.SplitN(backup.ID, "@", 2)
			if len(parts) == 2 {
				versions = append(versions, VersionInfo{
					Version:     parts[1],
					Size:        backup.Size,
					CreatedAt:   backup.CreatedAt,
					Description: backup.Description,
				})
			}
		}
	}

	return versions, nil
}

// GetLatestVersion returns the version string of the latest snapshot
func (s *SnapshotStorage) GetLatestVersion(ctx context.Context, name string) (string, error) {
	versions, err := s.ListVersions(ctx, name)
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for snapshot '%s'", name)
	}

	// Find the latest version by creation time
	latestVersion := versions[0]
	for _, v := range versions {
		if v.CreatedAt.After(latestVersion.CreatedAt) {
			latestVersion = v
		}
	}

	return latestVersion.Version, nil
}

// cleanSnapshotName ensures snapshot names are valid for storage
func cleanSnapshotName(name string) string {
	// Remove file extensions if provided
	name = strings.TrimSuffix(name, ".tar.gz")
	name = strings.TrimSuffix(name, ".zip")

	// Replace problematic characters
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")

	return name
}
