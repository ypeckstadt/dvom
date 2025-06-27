package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/ypeckstadt/dvom/internal/crypto"
	"github.com/ypeckstadt/dvom/internal/models"
	"github.com/ypeckstadt/dvom/internal/storage"
	"golang.org/x/term"
)

// BackupDirectVolume backs up a volume directly by volume name (no container required)
func (c *Client) BackupDirectVolume(volumeName, snapshotName string) error {
	if c.storage == nil {
		return fmt.Errorf("storage backend is required for volume operations")
	}

	if c.verbose {
		fmt.Printf("📸 Creating volume backup '%s' from volume '%s'...\n",
			snapshotName, volumeName)
	}

	// Check if volume exists
	exists, err := c.docker.VolumeExists(volumeName)
	if err != nil {
		return fmt.Errorf("failed to check volume: %w", err)
	}
	if !exists {
		return fmt.Errorf("volume '%s' not found", volumeName)
	}

	// Get volume info
	volumeInfo, err := c.docker.GetVolume(volumeName)
	if err != nil {
		return err
	}

	if c.verbose {
		fmt.Printf("📦 Found volume: %s (driver: %s)\n", volumeInfo.Name, volumeInfo.Driver)
	}

	// Create temporary file for backup
	tempFile, err := os.CreateTemp("", "dvom-volume-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil && c.verbose {
			fmt.Printf("Warning: failed to remove temp file: %v\n", err)
		}
	}()
	defer func() {
		if err := tempFile.Close(); err != nil && c.verbose {
			fmt.Printf("Warning: failed to close temp file: %v\n", err)
		}
	}()

	// Backup the volume using a temporary container
	var spinner *IndeterminateProgress
	if !c.quiet {
		spinner = NewIndeterminateProgress("💾 Creating volume backup")
		defer spinner.Stop()
	} else if c.verbose {
		fmt.Println("💾 Creating volume backup...")
	}

	if err := c.backupDirectVolume(*volumeInfo, tempFile.Name()); err != nil {
		return err
	}

	if spinner != nil {
		spinner.Stop()
	}

	// Prepare for storage
	if _, err := tempFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek temp file: %w", err)
	}
	stat, err := tempFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %w", err)
	}

	// Handle encryption if enabled
	var finalReader io.Reader = tempFile
	encryptedSize := stat.Size()
	isEncrypted := false

	if c.encryptEnabled {
		// Get password if not provided
		password := c.password
		if password == "" {
			password = c.promptPassword("Enter encryption password: ", true)
			if password == "" {
				return fmt.Errorf("encryption password is required")
			}
		}

		// Create encrypted reader wrapper
		encryptReader, header, err := crypto.NewEncryptReader(tempFile, password)
		if err != nil {
			return fmt.Errorf("failed to create encryption: %w", err)
		}

		// Write encryption header to a buffer first
		var headerBuf bytes.Buffer
		if err := crypto.WriteEncryptionHeader(&headerBuf, header); err != nil {
			return fmt.Errorf("failed to write encryption header: %w", err)
		}

		// Combine header and encrypted data
		finalReader = io.MultiReader(&headerBuf, encryptReader)
		// Estimate encrypted size (header + data + overhead)
		encryptedSize = int64(headerBuf.Len()) + stat.Size() + (stat.Size()/64/1024)*16 // GCM overhead
		isEncrypted = true

		if c.verbose {
			fmt.Println("🔐 Encryption enabled")
		}
	}

	// Create progress reader for upload
	dataReader := finalReader
	var progressReader *ProgressReader
	if !c.quiet && encryptedSize > 0 {
		progressReader = NewProgressReader(finalReader, encryptedSize, "📤 Uploading backup")
		dataReader = progressReader
		defer func() {
			if err := progressReader.Close(); err != nil && c.verbose {
				fmt.Printf("Warning: failed to close progress reader: %v\n", err)
			}
		}()
	}

	// Create storage backup object
	backup := &storage.Backup{
		ID: snapshotName,
		Metadata: storage.BackupMetadata{
			Name:        snapshotName,
			Type:        "direct-volume-backup",
			Size:        encryptedSize,
			CreatedAt:   time.Now(),
			VolumeName:  volumeInfo.Name,
			Description: fmt.Sprintf("Direct volume backup of %s", volumeName),
			Encrypted:   isEncrypted,
		},
		DataReader: dataReader,
	}

	// Store the volume backup
	snapshotStorage := storage.NewSnapshotStorage(c.storage)
	if err := snapshotStorage.StoreSnapshot(c.ctx, snapshotName, backup); err != nil {
		return fmt.Errorf("failed to store volume backup: %w", err)
	}

	if progressReader != nil {
		if err := progressReader.Close(); err != nil && c.verbose {
			fmt.Printf("Warning: failed to close progress reader: %v\n", err)
		}
	}

	if c.verbose {
		fmt.Printf("✅ Volume backup created: %s (%.1f MB)\n", snapshotName, float64(stat.Size())/(1024*1024))
	}

	return nil
}

// BackupDirectVolumeWithContainers backs up a volume directly with optional container stop/start
func (c *Client) BackupDirectVolumeWithContainers(volumeName, snapshotName string, stopContainers []string) error {
	// Stop specified containers before backup
	stoppedContainers, err := c.stopContainers(stopContainers)
	if err != nil {
		return fmt.Errorf("failed to stop containers: %w", err)
	}

	// Ensure we restart containers even if backup fails
	defer func() {
		if err := c.restartContainers(stoppedContainers); err != nil && c.verbose {
			fmt.Printf("Warning: failed to restart some containers: %v\n", err)
		}
	}()

	// Perform the backup
	return c.BackupDirectVolume(volumeName, snapshotName)
}

// RestoreDirectVolume restores a volume backup directly to a volume (no container required)
func (c *Client) RestoreDirectVolume(volumeName, snapshotName string, dryRun, force bool) error {
	if c.storage == nil {
		return fmt.Errorf("storage backend is required for volume operations")
	}

	if c.verbose {
		fmt.Printf("🔄 Restoring volume backup '%s' to volume '%s'...\n",
			snapshotName, volumeName)
	}

	// Check if target volume exists
	exists, err := c.docker.VolumeExists(volumeName)
	if err != nil {
		return fmt.Errorf("failed to check target volume: %w", err)
	}
	if !exists {
		return fmt.Errorf("target volume '%s' not found", volumeName)
	}

	// Get target volume info
	volumeInfo, err := c.docker.GetVolume(volumeName)
	if err != nil {
		return err
	}

	// Retrieve volume backup
	snapshotStorage := storage.NewSnapshotStorage(c.storage)
	backup, err := snapshotStorage.GetSnapshot(c.ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to retrieve volume backup: %w", err)
	}
	defer func() {
		if closer, ok := backup.DataReader.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && c.verbose {
				fmt.Printf("Warning: failed to close backup data reader: %v\n", err)
			}
		}
	}()

	if c.verbose {
		fmt.Printf("📦 Volume backup info:\n")
		fmt.Printf("   Name: %s\n", backup.Metadata.Name)
		fmt.Printf("   Created: %s\n", backup.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Size: %.1f MB\n", float64(backup.Metadata.Size)/(1024*1024))
		fmt.Printf("   Original volume: %s\n", backup.Metadata.VolumeName)
		fmt.Printf("   Encrypted: %v\n", backup.Metadata.Encrypted)
	}

	if dryRun {
		fmt.Printf("\n🎯 Would restore to:\n")
		fmt.Printf("   Volume: %s (driver: %s)\n", volumeInfo.Name, volumeInfo.Driver)
		fmt.Println("\n✋ Dry run - no changes made")
		return nil
	}

	if !force {
		fmt.Printf("\n⚠️  This will completely overwrite the contents of volume '%s'\n", volumeName)
		fmt.Printf("⚠️  For best results, stop any containers using this volume first\n")
		fmt.Printf("⚠️  All existing data in the volume will be deleted and replaced\n")
		fmt.Print("Continue? (y/N): ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			// Treat as "N" if there's an error reading response
			response = "N"
		}
		if response != "y" && response != "Y" {
			fmt.Println("Restore cancelled")
			return nil
		}
	}

	// Create temp file for the backup data
	tempFile, err := os.CreateTemp("", "dvom-restore-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil && c.verbose {
			fmt.Printf("Warning: failed to remove temp file: %v\n", err)
		}
	}()
	defer func() {
		if err := tempFile.Close(); err != nil && c.verbose {
			fmt.Printf("Warning: failed to close temp file: %v\n", err)
		}
	}()

	// Handle decryption if the backup is encrypted
	finalReader := backup.DataReader
	
	if backup.Metadata.Encrypted {
		// Check if backup starts with encryption header
		headerBytes := make([]byte, 512) // Read first 512 bytes to check
		n, err := backup.DataReader.Read(headerBytes)
		if err != nil {
			return fmt.Errorf("failed to read backup header: %w", err)
		}
		
		if !crypto.IsEncrypted(headerBytes) {
			return fmt.Errorf("backup marked as encrypted but no encryption header found")
		}
		
		// Get password if not provided
		password := c.password
		if password == "" {
			password = c.promptPassword("Enter decryption password: ", false)
			if password == "" {
				return fmt.Errorf("decryption password is required")
			}
		}
		
		// Create reader from the header bytes and remaining data
		remainingReader := io.MultiReader(bytes.NewReader(headerBytes[:n]), backup.DataReader)
		
		// Read encryption header
		header, err := crypto.ReadEncryptionHeader(remainingReader)
		if err != nil {
			return fmt.Errorf("failed to read encryption header: %w", err)
		}
		
		// Create decryption reader
		decryptReader, err := crypto.NewDecryptReader(remainingReader, password, header)
		if err != nil {
			return fmt.Errorf("failed to create decryption: %w", err)
		}
		
		finalReader = decryptReader
		
		if c.verbose {
			fmt.Println("🔓 Decrypting backup...")
		}
	}

	// Copy backup data to temp file with progress
	var progressWriter *ProgressWriter
	var writer io.Writer = tempFile
	if !c.quiet && backup.Metadata.Size > 0 {
		progressWriter = NewProgressWriter(tempFile, backup.Metadata.Size, "📥 Downloading backup")
		writer = progressWriter
	}

	if _, err := io.Copy(writer, finalReader); err != nil {
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	if progressWriter != nil {
		if err := progressWriter.Close(); err != nil && c.verbose {
			fmt.Printf("Warning: failed to close progress writer: %v\n", err)
		}
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Restore the volume
	var spinner *IndeterminateProgress
	if !c.quiet {
		spinner = NewIndeterminateProgress("📥 Restoring volume data")
		defer spinner.Stop()
	} else if c.verbose {
		fmt.Println("📥 Restoring volume data...")
	}

	if err := c.restoreDirectVolume(*volumeInfo, tempFile.Name()); err != nil {
		return fmt.Errorf("failed to restore volume: %w", err)
	}

	if spinner != nil {
		spinner.Stop()
	}

	if c.verbose {
		fmt.Printf("✅ Volume restored successfully to %s\n", volumeInfo.Name)
	}

	return nil
}

// RestoreDirectVolumeWithContainers restores a volume backup with optional container stop/start
func (c *Client) RestoreDirectVolumeWithContainers(volumeName, snapshotName string, dryRun, force bool, stopContainers []string) error {
	// Stop specified containers before restore
	stoppedContainers, err := c.stopContainers(stopContainers)
	if err != nil {
		return fmt.Errorf("failed to stop containers: %w", err)
	}

	// Ensure we restart containers even if restore fails
	defer func() {
		if err := c.restartContainers(stoppedContainers); err != nil && c.verbose {
			fmt.Printf("Warning: failed to restart some containers: %v\n", err)
		}
	}()

	// Perform the restore
	return c.RestoreDirectVolume(volumeName, snapshotName, dryRun, force)
}

// stopContainers stops the specified containers and returns their IDs and running states
func (c *Client) stopContainers(containerNames []string) (map[string]bool, error) {
	if len(containerNames) == 0 {
		return make(map[string]bool), nil
	}

	stoppedContainers := make(map[string]bool)

	if c.verbose {
		fmt.Printf("🛑 Stopping %d container(s)...\n", len(containerNames))
	}

	for _, name := range containerNames {
		// Get container info
		container, err := c.docker.GetContainer(name)
		if err != nil {
			return stoppedContainers, fmt.Errorf("container '%s' not found: %w", name, err)
		}

		// Check if container is running and stop it
		wasRunning, err := c.docker.StopContainer(container.ID)
		if err != nil {
			return stoppedContainers, fmt.Errorf("failed to stop container '%s': %w", name, err)
		}

		stoppedContainers[container.ID] = wasRunning

		if c.verbose {
			if wasRunning {
				fmt.Printf("   ✅ Stopped: %s (%s)\n", name, container.ID[:12])
			} else {
				fmt.Printf("   ℹ️  Already stopped: %s (%s)\n", name, container.ID[:12])
			}
		}
	}

	return stoppedContainers, nil
}

// restartContainers restarts containers that were previously running
func (c *Client) restartContainers(stoppedContainers map[string]bool) error {
	if len(stoppedContainers) == 0 {
		return nil
	}

	var errors []string
	runningCount := 0
	for _, wasRunning := range stoppedContainers {
		if wasRunning {
			runningCount++
		}
	}

	if runningCount > 0 && c.verbose {
		fmt.Printf("🔄 Restarting %d container(s)...\n", runningCount)
	}

	for containerID, wasRunning := range stoppedContainers {
		if wasRunning {
			if err := c.docker.StartContainer(containerID); err != nil {
				errors = append(errors, fmt.Sprintf("failed to restart container %s: %v", containerID[:12], err))
				if c.verbose {
					fmt.Printf("   ❌ Failed to restart: %s\n", containerID[:12])
				}
			} else if c.verbose {
				fmt.Printf("   ✅ Restarted: %s\n", containerID[:12])
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("restart errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// backupDirectVolume backs up a volume using a temporary container
func (c *Client) backupDirectVolume(volume models.VolumeInfo, outputFile string) error {
	dockerClient := c.docker.GetDockerClient()

	// Create a temporary container to access the volume
	resp, err := dockerClient.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"tar", "czf", "/backup.tar.gz", "-C", "/data", "."},
		},
		&container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s:/data:ro", volume.Name),
			},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to create backup container: %w", err)
	}

	defer func() {
		if err := dockerClient.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true}); err != nil && c.verbose {
			fmt.Printf("Warning: failed to remove container %s: %v\n", resp.ID, err)
		}
	}()

	// Start the container
	if err := dockerClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start backup container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := dockerClient.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("backup container error: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get container logs for debugging
			logs, logErr := dockerClient.ContainerLogs(context.Background(), resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			if logErr == nil {
				defer func() {
					if err := logs.Close(); err != nil && c.verbose {
						fmt.Printf("Warning: failed to close logs: %v\n", err)
					}
				}()
				logData, _ := io.ReadAll(logs)
				if len(logData) > 0 {
					return fmt.Errorf("backup container failed with exit code %d. Logs: %s", status.StatusCode, string(logData))
				}
			}
			return fmt.Errorf("backup container exited with code %d", status.StatusCode)
		}
	}

	// Copy the backup file from container
	reader, _, err := dockerClient.CopyFromContainer(context.Background(), resp.ID, "/backup.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to copy backup from container: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil && c.verbose {
			fmt.Printf("Warning: failed to close reader: %v\n", err)
		}
	}()

	// Write to output file
	outFile, err := os.Create(outputFile) // #nosec G304 - controlled backup output path
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil && c.verbose {
			fmt.Printf("Warning: failed to close output file: %v\n", err)
		}
	}()

	// Extract from tar stream - CopyFromContainer wraps the file in a tar
	tarReader := tar.NewReader(reader)
	found := false
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar stream: %w", err)
		}

		// The file might be named just "backup.tar.gz" or have a path prefix
		if header.Name == "backup.tar.gz" || strings.HasSuffix(header.Name, "/backup.tar.gz") {
			// Limit copy size to prevent decompression bombs (100GB max)
			const maxBackupSize = 100 * 1024 * 1024 * 1024
			if _, err := io.CopyN(outFile, tarReader, maxBackupSize); err != nil && err != io.EOF {
				return fmt.Errorf("failed to copy backup data: %w", err)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("backup.tar.gz not found in tar stream")
	}

	return nil
}

// restoreDirectVolume restores a volume using a temporary container
func (c *Client) restoreDirectVolume(volume models.VolumeInfo, backupFile string) error {
	dockerClient := c.docker.GetDockerClient()

	// Read backup file
	backupData, err := os.ReadFile(backupFile) // #nosec G304 - controlled backup file path
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Create a temporary container with the backup file
	resp, err := dockerClient.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"sh", "-c", "rm -rf /data/* /data/.[^.]* && cd /data && tar xzf /backup.tar.gz"},
		},
		&container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s:/data", volume.Name),
			},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to create restore container: %w", err)
	}

	defer func() {
		if err := dockerClient.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true}); err != nil && c.verbose {
			fmt.Printf("Warning: failed to remove container %s: %v\n", resp.ID, err)
		}
	}()

	// Copy backup file to container
	if err := dockerClient.CopyToContainer(
		context.Background(),
		resp.ID,
		"/",
		createTarWithFile("backup.tar.gz", backupData),
		types.CopyToContainerOptions{},
	); err != nil {
		return fmt.Errorf("failed to copy backup to container: %w", err)
	}

	// Start the container
	if err := dockerClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start restore container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := dockerClient.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("restore container error: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get container logs for debugging
			logs, logErr := dockerClient.ContainerLogs(context.Background(), resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			if logErr == nil {
				defer func() {
					if err := logs.Close(); err != nil && c.verbose {
						fmt.Printf("Warning: failed to close logs: %v\n", err)
					}
				}()
				logData, _ := io.ReadAll(logs)
				if c.verbose && len(logData) > 0 {
					fmt.Printf("Container logs: %s\n", string(logData))
				}
			}
			return fmt.Errorf("restore container exited with code %d", status.StatusCode)
		}
	}

	if c.verbose {
		fmt.Println("🔍 Verifying restore completion...")
		// Get container logs for verification
		logs, logErr := dockerClient.ContainerLogs(context.Background(), resp.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		})
		if logErr == nil {
			defer func() {
				if err := logs.Close(); err != nil && c.verbose {
					fmt.Printf("Warning: failed to close logs: %v\n", err)
				}
			}()
			logData, _ := io.ReadAll(logs)
			if len(logData) > 0 {
				fmt.Printf("Restore output: %s\n", string(logData))
			}
		}
	}

	return nil
}

// ListDockerVolumes lists all Docker volumes
func (c *Client) ListDockerVolumes() error {
	volumes, err := c.docker.ListVolumes()
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	if len(volumes) == 0 {
		fmt.Println("No Docker volumes found")
		return nil
	}

	fmt.Printf("Docker Volumes:\n\n")
	fmt.Printf("%-30s %-15s %-20s %s\n", "VOLUME NAME", "DRIVER", "CREATED", "MOUNTPOINT")
	fmt.Printf("%-30s %-15s %-20s %s\n",
		"------------------------------",
		"---------------",
		"--------------------",
		"--------------------")

	for _, vol := range volumes {
		created := vol.CreatedAt
		if created == "" {
			created = "unknown"
		}

		fmt.Printf("%-30s %-15s %-20s %s\n", vol.Name, vol.Driver, created, vol.Source)
	}

	return nil
}

// createTarWithFile creates a tar archive containing a single file
func createTarWithFile(filename string, data []byte) io.Reader {
	buf := new(strings.Builder)
	tw := tar.NewWriter(buf)

	header := &tar.Header{
		Name: filename,
		Mode: 0600,
		Size: int64(len(data)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return strings.NewReader("")
	}
	if _, err := tw.Write(data); err != nil {
		return strings.NewReader("")
	}
	if err := tw.Close(); err != nil {
		return strings.NewReader("")
	}

	return strings.NewReader(buf.String())
}

// promptPassword prompts the user for a password
func (c *Client) promptPassword(prompt string, confirm bool) string {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after password input
	if err != nil {
		if c.verbose {
			fmt.Printf("Error reading password: %v\n", err)
		}
		return ""
	}

	password := string(bytePassword)
	
	if confirm {
		fmt.Print("Confirm password: ")
		byteConfirm, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			if c.verbose {
				fmt.Printf("Error reading password confirmation: %v\n", err)
			}
			return ""
		}
		
		if password != string(byteConfirm) {
			fmt.Println("❌ Passwords do not match")
			return ""
		}
	}
	
	return password
}
