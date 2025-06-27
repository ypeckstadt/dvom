package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(config *LocalConfig) (*LocalStorage, error) {
	if config.BasePath == "" {
		return nil, fmt.Errorf("base path is required for local storage")
	}

	if err := os.MkdirAll(config.BasePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalStorage{
		basePath: config.BasePath,
	}, nil
}

func (l *LocalStorage) Store(ctx context.Context, backup *Backup) error {
	backupPath := filepath.Join(l.basePath, backup.ID)

	dataFile, err := os.Create(backupPath + ".tar.gz") // #nosec G304 - controlled backup storage path
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() {
		if err := dataFile.Close(); err != nil {
			fmt.Printf("Warning: failed to close data file: %v\n", err)
		}
	}()

	if _, err := io.Copy(dataFile, backup.DataReader); err != nil {
		if removeErr := os.Remove(backupPath + ".tar.gz"); removeErr != nil {
			fmt.Printf("Warning: failed to remove backup file: %v\n", removeErr)
		}
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	metadataFile, err := os.Create(backupPath + ".json") // #nosec G304 - controlled backup storage path
	if err != nil {
		if removeErr := os.Remove(backupPath + ".tar.gz"); removeErr != nil {
			fmt.Printf("Warning: failed to remove backup file: %v\n", removeErr)
		}
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer func() {
		if err := metadataFile.Close(); err != nil {
			fmt.Printf("Warning: failed to close metadata file: %v\n", err)
		}
	}()

	if err := json.NewEncoder(metadataFile).Encode(backup.Metadata); err != nil {
		if removeErr := os.Remove(backupPath + ".tar.gz"); removeErr != nil {
			fmt.Printf("Warning: failed to remove backup file: %v\n", removeErr)
		}
		if removeErr := os.Remove(backupPath + ".json"); removeErr != nil {
			fmt.Printf("Warning: failed to remove metadata file: %v\n", removeErr)
		}
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

func (l *LocalStorage) Retrieve(ctx context.Context, id string) (*Backup, error) {
	backupPath := filepath.Join(l.basePath, id)

	metadataFile, err := os.Open(backupPath + ".json") // #nosec G304 - controlled backup storage path
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("backup not found: %s", id)
		}
		return nil, fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer func() {
		if err := metadataFile.Close(); err != nil {
			fmt.Printf("Warning: failed to close metadata file: %v\n", err)
		}
	}()

	var metadata BackupMetadata
	if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	dataFile, err := os.Open(backupPath + ".tar.gz") // #nosec G304 - controlled backup storage path
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}

	return &Backup{
		ID:         id,
		Metadata:   metadata,
		DataReader: dataFile,
	}, nil
}

func (l *LocalStorage) List(ctx context.Context) ([]BackupMetadata, error) {
	entries, err := os.ReadDir(l.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupMetadata
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			metadataPath := filepath.Join(l.basePath, entry.Name())

			metadataFile, err := os.Open(metadataPath) // #nosec G304 - controlled backup storage path
			if err != nil {
				continue
			}

			var metadata BackupMetadata
			if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
				if closeErr := metadataFile.Close(); closeErr != nil {
					fmt.Printf("Warning: failed to close metadata file: %v\n", closeErr)
				}
				continue
			}
			if err := metadataFile.Close(); err != nil {
				fmt.Printf("Warning: failed to close metadata file: %v\n", err)
			}

			backups = append(backups, metadata)
		}
	}

	return backups, nil
}

func (l *LocalStorage) Delete(ctx context.Context, id string) error {
	backupPath := filepath.Join(l.basePath, id)

	if err := os.Remove(backupPath + ".tar.gz"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup file: %w", err)
	}

	if err := os.Remove(backupPath + ".json"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove metadata file: %w", err)
	}

	return nil
}

func (l *LocalStorage) Exists(ctx context.Context, id string) (bool, error) {
	backupPath := filepath.Join(l.basePath, id)

	if _, err := os.Stat(backupPath + ".json"); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check backup existence: %w", err)
	}

	return true, nil
}
