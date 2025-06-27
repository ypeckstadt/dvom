package models

import (
	"time"

	"github.com/docker/docker/api/types/container"
)

// BackupMetadata stores information about a backup
type BackupMetadata struct {
	ContainerName string            `json:"container_name"`
	ContainerID   string            `json:"container_id"`
	Volumes       []VolumeInfo      `json:"volumes"`
	CreatedAt     time.Time         `json:"created_at"`
	Config        *container.Config `json:"config,omitempty"`
	Version       string            `json:"version"`
}

// VolumeInfo stores volume details
type VolumeInfo struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Size        int64  `json:"size,omitempty"`
	Driver      string `json:"driver,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}
