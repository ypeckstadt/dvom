package backup

import (
	"context"
	"fmt"
	"os"

	"github.com/ypeckstadt/dvom/internal/docker"
	"github.com/ypeckstadt/dvom/internal/storage"
)

// Client wraps Docker client with backup functionality
type Client struct {
	docker       *docker.Client
	backupDir    string
	verbose      bool
	quiet        bool
	storage      storage.Backend
	ctx          context.Context
	encryptEnabled bool
	password     string
}

// NewClient creates a new backup client
func NewClient(backupDir string, verbose bool) (*Client, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, err
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &Client{
		docker:    dockerClient,
		backupDir: backupDir,
		verbose:   verbose,
		ctx:       context.Background(),
	}, nil
}

// NewClientWithStorage creates a new backup client with custom storage backend
func NewClientWithStorage(ctx context.Context, storageBackend storage.Backend, verbose bool) (*Client, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, err
	}

	return &Client{
		docker:  dockerClient,
		verbose: verbose,
		quiet:   false,
		storage: storageBackend,
		ctx:     ctx,
	}, nil
}

// SetQuiet sets the quiet mode for the client
func (c *Client) SetQuiet(quiet bool) {
	c.quiet = quiet
}

// SetEncryption sets encryption settings for the client
func (c *Client) SetEncryption(enabled bool, password string) {
	c.encryptEnabled = enabled
	c.password = password
}
