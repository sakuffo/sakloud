package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo represents metadata about a stored file
type FileInfo struct {
	ID        string
	Name      string
	Size      int64
	ChunkSize []ChunkInfo
	CreatedAt time.Time
	UpdatedAt time.Time
	Checksum  string
}

// ChunkInfo represents metadata about a file chunk
type ChunkInfo struct {
	ID       string
	FileID   string
	Index    int
	Size     int64
	Checksum string
}

// Storage is the interface for storing and retrieving files
type Storage interface {
	// Write writes a file to the storage and returns the file metadata
	Write(ctx context.Context, name string, reader io.Reader) (*FileInfo, error)

	// Read reads a file from the storage and writes it to the writer
	Read(ctx context.Context, fileID string, writer io.Writer) error

	// Delete deletes a file from the storage
	Delete(ctx context.Context, fileID string) error

	// GetFileInfo retrieves the metadata of a file
	GetFileInfo(ctx context.Context, fileID string) (*FileInfo, error)

	// ListFiles returns metadata of all files in the storage
	ListFiles(ctx context.Context) ([]*FileInfo, error)

	// Close closes the storage
	Close() error
}

// StorageConfig holds configuration for a storage implementation
type StorageConfig struct {
	BasePath     string
	MaxChunkSize int64
	MinFreeSpace int64
}

// DefaultConfig returns the default storage configuration
func DefaultConfig() *StorageConfig {
	return &StorageConfig{
		BasePath:     "data",
		MaxChunkSize: 64 * 1024 * 1024,
		MinFreeSpace: 1024 * 1024 * 1024 * 1024,
	}
}
