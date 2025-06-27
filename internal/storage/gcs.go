package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GCSStorage struct {
	client *storage.Client
	bucket string
}

func NewGCSStorage(ctx context.Context, config *GCSConfig) (*GCSStorage, error) {
	if config.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required for GCS storage")
	}

	var opts []option.ClientOption
	if config.Credentials != "" {
		opts = append(opts, option.WithCredentialsFile(config.Credentials))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSStorage{
		client: client,
		bucket: config.Bucket,
	}, nil
}

func (g *GCSStorage) Store(ctx context.Context, backup *Backup) error {
	bucket := g.client.Bucket(g.bucket)

	dataObj := bucket.Object(backup.ID + ".tar.gz")
	w := dataObj.NewWriter(ctx)

	if _, err := io.Copy(w, backup.DataReader); err != nil {
		if closeErr := w.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close writer: %v\n", closeErr)
		}
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	metadataObj := bucket.Object(backup.ID + ".json")
	metaWriter := metadataObj.NewWriter(ctx)

	if err := json.NewEncoder(metaWriter).Encode(backup.Metadata); err != nil {
		if closeErr := metaWriter.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close metadata writer: %v\n", closeErr)
		}
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := metaWriter.Close(); err != nil {
		return fmt.Errorf("failed to close metadata writer: %w", err)
	}

	return nil
}

func (g *GCSStorage) Retrieve(ctx context.Context, id string) (*Backup, error) {
	bucket := g.client.Bucket(g.bucket)

	metadataObj := bucket.Object(id + ".json")
	metaReader, err := metadataObj.NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, fmt.Errorf("backup not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	defer func() {
		if err := metaReader.Close(); err != nil {
			fmt.Printf("Warning: failed to close metadata reader: %v\n", err)
		}
	}()

	var metadata BackupMetadata
	if err := json.NewDecoder(metaReader).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	dataObj := bucket.Object(id + ".tar.gz")
	dataReader, err := dataObj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup data: %w", err)
	}

	return &Backup{
		ID:         id,
		Metadata:   metadata,
		DataReader: dataReader,
	}, nil
}

func (g *GCSStorage) List(ctx context.Context) ([]BackupMetadata, error) {
	bucket := g.client.Bucket(g.bucket)

	var backups []BackupMetadata
	it := bucket.Objects(ctx, &storage.Query{Delimiter: "/"})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		if len(attrs.Name) > 5 && attrs.Name[len(attrs.Name)-5:] == ".json" {
			obj := bucket.Object(attrs.Name)
			reader, err := obj.NewReader(ctx)
			if err != nil {
				continue
			}

			var metadata BackupMetadata
			if err := json.NewDecoder(reader).Decode(&metadata); err != nil {
				if closeErr := reader.Close(); closeErr != nil {
					fmt.Printf("Warning: failed to close reader: %v\n", closeErr)
				}
				continue
			}
			if err := reader.Close(); err != nil {
				fmt.Printf("Warning: failed to close reader: %v\n", err)
			}

			backups = append(backups, metadata)
		}
	}

	return backups, nil
}

func (g *GCSStorage) Delete(ctx context.Context, id string) error {
	bucket := g.client.Bucket(g.bucket)

	dataObj := bucket.Object(id + ".tar.gz")
	if err := dataObj.Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
		return fmt.Errorf("failed to delete backup data: %w", err)
	}

	metadataObj := bucket.Object(id + ".json")
	if err := metadataObj.Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

func (g *GCSStorage) Exists(ctx context.Context, id string) (bool, error) {
	bucket := g.client.Bucket(g.bucket)
	obj := bucket.Object(id + ".json")

	_, err := obj.Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, fmt.Errorf("failed to check backup existence: %w", err)
	}

	return true, nil
}

func (g *GCSStorage) Close() error {
	return g.client.Close()
}
