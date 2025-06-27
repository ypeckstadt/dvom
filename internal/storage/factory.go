package storage

import (
	"context"
	"fmt"
)

func NewBackend(ctx context.Context, config *Config) (Backend, error) {
	switch config.Type {
	case "local":
		if config.Local == nil {
			return nil, fmt.Errorf("local configuration is required")
		}
		return NewLocalStorage(config.Local)

	case "gcs":
		if config.GCS == nil {
			return nil, fmt.Errorf("GCS configuration is required")
		}
		return NewGCSStorage(ctx, config.GCS)

	case "s3":
		if config.S3 == nil {
			return nil, fmt.Errorf("S3 configuration is required")
		}
		return NewS3Storage(ctx, config.S3)

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

// NewRepositoryBackend creates a repository-aware storage backend
func NewRepositoryBackend(ctx context.Context, config *Config) (RepositoryBackend, error) {
	backend, err := NewBackend(ctx, config)
	if err != nil {
		return nil, err
	}

	return NewRepository(backend, config)
}
