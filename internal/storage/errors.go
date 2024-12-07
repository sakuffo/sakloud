package storage

import "errors"

var (
	ErrFileNotFound    = errors.New("file not found")
	ErrInvalidFileID   = errors.New("invalid file ID")
	ErrStorageFull     = errors.New("storage is full")
	ErrInvalidChecksum = errors.New("invalid checksum")
	ErrStorageClosed   = errors.New("storage is closed")
)
