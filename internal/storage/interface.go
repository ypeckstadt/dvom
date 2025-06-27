package storage

import (
	"context"
	"io"
	"time"
)

type Backup struct {
	ID         string
	Metadata   BackupMetadata
	DataReader io.Reader
}

type BackupMetadata struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	ContainerID string    `json:"container_id,omitempty"`
	VolumeName  string    `json:"volume_name,omitempty"`
	ImageName   string    `json:"image_name,omitempty"`
	ImageTag    string    `json:"image_tag,omitempty"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version,omitempty"`
	Encrypted   bool      `json:"encrypted,omitempty"`
}

type Backend interface {
	Store(ctx context.Context, backup *Backup) error
	Retrieve(ctx context.Context, id string) (*Backup, error)
	List(ctx context.Context) ([]BackupMetadata, error)
	Delete(ctx context.Context, id string) error
	Exists(ctx context.Context, id string) (bool, error)
}

// RepositoryBackend extends Backend with repository-aware operations
type RepositoryBackend interface {
	Backend

	// Repository operations
	StoreBackup(ctx context.Context, backup *Backup, tags map[string]string, description string) error
	GetBackup(ctx context.Context, containerName string, version int) (*Backup, error)
	GetLatestBackup(ctx context.Context, containerName string) (*Backup, error)
	ListContainers(ctx context.Context) ([]string, error)
	ListBackups(ctx context.Context, containerName string) ([]*BackupReference, error)
	GetContainerHistory(ctx context.Context, containerName string) (*ContainerHistory, error)
	DeleteBackup(ctx context.Context, containerName string, version int) error
	GetRepositoryStats(ctx context.Context) (*RepositoryStats, error)
}

type Config struct {
	Type  string
	Local *LocalConfig
	GCS   *GCSConfig
	S3    *S3Config
}

type LocalConfig struct {
	BasePath string
}

type GCSConfig struct {
	Bucket      string
	ProjectID   string
	Credentials string
}

type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
}
