package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/ypeckstadt/dvom/internal/models"
)

// Client wraps Docker client with utility methods
type Client struct {
	docker *client.Client
}

// NewClient creates a new Docker client wrapper
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test Docker connection
	_, err = cli.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Docker daemon: %w", err)
	}

	return &Client{docker: cli}, nil
}

// GetContainer retrieves container information by name or ID
func (c *Client) GetContainer(name string) (*types.Container, error) {
	containers, err := c.docker.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		// Check by ID
		if container.ID == name || strings.HasPrefix(container.ID, name) {
			return &container, nil
		}
		// Check by name
		for _, containerName := range container.Names {
			cleanName := strings.TrimPrefix(containerName, "/")
			if cleanName == name {
				return &container, nil
			}
		}
	}

	return nil, fmt.Errorf("container '%s' not found", name)
}

// GetContainerVolumes retrieves volume information for a container
func (c *Client) GetContainerVolumes(containerID string) ([]models.VolumeInfo, error) {
	containerInfo, err := c.docker.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var volumes []models.VolumeInfo
	for _, mount := range containerInfo.Mounts {
		if mount.Type == "volume" && mount.Name != "" {
			volumeInfo := models.VolumeInfo{
				Name:        mount.Name,
				Source:      mount.Source,
				Destination: mount.Destination,
			}
			volumes = append(volumes, volumeInfo)
		}
	}

	return volumes, nil
}

// IsContainerRunning checks if a container is currently running
func (c *Client) IsContainerRunning(containerID string) (bool, error) {
	containerInfo, err := c.docker.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return false, err
	}
	return containerInfo.State.Running, nil
}

// StopContainer stops a container and returns whether it was running
func (c *Client) StopContainer(containerID string) (bool, error) {
	wasRunning, err := c.IsContainerRunning(containerID)
	if err != nil {
		return false, err
	}

	if wasRunning {
		timeout := 30 // seconds
		err = c.docker.ContainerStop(context.Background(), containerID, container.StopOptions{
			Timeout: &timeout,
		})
		if err != nil {
			return wasRunning, fmt.Errorf("failed to stop container: %w", err)
		}
	}

	return wasRunning, nil
}

// StartContainer starts a container
func (c *Client) StartContainer(containerID string) error {
	err := c.docker.ContainerStart(context.Background(), containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// GetDockerClient returns the underlying Docker client for advanced operations
func (c *Client) GetDockerClient() *client.Client {
	return c.docker
}

// ListVolumes returns all Docker volumes
func (c *Client) ListVolumes() ([]models.VolumeInfo, error) {
	volumeList, err := c.docker.VolumeList(context.Background(), volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var volumes []models.VolumeInfo
	for _, vol := range volumeList.Volumes {
		volumeInfo := models.VolumeInfo{
			Name:        vol.Name,
			Source:      vol.Mountpoint,
			Destination: vol.Mountpoint, // For direct volume access, source and destination are the same
			Driver:      vol.Driver,
			CreatedAt:   vol.CreatedAt,
		}
		volumes = append(volumes, volumeInfo)
	}

	return volumes, nil
}

// GetVolume retrieves information about a specific volume
func (c *Client) GetVolume(volumeName string) (*models.VolumeInfo, error) {
	vol, err := c.docker.VolumeInspect(context.Background(), volumeName)
	if err != nil {
		return nil, fmt.Errorf("volume '%s' not found: %w", volumeName, err)
	}

	volumeInfo := &models.VolumeInfo{
		Name:        vol.Name,
		Source:      vol.Mountpoint,
		Destination: vol.Mountpoint,
		Driver:      vol.Driver,
		CreatedAt:   vol.CreatedAt,
	}

	return volumeInfo, nil
}

// VolumeExists checks if a volume exists
func (c *Client) VolumeExists(volumeName string) (bool, error) {
	_, err := c.docker.VolumeInspect(context.Background(), volumeName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetContainersUsingVolume returns all containers that are using the specified volume
func (c *Client) GetContainersUsingVolume(volumeName string) ([]types.Container, error) {
	containers, err := c.docker.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var containersUsingVolume []types.Container
	for _, container := range containers {
		// Inspect container to get mount details
		containerInfo, err := c.docker.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			continue // Skip containers we can't inspect
		}

		// Check if this container uses the volume
		for _, mount := range containerInfo.Mounts {
			if mount.Type == "volume" && mount.Name == volumeName {
				containersUsingVolume = append(containersUsingVolume, container)
				break
			}
		}
	}

	return containersUsingVolume, nil
}
