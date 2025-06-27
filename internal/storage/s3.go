package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3Storage(ctx context.Context, cfg *S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required for S3 storage")
	}

	var awsConfig aws.Config
	var err error

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
			),
		)
	} else {
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var clientOptions []func(*s3.Options)
	if cfg.Endpoint != "" {
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsConfig, clientOptions...)

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Storage) Store(ctx context.Context, backup *Backup) error {
	data, err := io.ReadAll(backup.DataReader)
	if err != nil {
		return fmt.Errorf("failed to read backup data: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(backup.ID + ".tar.gz"),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload backup data: %w", err)
	}

	metadataBytes, err := json.Marshal(backup.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(backup.ID + ".json"),
		Body:        bytes.NewReader(metadataBytes),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

func (s *S3Storage) Retrieve(ctx context.Context, id string) (*Backup, error) {
	metadataResult, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(id + ".json"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve metadata: %w", err)
	}
	defer func() {
		if err := metadataResult.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close metadata result body: %v\n", err)
		}
	}()

	var metadata BackupMetadata
	if err := json.NewDecoder(metadataResult.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	dataResult, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(id + ".tar.gz"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backup data: %w", err)
	}

	return &Backup{
		ID:         id,
		Metadata:   metadata,
		DataReader: dataResult.Body,
	}, nil
}

func (s *S3Storage) List(ctx context.Context) ([]BackupMetadata, error) {
	var backups []BackupMetadata

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range output.Contents {
			if len(*obj.Key) > 5 && (*obj.Key)[len(*obj.Key)-5:] == ".json" {
				metadataResult, err := s.client.GetObject(ctx, &s3.GetObjectInput{
					Bucket: aws.String(s.bucket),
					Key:    obj.Key,
				})
				if err != nil {
					continue
				}

				var metadata BackupMetadata
				if err := json.NewDecoder(metadataResult.Body).Decode(&metadata); err != nil {
					if closeErr := metadataResult.Body.Close(); closeErr != nil {
						fmt.Printf("Warning: failed to close metadata result body: %v\n", closeErr)
					}
					continue
				}
				if err := metadataResult.Body.Close(); err != nil {
					fmt.Printf("Warning: failed to close metadata result body: %v\n", err)
				}

				backups = append(backups, metadata)
			}
		}
	}

	return backups, nil
}

func (s *S3Storage) Delete(ctx context.Context, id string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(id + ".tar.gz"),
	})
	if err != nil {
		return fmt.Errorf("failed to delete backup data: %w", err)
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(id + ".json"),
	})
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

func (s *S3Storage) Exists(ctx context.Context, id string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(id + ".json"),
	})
	if err != nil {
		return false, nil
	}

	return true, nil
}
