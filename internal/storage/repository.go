package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Repository represents a structured storage repository
type Repository struct {
	backend Backend
	config  *Config
}

// Ensure Repository implements RepositoryBackend interface
var _ RepositoryBackend = (*Repository)(nil)

// RepositoryIndex tracks all containers and their backups
type RepositoryIndex struct {
	Version    string                       `json:"version"`
	CreatedAt  time.Time                    `json:"created_at"`
	UpdatedAt  time.Time                    `json:"updated_at"`
	Containers map[string]*ContainerHistory `json:"containers"`
}

// ContainerHistory tracks backup history for a container
type ContainerHistory struct {
	Name      string             `json:"name"`
	ID        string             `json:"id"`
	Backups   []*BackupReference `json:"backups"`
	LatestID  string             `json:"latest_id"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// BackupReference points to a backup with metadata
type BackupReference struct {
	ID          string            `json:"id"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	Size        int64             `json:"size"`
	VolumeCount int               `json:"volume_count"`
	Tags        map[string]string `json:"tags,omitempty"`
	Description string            `json:"description,omitempty"`
}

// NewRepository creates a repository-aware storage layer
func NewRepository(backend Backend, config *Config) (*Repository, error) {
	repo := &Repository{
		backend: backend,
		config:  config,
	}

	// Initialize repository if it doesn't exist
	if err := repo.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	return repo, nil
}

// initialize sets up the repository structure
func (r *Repository) initialize() error {
	ctx := context.Background()

	// Check if repository index exists
	exists, err := r.backend.Exists(ctx, ".dvom/index.json")
	if err != nil {
		return err
	}

	if !exists {
		// Create initial repository index
		index := &RepositoryIndex{
			Version:    "1.0",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Containers: make(map[string]*ContainerHistory),
		}

		if err := r.saveIndex(ctx, index); err != nil {
			return fmt.Errorf("failed to create repository index: %w", err)
		}
	}

	return nil
}

// StoreBackup stores a backup in repository structure
func (r *Repository) StoreBackup(ctx context.Context, backup *Backup, tags map[string]string, description string) error {
	// Load current index
	index, err := r.loadIndex(ctx)
	if err != nil {
		return fmt.Errorf("failed to load repository index: %w", err)
	}

	containerName := backup.Metadata.Name
	if containerName == "" {
		return fmt.Errorf("container name is required for repository storage")
	}

	// Get or create container history
	containerHistory := index.Containers[containerName]
	if containerHistory == nil {
		containerHistory = &ContainerHistory{
			Name:      containerName,
			ID:        backup.Metadata.ContainerID,
			Backups:   []*BackupReference{},
			CreatedAt: time.Now(),
		}
		index.Containers[containerName] = containerHistory
	}

	// Generate version number
	version := len(containerHistory.Backups) + 1

	// Create backup path: containers/{name}/v{version}/
	backupPath := fmt.Sprintf("containers/%s/v%d/%s", containerName, version, backup.ID)

	// Update backup metadata with repository info
	backup.Metadata.ID = backup.ID
	backup.Metadata.CreatedAt = time.Now()

	// Store the actual backup data
	repoBackup := &Backup{
		ID:         backupPath,
		Metadata:   backup.Metadata,
		DataReader: backup.DataReader,
	}

	if err := r.backend.Store(ctx, repoBackup); err != nil {
		return fmt.Errorf("failed to store backup: %w", err)
	}

	// Add backup reference to container history
	backupRef := &BackupReference{
		ID:          backup.ID,
		Version:     version,
		CreatedAt:   backup.Metadata.CreatedAt,
		Size:        backup.Metadata.Size,
		VolumeCount: len(strings.Split(backup.Metadata.VolumeName, ",")), // Approximate
		Tags:        tags,
		Description: description,
	}

	containerHistory.Backups = append(containerHistory.Backups, backupRef)
	containerHistory.LatestID = backup.ID
	containerHistory.UpdatedAt = time.Now()

	// Update repository index
	index.UpdatedAt = time.Now()
	if err := r.saveIndex(ctx, index); err != nil {
		return fmt.Errorf("failed to update repository index: %w", err)
	}

	return nil
}

// GetBackup retrieves a backup by container name and version
func (r *Repository) GetBackup(ctx context.Context, containerName string, version int) (*Backup, error) {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return nil, err
	}

	containerHistory := index.Containers[containerName]
	if containerHistory == nil {
		return nil, fmt.Errorf("container '%s' not found in repository", containerName)
	}

	if version <= 0 || version > len(containerHistory.Backups) {
		return nil, fmt.Errorf("version %d not found for container '%s'", version, containerName)
	}

	backupRef := containerHistory.Backups[version-1]
	backupPath := fmt.Sprintf("containers/%s/v%d/%s", containerName, version, backupRef.ID)

	return r.backend.Retrieve(ctx, backupPath)
}

// GetLatestBackup retrieves the latest backup for a container
func (r *Repository) GetLatestBackup(ctx context.Context, containerName string) (*Backup, error) {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return nil, err
	}

	containerHistory := index.Containers[containerName]
	if containerHistory == nil {
		return nil, fmt.Errorf("container '%s' not found in repository", containerName)
	}

	if len(containerHistory.Backups) == 0 {
		return nil, fmt.Errorf("no backups found for container '%s'", containerName)
	}

	latestVersion := len(containerHistory.Backups)
	return r.GetBackup(ctx, containerName, latestVersion)
}

// ListContainers returns all containers in the repository
func (r *Repository) ListContainers(ctx context.Context) ([]string, error) {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return nil, err
	}

	containers := make([]string, 0, len(index.Containers))
	for name := range index.Containers {
		containers = append(containers, name)
	}

	sort.Strings(containers)
	return containers, nil
}

// ListBackups returns backup history for a container
func (r *Repository) ListBackups(ctx context.Context, containerName string) ([]*BackupReference, error) {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return nil, err
	}

	if containerName == "" {
		// Return all backups from all containers
		var allBackups []*BackupReference
		for _, containerHistory := range index.Containers {
			allBackups = append(allBackups, containerHistory.Backups...)
		}
		return allBackups, nil
	}

	containerHistory := index.Containers[containerName]
	if containerHistory == nil {
		return nil, fmt.Errorf("container '%s' not found in repository", containerName)
	}

	return containerHistory.Backups, nil
}

// GetContainerHistory returns the full history for a container
func (r *Repository) GetContainerHistory(ctx context.Context, containerName string) (*ContainerHistory, error) {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return nil, err
	}

	containerHistory := index.Containers[containerName]
	if containerHistory == nil {
		return nil, fmt.Errorf("container '%s' not found in repository", containerName)
	}

	return containerHistory, nil
}

// DeleteBackup removes a specific backup version
func (r *Repository) DeleteBackup(ctx context.Context, containerName string, version int) error {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return err
	}

	containerHistory := index.Containers[containerName]
	if containerHistory == nil {
		return fmt.Errorf("container '%s' not found in repository", containerName)
	}

	if version <= 0 || version > len(containerHistory.Backups) {
		return fmt.Errorf("version %d not found for container '%s'", version, containerName)
	}

	backupRef := containerHistory.Backups[version-1]
	backupPath := fmt.Sprintf("containers/%s/v%d/%s", containerName, version, backupRef.ID)

	// Delete from backend storage
	if err := r.backend.Delete(ctx, backupPath); err != nil {
		return fmt.Errorf("failed to delete backup from storage: %w", err)
	}

	// Remove from index (this will shift version numbers - might want to mark as deleted instead)
	containerHistory.Backups = append(
		containerHistory.Backups[:version-1],
		containerHistory.Backups[version:]...,
	)

	// Update latest if needed
	if len(containerHistory.Backups) > 0 {
		containerHistory.LatestID = containerHistory.Backups[len(containerHistory.Backups)-1].ID
	} else {
		containerHistory.LatestID = ""
	}

	containerHistory.UpdatedAt = time.Now()
	index.UpdatedAt = time.Now()

	return r.saveIndex(ctx, index)
}

// loadIndex loads the repository index
func (r *Repository) loadIndex(ctx context.Context) (*RepositoryIndex, error) {
	backup, err := r.backend.Retrieve(ctx, ".dvom/index.json")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve repository index: %w", err)
	}

	data, err := io.ReadAll(backup.DataReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read index data: %w", err)
	}

	// Close if the reader has a Close method
	if closer, ok := backup.DataReader.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			fmt.Printf("Warning: failed to close data reader: %v\n", err)
		}
	}

	var index RepositoryIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	return &index, nil
}

// saveIndex saves the repository index
func (r *Repository) saveIndex(ctx context.Context, index *RepositoryIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	indexBackup := &Backup{
		ID: ".dvom/index.json",
		Metadata: BackupMetadata{
			ID:        ".dvom/index.json",
			Name:      "repository-index",
			Type:      "index",
			Size:      int64(len(data)),
			CreatedAt: time.Now(),
		},
		DataReader: strings.NewReader(string(data)),
	}

	return r.backend.Store(ctx, indexBackup)
}

// GetRepositoryStats returns statistics about the repository
func (r *Repository) GetRepositoryStats(ctx context.Context) (*RepositoryStats, error) {
	index, err := r.loadIndex(ctx)
	if err != nil {
		return nil, err
	}

	stats := &RepositoryStats{
		Version:        index.Version,
		CreatedAt:      index.CreatedAt,
		UpdatedAt:      index.UpdatedAt,
		ContainerCount: len(index.Containers),
	}

	for _, containerHistory := range index.Containers {
		stats.BackupCount += len(containerHistory.Backups)
		for _, backup := range containerHistory.Backups {
			stats.TotalSize += backup.Size
		}
	}

	return stats, nil
}

// RepositoryStats contains repository statistics
type RepositoryStats struct {
	Version        string    `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ContainerCount int       `json:"container_count"`
	BackupCount    int       `json:"backup_count"`
	TotalSize      int64     `json:"total_size"`
}

// Backend interface delegation methods
func (r *Repository) Store(ctx context.Context, backup *Backup) error {
	return r.backend.Store(ctx, backup)
}

func (r *Repository) Retrieve(ctx context.Context, id string) (*Backup, error) {
	return r.backend.Retrieve(ctx, id)
}

func (r *Repository) List(ctx context.Context) ([]BackupMetadata, error) {
	return r.backend.List(ctx)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.backend.Delete(ctx, id)
}

func (r *Repository) Exists(ctx context.Context, id string) (bool, error) {
	return r.backend.Exists(ctx, id)
}
