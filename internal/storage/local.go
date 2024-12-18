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

// Read reads a file from storage and returns a reader for the file content
func (ls *LocalStorage) Read(ctx context.Context, fileID string) (io.Reader, *FileInfo, error) {
	ls.closeMutex.RLock()
	if ls.closed {
		ls.closeMutex.RUnlock()
		return nil, nil, ErrStorageClosed
	}
	ls.closeMutex.RUnlock()

	// Get file metadata
	ls.metaMutex.RLock()
	fileInfo, exists := ls.metadata[fileID]
	ls.metaMutex.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("file not found: %s", fileID)
	}

	// Create a pipe stream the file content
	pr, pw := io.Pipe()

	// Start a goroutine to write chunks to the pipe
	go func() {
		defer pw.Close()

		// Create a hasher for verifying file checksum
		hasher := sha256.New()

		// Read and verify each chunk
		for _, chunk := range fileInfo.ChunkSize {
			select {
			case <-ctx.Done():
				pw.CloseWithError(ctx.Err())
				return
			default:
			}

			// Read chunk file
			chunkPath := filepath.Join(ls.basePath, "chunks", fileID, fmt.Sprintf("%d.dat", chunk.Index))
			data, err := os.ReadFile(chunkPath)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("failed to read chunk %d: %w", chunk.Index, err))
				return
			}

			// Verify chunk checksum
			chunkHasher := sha256.New()
			chunkHasher.Write(data)
			checksum := hex.EncodeToString(chunkHasher.Sum(nil))
			if checksum != chunk.Checksum {
				pw.CloseWithError(fmt.Errorf("invalid chunk checksum: %s != %s", checksum, chunk.Checksum))
				return
			}

			// Write chunk data to pipe
			if _, err := pw.Write(data); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to write chunk %d: %w", chunk.Index, err))
				return
			}

			// Update file hash
			hasher.Write(data)
		}

		// Verify file checksum
		finalChecksum := hex.EncodeToString(hasher.Sum(nil))
		if finalChecksum != fileInfo.Checksum {
			pw.CloseWithError(fmt.Errorf("invalid file checksum: %s != %s", finalChecksum, fileInfo.Checksum))
		}

	}()

	// Return a reader to the pipe
	return pr, fileInfo, nil
}
