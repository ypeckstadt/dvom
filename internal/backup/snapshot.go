package backup

import (
	"fmt"
	"strings"

	"github.com/ypeckstadt/dvom/internal/storage"
)

// ListSnapshots lists all volume snapshots in the repository
func (c *Client) ListSnapshots() error {
	if c.storage == nil {
		return fmt.Errorf("storage backend is required for snapshot operations")
	}

	snapshotStorage := storage.NewSnapshotStorage(c.storage)
	snapshots, err := snapshotStorage.ListSnapshots(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		fmt.Println("No snapshots found in repository")
		return nil
	}

	fmt.Printf("Volume Backups:\n\n")
	fmt.Printf("%-30s %-20s %-10s %-10s %-10s %s\n", "BACKUP NAME", "LATEST VERSION", "SIZE", "VERSIONS", "ENCRYPTED", "VOLUME")
	fmt.Printf("%-30s %-20s %-10s %-10s %-10s %s\n", strings.Repeat("-", 30), strings.Repeat("-", 20), strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 20))

	for _, snapshot := range snapshots {
		size := fmt.Sprintf("%.1f MB", float64(snapshot.Size)/(1024*1024))
		created := snapshot.CreatedAt.Format("2006-01-02 15:04:05")
		volumeName := "unknown"
		if len(snapshot.Volumes) > 0 {
			volumeName = snapshot.Volumes[0]
		}

		versionCount := fmt.Sprintf("%d", snapshot.VersionCount)
		encrypted := "No"
		if snapshot.Encrypted {
			encrypted = "Yes"
		}

		fmt.Printf("%-30s %-20s %-10s %-10s %-10s %s\n", snapshot.Name, created, size, versionCount, encrypted, volumeName)

		if c.verbose {
			if snapshot.Description != "" {
				fmt.Printf("  Description: %s\n", snapshot.Description)
			}
			if snapshot.Version != "" {
				fmt.Printf("  Latest Version: %s\n", snapshot.Version)
			}
		}
	}

	return nil
}

// GetSnapshotInfo displays detailed information about a snapshot
func (c *Client) GetSnapshotInfo(snapshotName string) error {
	if c.storage == nil {
		return fmt.Errorf("storage backend is required for snapshot operations")
	}

	snapshotStorage := storage.NewSnapshotStorage(c.storage)
	backup, err := snapshotStorage.GetSnapshot(c.ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to retrieve snapshot: %w", err)
	}
	defer func() {
		if closer, ok := backup.DataReader.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && c.verbose {
				fmt.Printf("Warning: failed to close backup data reader: %v\n", err)
			}
		}
	}()

	fmt.Printf("Snapshot: %s\n", backup.Metadata.Name)
	fmt.Printf("Created: %s\n", backup.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Size: %.1f MB\n", float64(backup.Metadata.Size)/(1024*1024))
	fmt.Printf("Type: %s\n", backup.Metadata.Type)
	fmt.Printf("Encrypted: %v\n", backup.Metadata.Encrypted)

	if backup.Metadata.VolumeName != "" {
		volumes := strings.Split(backup.Metadata.VolumeName, ",")
		fmt.Printf("Volumes: %d\n", len(volumes))
		for _, vol := range volumes {
			fmt.Printf("  - %s\n", vol)
		}
	}

	if backup.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", backup.Metadata.Description)
	}

	return nil
}

// ListSnapshotVersions displays all versions of a specific snapshot
func (c *Client) ListSnapshotVersions(snapshotName string) error {
	if c.storage == nil {
		return fmt.Errorf("storage backend is required for snapshot operations")
	}

	snapshotStorage := storage.NewSnapshotStorage(c.storage)
	versions, err := snapshotStorage.ListVersions(c.ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to list versions: %w", err)
	}

	if len(versions) == 0 {
		fmt.Printf("No versions found for snapshot '%s'\n", snapshotName)
		return nil
	}

	fmt.Printf("Versions for snapshot '%s':\n\n", snapshotName)
	fmt.Printf("%-20s %-20s %-10s %s\n", "VERSION", "CREATED", "SIZE", "DESCRIPTION")
	fmt.Printf("%-20s %-20s %-10s %s\n", strings.Repeat("-", 20), strings.Repeat("-", 20), strings.Repeat("-", 10), strings.Repeat("-", 20))

	for _, version := range versions {
		size := fmt.Sprintf("%.1f MB", float64(version.Size)/(1024*1024))
		created := version.CreatedAt.Format("2006-01-02 15:04:05")
		description := version.Description
		if description == "" {
			description = "-"
		}

		fmt.Printf("%-20s %-20s %-10s %s\n", version.Version, created, size, description)
	}

	return nil
}

// DeleteSnapshot deletes snapshots with confirmation
func (c *Client) DeleteSnapshot(nameOrVersioned string, force bool) error {
	if c.storage == nil {
		return fmt.Errorf("storage backend is required for snapshot operations")
	}

	snapshotStorage := storage.NewSnapshotStorage(c.storage)

	// Check if it's a versioned delete or full name delete
	isVersioned := strings.Contains(nameOrVersioned, "@")

	if !force {
		if isVersioned {
			fmt.Printf("⚠️  This will permanently delete the specific version: %s\n", nameOrVersioned)
		} else {
			// Show how many versions will be deleted
			versions, err := snapshotStorage.ListVersions(c.ctx, nameOrVersioned)
			if err != nil {
				return fmt.Errorf("failed to check versions: %w", err)
			}
			if len(versions) == 0 {
				return fmt.Errorf("no snapshots found with name '%s'", nameOrVersioned)
			}
			fmt.Printf("⚠️  This will permanently delete ALL %d version(s) of snapshot '%s'\n", len(versions), nameOrVersioned)
		}

		fmt.Print("Continue? (y/N): ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			// Treat as "N" if there's an error reading response
			response = "N"
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Delete cancelled")
			return nil
		}
	}

	if c.verbose {
		if isVersioned {
			fmt.Printf("🗑️  Deleting snapshot version: %s\n", nameOrVersioned)
		} else {
			fmt.Printf("🗑️  Deleting all versions of snapshot: %s\n", nameOrVersioned)
		}
	}

	if err := snapshotStorage.DeleteSnapshot(c.ctx, nameOrVersioned); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	if c.verbose {
		fmt.Printf("✅ Snapshot deleted successfully\n")
	}

	return nil
}
