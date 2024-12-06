package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type LocalStorage struct {
	config     *StorageConfig
	basePath   string
	metadata   map[string]*FileInfo
	metaMutex  sync.RWMutex
	closed     bool
	closeMutex sync.RWMutex
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(cfg *StorageConfig) (*LocalStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	// Create chunks directory if it doesn't exist
	if err := os.MkdirAll(filepath.Join(cfg.BasePath, "chunks"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunks directory: %w", err)
	}

	return &LocalStorage{
		config:   cfg,
		basePath: cfg.BasePath,
		metadata: make(map[string]*FileInfo),
	}, nil

}

// Write implements the Storage interface
func (ls *LocalStorage) Write(ctx context.Context, name string, reader io.Reader) (*FileInfo, error) {
	ls.closeMutex.RLock()
	if ls.closed {
		ls.closeMutex.RUnlock()
		return nil, ErrStorageClosed
	}
	ls.closeMutex.RUnlock()

	// Create file info
	fileID := uuid.New().String()
	fileInfo := &FileInfo{
		ID:        fileID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Create chunks directory for this file
	chunkDir := filepath.Join(ls.basePath, "chunks", fileID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunks directory: %w", err)
	}

	// Initialize chunking
	buffer := make([]byte, ls.config.MaxChunkSize)
	hasher := sha256.New()
	var totalSize int64
	var chunks []ChunkInfo

	for chunkIndex := 0; ; chunkIndex++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		n, err := io.ReadFull(reader, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}

		if n == 0 {
			break
		}

		// Calculate chunk checksum
		chunkHasher := sha256.New()
		chunkHasher.Write(buffer[:n])
		chunkChecksum := hex.EncodeToString(chunkHasher.Sum(nil))

		// Create chunk info
		chunk := ChunkInfo{
			ID:       uuid.New().String(),
			FileID:   fileID,
			Index:    chunkIndex,
			Size:     int64(n),
Checksum: chunkChecksum,
		}

		// Write chunk to file
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d.dat", chunkIndex))
		if err := os.WriteFile(chunkPath, buffer[:n], 0644); err != nil {
			return nil, fmt.Errorf("failed to write chunk: %w", err)
		}

		chunks = append(chunks, chunk)
		totalSize += int64(n)
		hasher.Write(buffer[:n])
	}

	// Update file info
	fileInfo.Size = totalSize
	fileInfo.Checksum = hex.EncodeToString(hasher.Sum(nil))
	fileInfo.ChunkSize = chunks

	// Store metadata
	ls.metaMutex.Lock()
	ls.metadata[fileID] = fileInfo
	ls.metaMutex.Unlock()

	return fileInfo, nil
}
