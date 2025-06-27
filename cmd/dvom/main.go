package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ypeckstadt/dvom/internal/backup"
	"github.com/ypeckstadt/dvom/internal/storage"
	"github.com/ypeckstadt/dvom/pkg/version"
)

// Global variables for CLI flags
var (
	backupDir    string
	verbose      bool
	quiet        bool
	dryRun       bool
	force        bool
	snapshotName string
	volumeName   string
	targetVolume string
	versionFlag  string
	// Storage flags
	storageType  string
	gcsBucket    string
	gcsProject   string
	gcsCredsFile string
	s3Bucket     string
	s3Region     string
	s3Endpoint   string
	s3AccessKey  string
	s3SecretKey  string
	// Container management flags
	stopContainers []string
	// Encryption flags
	encrypt  bool
	password string
)

func buildStorageConfig() (*storage.Config, error) {
	config := &storage.Config{
		Type: storageType,
	}

	switch storageType {
	case "local":
		config.Local = &storage.LocalConfig{
			BasePath: backupDir,
		}
	case "gcs":
		if gcsBucket == "" {
			return nil, fmt.Errorf("GCS bucket is required when using GCS storage")
		}
		config.GCS = &storage.GCSConfig{
			Bucket:      gcsBucket,
			ProjectID:   gcsProject,
			Credentials: gcsCredsFile,
		}
	case "s3":
		if s3Bucket == "" {
			return nil, fmt.Errorf("S3 bucket is required when using S3 storage")
		}
		config.S3 = &storage.S3Config{
			Bucket:    s3Bucket,
			Region:    s3Region,
			Endpoint:  s3Endpoint,
			AccessKey: s3AccessKey,
			SecretKey: s3SecretKey,
		}
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return config, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:     "dvom",
		Short:   "Docker Volume Manager - backup and restore tool",
		Long:    "DVOM (Docker Volume Manager) - A simple tool for backing up and restoring Docker container volumes with support for local and cloud storage backends",
		Version: version.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip backup directory validation for commands that don't need storage
			cmdName := cmd.Name()
			if cmdName == "volumes" {
				return nil
			}

			// Validate that backup directory is accessible (for local storage)
			if storageType == "local" && backupDir != "" {
				if _, err := os.Stat(backupDir); os.IsNotExist(err) {
					if err := os.MkdirAll(backupDir, 0750); err != nil {
						return fmt.Errorf("cannot create backup directory %s: %w", backupDir, err)
					}
				}
			}
			return nil
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&backupDir, "backup-dir", "./backups", "Directory to store backups (for local storage)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet output")

	// Storage backend flags
	rootCmd.PersistentFlags().StringVar(&storageType, "storage", "local", "Storage backend type (local, gcs, s3)")

	// GCS flags
	rootCmd.PersistentFlags().StringVar(&gcsBucket, "gcs-bucket", "", "GCS bucket name")
	rootCmd.PersistentFlags().StringVar(&gcsProject, "gcs-project", "", "GCS project ID")
	rootCmd.PersistentFlags().StringVar(&gcsCredsFile, "gcs-creds", "", "Path to GCS credentials file")

	// S3 flags
	rootCmd.PersistentFlags().StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket name")
	rootCmd.PersistentFlags().StringVar(&s3Region, "s3-region", "us-east-1", "S3 region")
	rootCmd.PersistentFlags().StringVar(&s3Endpoint, "s3-endpoint", "", "S3 endpoint (for S3-compatible services)")
	rootCmd.PersistentFlags().StringVar(&s3AccessKey, "s3-access-key", "", "S3 access key")
	rootCmd.PersistentFlags().StringVar(&s3SecretKey, "s3-secret-key", "", "S3 secret key")

	// Add commands
	rootCmd.AddCommand(createBackupCommand())
	rootCmd.AddCommand(createRestoreCommand())
	rootCmd.AddCommand(createListCommand())
	rootCmd.AddCommand(createInfoCommand())
	rootCmd.AddCommand(createVersionsCommand())
	rootCmd.AddCommand(createDeleteCommand())
	rootCmd.AddCommand(createVolumesCommand())
	rootCmd.AddCommand(createRepositoryCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a backup of a volume",
		Long:  "Create a backup of a Docker volume by name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			// Validate required flags
			if snapshotName == "" {
				return fmt.Errorf("--name is required to name the volume backup")
			}
			if volumeName == "" {
				return fmt.Errorf("--volume is required to specify which volume to backup")
			}

			// Set encryption options
			if encrypt || password != "" {
				client.SetEncryption(true, password)
			}

			// Direct volume backup
			return client.BackupDirectVolumeWithContainers(volumeName, snapshotName, stopContainers)
		},
	}

	cmd.Flags().StringVarP(&snapshotName, "name", "n", "", "Name for the volume backup")
	cmd.Flags().StringVar(&volumeName, "volume", "", "Volume name to backup")
	cmd.Flags().StringSliceVar(&stopContainers, "stop-containers", []string{}, "Container names/IDs to stop during backup (comma-separated)")
	cmd.Flags().BoolVar(&encrypt, "encrypt", false, "Encrypt the backup with AES-256")
	cmd.Flags().StringVar(&password, "password", "", "Password for encryption (will prompt if not provided)")

	return cmd
}

func createRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a volume backup to a volume",
		Long:  "Restore a volume backup directly to a Docker volume by name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			// Validate required flags
			if snapshotName == "" {
				return fmt.Errorf("--snapshot is required to specify which backup to restore")
			}
			if targetVolume == "" {
				return fmt.Errorf("--target-volume is required to specify which volume to restore to")
			}

			// Build versioned snapshot name if version is specified
			finalSnapshotName := snapshotName
			if versionFlag != "" {
				finalSnapshotName = fmt.Sprintf("%s@%s", snapshotName, versionFlag)
			}

			// Set password for decryption if provided
			if password != "" {
				client.SetEncryption(true, password)
			}

			// Direct volume restore
			return client.RestoreDirectVolumeWithContainers(targetVolume, finalSnapshotName, dryRun, force, stopContainers)
		},
	}

	cmd.Flags().StringVarP(&snapshotName, "snapshot", "s", "", "Name of the volume backup to restore")
	cmd.Flags().StringVar(&versionFlag, "version", "", "Specific version to restore (format: YYYYMMDD-HHMMSS)")
	cmd.Flags().StringVar(&targetVolume, "target-volume", "", "Target volume name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be restored without making changes")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	cmd.Flags().StringSliceVar(&stopContainers, "stop-containers", []string{}, "Container names/IDs to stop during restore (comma-separated)")
	cmd.Flags().StringVar(&password, "password", "", "Password for decryption (will prompt if encrypted and not provided)")

	return cmd
}

func createListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backups",
		Long:  "List all backup files in the configured storage backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			// List snapshots
			return client.ListSnapshots()
		},
	}
}

func createInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <snapshot-name>",
		Short: "Show detailed information about a volume backup",
		Long:  "Display detailed information about a volume backup including metadata and versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			snapshotName := args[0]

			// Get snapshot info
			return client.GetSnapshotInfo(snapshotName)
		},
	}
}

func createRepositoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Manage volume snapshots",
		Long:  "List and manage volume snapshots in the repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to listing snapshots
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			return client.ListSnapshots()
		},
	}

	return cmd
}

func createVersionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions <snapshot-name>",
		Short: "List all versions of a snapshot",
		Long:  "List all versions of a volume backup with timestamps and sizes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			snapshotName := args[0]
			return client.ListSnapshotVersions(snapshotName)
		},
	}

	return cmd
}

func createDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <snapshot-name> [--version=VERSION]",
		Short: "Delete volume backups by name or specific version",
		Long:  "Delete all versions of a volume backup or a specific version if --version is specified",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			storageConfig, err := buildStorageConfig()
			if err != nil {
				return err
			}

			storageBackend, err := storage.NewBackend(ctx, storageConfig)
			if err != nil {
				return err
			}

			client, err := backup.NewClientWithStorage(ctx, storageBackend, verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			snapshotName := args[0]

			// Build versioned snapshot name if version is specified
			finalSnapshotName := snapshotName
			if versionFlag != "" {
				finalSnapshotName = fmt.Sprintf("%s@%s", snapshotName, versionFlag)
			}

			return client.DeleteSnapshot(finalSnapshotName, force)
		},
	}

	cmd.Flags().StringVar(&versionFlag, "version", "", "Specific version to delete (format: YYYYMMDD-HHMMSS)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")

	return cmd
}

func createVolumesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "List all Docker volumes",
		Long:  "List all Docker volumes available on the system",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// We don't need storage backend for listing Docker volumes
			client, err := backup.NewClient("", verbose && !quiet)
			if err != nil {
				return err
			}
			client.SetQuiet(quiet)

			return client.ListDockerVolumes()
		},
	}

	return cmd
}
