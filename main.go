package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

const (
	// MaxCopySize limits decompression to prevent decompression bombs (100GB)
	MaxCopySize = 100 * 1024 * 1024 * 1024
)

// limitedCopy copies from src to dst with a size limit to prevent decompression bombs
func limitedCopy(dst io.Writer, src io.Reader, maxSize int64) (int64, error) {
	return io.CopyN(dst, src, maxSize)
}

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
}

// DockupClient wraps Docker client with backup functionality
type DockupClient struct {
	docker    *client.Client
	backupDir string
	verbose   bool
}

// NewDockupClient creates a new backup client
func NewDockupClient(backupDir string, verbose bool) (*DockupClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test Docker connection
	_, err = cli.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Docker daemon: %w", err)
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &DockupClient{
		docker:    cli,
		backupDir: backupDir,
		verbose:   verbose,
	}, nil
}

// GetContainer retrieves container information by name or ID
func (dc *DockupClient) GetContainer(name string) (*types.Container, error) {
	containers, err := dc.docker.ContainerList(context.Background(), container.ListOptions{All: true})
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
func (dc *DockupClient) GetContainerVolumes(containerID string) ([]VolumeInfo, error) {
	containerInfo, err := dc.docker.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var volumes []VolumeInfo
	for _, mount := range containerInfo.Mounts {
		if mount.Type == "volume" && mount.Name != "" {
			volumeInfo := VolumeInfo{
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
func (dc *DockupClient) IsContainerRunning(containerID string) (bool, error) {
	containerInfo, err := dc.docker.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return false, err
	}
	return containerInfo.State.Running, nil
}

// StopContainer stops a container and returns whether it was running
func (dc *DockupClient) StopContainer(containerID string) (bool, error) {
	wasRunning, err := dc.IsContainerRunning(containerID)
	if err != nil {
		return false, err
	}

	if wasRunning {
		timeout := 30 // seconds
		err = dc.docker.ContainerStop(context.Background(), containerID, container.StopOptions{
			Timeout: &timeout,
		})
		if err != nil {
			return wasRunning, fmt.Errorf("failed to stop container: %w", err)
		}
	}

	return wasRunning, nil
}

// StartContainer starts a container
func (dc *DockupClient) StartContainer(containerID string) error {
	err := dc.docker.ContainerStart(context.Background(), containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// BackupContainer creates a backup of the specified container
func (dc *DockupClient) BackupContainer(containerName, outputFile string, noStop bool) error {
	if dc.verbose {
		fmt.Printf("🔍 Analyzing container '%s'...\n", containerName)
	}

	// Get container info
	container, err := dc.GetContainer(containerName)
	if err != nil {
		return err
	}

	// Get volumes
	volumes, err := dc.GetContainerVolumes(container.ID)
	if err != nil {
		return err
	}

	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found for container '%s'", containerName)
	}

	if dc.verbose {
		fmt.Printf("📦 Found %d volume(s): ", len(volumes))
		for i, vol := range volumes {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(vol.Name)
		}
		fmt.Println()
	}

	// Generate output filename if not provided
	if outputFile == "" {
		timestamp := time.Now().Format("2006-01-02-15-04-05")
		outputFile = filepath.Join(dc.backupDir, fmt.Sprintf("%s-%s.zip", containerName, timestamp))
	} else if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(dc.backupDir, outputFile)
	}

	// Stop container if needed
	var wasRunning bool
	if !noStop {
		if dc.verbose {
			fmt.Println("⏸️  Stopping container...")
		}

		wasRunning, err = dc.StopContainer(container.ID)
		if err != nil {
			return err
		}
	}

	// Create backup
	if dc.verbose {
		fmt.Println("💾 Creating backup...")
	}

	err = dc.createBackupZip(container, volumes, outputFile)

	// Restart container if it was running
	if wasRunning && !noStop {
		if dc.verbose {
			fmt.Println("▶️  Starting container...")
		}
		if startErr := dc.StartContainer(container.ID); startErr != nil {
			fmt.Printf("Warning: failed to restart container: %v\n", startErr)
		}
	}

	if err != nil {
		return err
	}

	if dc.verbose {
		if stat, statErr := os.Stat(outputFile); statErr == nil {
			size := float64(stat.Size()) / (1024 * 1024) // Convert to MB
			fmt.Printf("✅ Backup created: %s (%.1f MB)\n", filepath.Base(outputFile), size)
		} else {
			fmt.Printf("✅ Backup created: %s\n", filepath.Base(outputFile))
		}
	}

	return nil
}

// createBackupZip creates a zip file containing volume data and metadata
func (dc *DockupClient) createBackupZip(container *types.Container, volumes []VolumeInfo, outputFile string) error {
	zipFile, err := os.Create(outputFile) // #nosec G304 - controlled path for backup files
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() {
		if err := zipFile.Close(); err != nil {
			fmt.Printf("Warning: failed to close zip file: %v\n", err)
		}
	}()

	writer := zip.NewWriter(zipFile)
	defer func() {
		if err := writer.Close(); err != nil {
			fmt.Printf("Warning: failed to close zip writer: %v\n", err)
		}
	}()

	// Create metadata
	containerName := strings.TrimPrefix(container.Names[0], "/")
	metadata := BackupMetadata{
		ContainerName: containerName,
		ContainerID:   container.ID,
		Volumes:       volumes,
		CreatedAt:     time.Now(),
		Version:       "1.0",
	}

	// Add metadata to zip
	metadataWriter, err := writer.Create("metadata.json")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if _, err := metadataWriter.Write(metadataJSON); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Backup each volume
	for _, vol := range volumes {
		if dc.verbose {
			fmt.Printf("   ├─ Volume '%s'...", vol.Name)
		}

		err := dc.backupVolume(writer, vol)
		if err != nil {
			if dc.verbose {
				fmt.Println(" ❌")
			}
			return fmt.Errorf("failed to backup volume '%s': %w", vol.Name, err)
		}

		if dc.verbose {
			fmt.Println(" ✓")
		}
	}

	return nil
}

// backupVolume backs up a single volume using a temporary container
func (dc *DockupClient) backupVolume(writer *zip.Writer, vol VolumeInfo) error {
	// Create a temporary container to access the volume
	resp, err := dc.docker.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"tar", "--numeric-owner", "--no-xattrs", "--no-acls", "--format=pax", "czf", "/tmp/volume.tar.gz", "-C", vol.Destination, "."},
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:%s:ro", vol.Name, vol.Destination)},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to create backup container: %w", err)
	}

	defer func() {
		if err := dc.docker.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true}); err != nil {
			fmt.Printf("Warning: failed to remove container %s: %v\n", resp.ID, err)
		}
	}()

	// Start the container
	if err := dc.docker.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start backup container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := dc.docker.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("backup container error: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get logs to see what went wrong
			logs, _ := dc.docker.ContainerLogs(context.Background(), resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			if logs != nil {
				if err := logs.Close(); err != nil {
					fmt.Printf("Warning: failed to close logs: %v\n", err)
				}
			}
			return fmt.Errorf("backup container exited with code %d", status.StatusCode)
		}
	}

	// Copy the archive from the container
	reader, _, err := dc.docker.CopyFromContainer(context.Background(), resp.ID, "/tmp/volume.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to copy archive from container: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Printf("Warning: failed to close reader: %v\n", err)
		}
	}()

	// Extract the tar.gz from the tar stream and add to zip
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar stream: %w", err)
		}

		if header.Name == "volume.tar.gz" {
			volumeWriter, err := writer.Create(fmt.Sprintf("volumes/%s.tar.gz", vol.Name))
			if err != nil {
				return fmt.Errorf("failed to create volume entry in zip: %w", err)
			}

			_, err = limitedCopy(volumeWriter, tarReader, MaxCopySize)
			if err != nil {
				return fmt.Errorf("failed to copy volume data: %w", err)
			}
			break
		}
	}

	return nil
}

// RestoreBackup restores from a backup file to specified container
func (dc *DockupClient) RestoreBackup(containerName, backupFile string, dryRun, force bool) error {
	if dc.verbose {
		fmt.Printf("🔍 Analyzing backup file...\n")
	}

	metadata, err := dc.readBackupMetadata(backupFile)
	if err != nil {
		return err
	}

	if dc.verbose {
		fmt.Printf("📋 Backup contains:\n")
		fmt.Printf("   ├─ Container: %s\n", metadata.ContainerName)
		fmt.Printf("   ├─ Volumes: ")
		for i, vol := range metadata.Volumes {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s", vol.Name)
		}
		fmt.Println()
		fmt.Printf("   ├─ Created: %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	if dryRun {
		fmt.Println("🧪 Dry run - no changes would be made")
		fmt.Printf("Would restore %d volume(s) to container '%s'\n", len(metadata.Volumes), containerName)
		return nil
	}

	// Check if target container exists
	container, err := dc.GetContainer(containerName)
	if err != nil {
		return fmt.Errorf("target container '%s' not found: %w", containerName, err)
	}

	// Confirmation prompt
	if !force {
		fmt.Printf("⚠️  This will overwrite volumes in container '%s'. Continue? (y/N): ", containerName)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Restore cancelled")
			return nil
		}
	}

	// Stop container
	if dc.verbose {
		fmt.Println("⏸️  Stopping container...")
	}

	wasRunning, err := dc.StopContainer(container.ID)
	if err != nil {
		return err
	}

	// Restore volumes
	if dc.verbose {
		fmt.Println("📤 Restoring volumes...")
	}

	err = dc.restoreVolumes(backupFile, metadata.Volumes)

	// Start container if it was running
	if wasRunning {
		if dc.verbose {
			fmt.Println("▶️  Starting container...")
		}
		if startErr := dc.StartContainer(container.ID); startErr != nil {
			fmt.Printf("Warning: failed to start container: %v\n", startErr)
		}
	}

	if err != nil {
		return err
	}

	if dc.verbose {
		fmt.Println("✅ Restore completed successfully")
	}

	return nil
}

// readBackupMetadata reads metadata from backup file
func (dc *DockupClient) readBackupMetadata(backupFile string) (*BackupMetadata, error) {
	reader, err := zip.OpenReader(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Printf("Warning: failed to close reader: %v\n", err)
		}
	}()

	for _, file := range reader.File {
		if file.Name == "metadata.json" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open metadata file: %w", err)
			}
			defer func() {
				if err := rc.Close(); err != nil {
					fmt.Printf("Warning: failed to close reader: %v\n", err)
				}
			}()

			var metadata BackupMetadata
			decoder := json.NewDecoder(rc)
			if err := decoder.Decode(&metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}

			return &metadata, nil
		}
	}

	return nil, fmt.Errorf("metadata.json not found in backup file")
}

// restoreVolumes restores volumes from backup
func (dc *DockupClient) restoreVolumes(backupFile string, volumes []VolumeInfo) error {
	reader, err := zip.OpenReader(backupFile)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Printf("Warning: failed to close reader: %v\n", err)
		}
	}()

	for _, vol := range volumes {
		if dc.verbose {
			fmt.Printf("   ├─ Volume '%s'...", vol.Name)
		}

		err := dc.restoreVolume(reader, vol)
		if err != nil {
			if dc.verbose {
				fmt.Println(" ❌")
			}
			return fmt.Errorf("failed to restore volume '%s': %w", vol.Name, err)
		}

		if dc.verbose {
			fmt.Println(" ✓")
		}
	}

	return nil
}

// restoreVolume restores a single volume
func (dc *DockupClient) restoreVolume(reader *zip.ReadCloser, vol VolumeInfo) error {
	volumePath := fmt.Sprintf("volumes/%s.tar.gz", vol.Name)

	var volumeFile *zip.File
	for _, file := range reader.File {
		if file.Name == volumePath {
			volumeFile = file
			break
		}
	}

	if volumeFile == nil {
		return fmt.Errorf("volume data not found in backup: %s", volumePath)
	}

	rc, err := volumeFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open volume data: %w", err)
	}
	defer func() {
		if err := rc.Close(); err != nil {
			fmt.Printf("Warning: failed to close reader: %v\n", err)
		}
	}()

	// Create temporary container to restore volume
	resp, err := dc.docker.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"sh", "-c", "rm -rf " + vol.Destination + "/* && tar xzf /tmp/volume.tar.gz -C " + vol.Destination},
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:%s", vol.Name, vol.Destination)},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to create restore container: %w", err)
	}

	defer func() {
		if err := dc.docker.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true}); err != nil {
			fmt.Printf("Warning: failed to remove container %s: %v\n", resp.ID, err)
		}
	}()

	// Create a tar stream with the volume.tar.gz file
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer func() {
			if err := pipeWriter.Close(); err != nil {
				fmt.Printf("Warning: failed to close pipe writer: %v\n", err)
			}
		}()
		tarWriter := tar.NewWriter(pipeWriter)
		defer func() {
			if err := tarWriter.Close(); err != nil {
				fmt.Printf("Warning: failed to close tar writer: %v\n", err)
			}
		}()

		// Create tar header for the file
		size := volumeFile.UncompressedSize64
		var tarSize int64
		if size > math.MaxInt64 {
			fmt.Printf("Warning: file size too large, truncating to MaxInt64\n")
			tarSize = math.MaxInt64
		} else {
			tarSize = int64(size) // #nosec G115 - bounds check performed above
		}
		header := &tar.Header{
			Name: "volume.tar.gz",
			Mode: 0644,
			Size: tarSize,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return
		}

		// Copy the gzipped data
		if _, err := limitedCopy(tarWriter, rc, MaxCopySize); err != nil {
			fmt.Printf("Warning: failed to copy data: %v\n", err)
		}
	}()

	// Copy archive to container
	if err := dc.docker.CopyToContainer(context.Background(), resp.ID, "/tmp", pipeReader, types.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy archive to container: %w", err)
	}

	// Start container to extract
	if err := dc.docker.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start restore container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := dc.docker.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("restore container error: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("restore container exited with code %d", status.StatusCode)
		}
	}

	return nil
}

// ListBackups lists available backup files
func (dc *DockupClient) ListBackups() error {
	pattern := filepath.Join(dc.backupDir, "*.zip")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("No backups found in %s\n", dc.backupDir)
		return nil
	}

	fmt.Printf("Available backups in %s:\n", dc.backupDir)
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}

		// Try to read metadata for more info
		metadata, err := dc.readBackupMetadata(file)
		if err == nil {
			size := float64(stat.Size()) / (1024 * 1024)
			fmt.Printf("  %s\n", filepath.Base(file))
			fmt.Printf("    ├─ Container: %s\n", metadata.ContainerName)
			fmt.Printf("    ├─ Volumes: %d\n", len(metadata.Volumes))
			fmt.Printf("    ├─ Size: %.1f MB\n", size)
			fmt.Printf("    └─ Created: %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"))
		} else {
			size := float64(stat.Size()) / (1024 * 1024)
			fmt.Printf("  %s (%.1f MB, %s)\n",
				filepath.Base(file),
				size,
				stat.ModTime().Format("2006-01-02 15:04"))
		}
		fmt.Println()
	}

	return nil
}

// InfoBackup shows detailed information about a backup file
func (dc *DockupClient) InfoBackup(backupFile string) error {
	metadata, err := dc.readBackupMetadata(backupFile)
	if err != nil {
		return err
	}

	stat, err := os.Stat(backupFile)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fmt.Printf("Backup Information: %s\n", filepath.Base(backupFile))
	fmt.Printf("├─ Container: %s (%s)\n", metadata.ContainerName, metadata.ContainerID[:12])
	fmt.Printf("├─ Created: %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("├─ Size: %.1f MB\n", float64(stat.Size())/(1024*1024))
	fmt.Printf("├─ Version: %s\n", metadata.Version)
	fmt.Printf("└─ Volumes (%d):\n", len(metadata.Volumes))

	for i, vol := range metadata.Volumes {
		prefix := "   ├─"
		if i == len(metadata.Volumes)-1 {
			prefix = "   └─"
		}
		fmt.Printf("%s %s -> %s\n", prefix, vol.Name, vol.Destination)
	}

	return nil
}

// Global variables for CLI flags
var (
	backupDir  string
	verbose    bool
	quiet      bool
	noStop     bool
	outputFile string
	dryRun     bool
	force      bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "dockup",
		Short: "Docker volume backup and restore tool",
		Long:  "A simple tool for backing up and restoring Docker container volumes",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate that backup directory is accessible
			if _, err := os.Stat(backupDir); os.IsNotExist(err) {
				if err := os.MkdirAll(backupDir, 0750); err != nil {
					return fmt.Errorf("cannot create backup directory %s: %w", backupDir, err)
				}
			}
			return nil
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&backupDir, "backup-dir", "./backups", "Directory to store backups")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet output")

	// Backup command
	var backupCmd = &cobra.Command{
		Use:   "backup <container-name>",
		Short: "Create a backup of container volumes",
		Long:  "Create a backup of all volumes from the specified container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewDockupClient(backupDir, verbose && !quiet)
			if err != nil {
				return err
			}

			return client.BackupContainer(args[0], outputFile, noStop)
		},
	}

	backupCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path")
	backupCmd.Flags().BoolVar(&noStop, "no-stop", false, "Don't stop container during backup")

	// Restore command
	var restoreCmd = &cobra.Command{
		Use:   "restore <container-name> <backup-file>",
		Short: "Restore volumes to a container from backup",
		Long:  "Restore volumes from a backup file to the specified container",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewDockupClient(backupDir, verbose && !quiet)
			if err != nil {
				return err
			}

			containerName := args[0]
			backupFile := args[1]

			// If backup file is not absolute, try to find it in backup directory
			if !filepath.IsAbs(backupFile) {
				candidatePath := filepath.Join(backupDir, backupFile)
				if _, err := os.Stat(candidatePath); err == nil {
					backupFile = candidatePath
				}
			}

			return client.RestoreBackup(containerName, backupFile, dryRun, force)
		},
	}

	restoreCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be restored without making changes")
	restoreCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")

	// List command
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List available backups",
		Long:  "List all backup files in the backup directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewDockupClient(backupDir, verbose && !quiet)
			if err != nil {
				return err
			}

			return client.ListBackups()
		},
	}

	// Info command
	var infoCmd = &cobra.Command{
		Use:   "info <backup-file>",
		Short: "Show detailed information about a backup",
		Long:  "Display detailed information about a backup file including metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewDockupClient(backupDir, verbose && !quiet)
			if err != nil {
				return err
			}

			backupFile := args[0]
			if !filepath.IsAbs(backupFile) {
				candidatePath := filepath.Join(backupDir, backupFile)
				if _, err := os.Stat(candidatePath); err == nil {
					backupFile = candidatePath
				}
			}

			return client.InfoBackup(backupFile)
		},
	}

	rootCmd.AddCommand(backupCmd, restoreCmd, listCmd, infoCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
