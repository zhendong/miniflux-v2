// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"time"
)

// AudioStorage is the interface for TTS audio storage backends.
type AudioStorage interface {
	// Save stores audio data at the specified path.
	// path is relative (e.g., "tts_audio/123_456_789.mp3")
	Save(data []byte, path string) error

	// GetURL returns a URL for accessing the audio file.
	// For local storage: returns the path
	// For R2 storage: returns presigned URL valid until expiresAt
	GetURL(path string, expiresAt time.Time) (string, error)

	// Delete removes the audio file at the specified path.
	Delete(path string) error
}

// StorageConfig contains configuration for audio storage backends.
type StorageConfig struct {
	// Backend type: "local" or "r2"
	Backend string

	// For local storage: base directory path
	// For R2 storage: not used (uses R2 config instead)
	BasePath string

	// R2-specific configuration
	R2Endpoint        string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2PublicURL       string
}

// NewAudioStorage creates a new audio storage backend based on config.
func NewAudioStorage(config *StorageConfig) (AudioStorage, error) {
	switch config.Backend {
	case "local":
		return newLocalStorage(config), nil
	case "r2":
		return newR2Storage(config)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", config.Backend)
	}
}

// R2Storage is a placeholder type for Cloudflare R2 storage backend.
// Will be fully implemented in Task 3.1.
type R2Storage struct {
	// TODO: add fields in Task 3.1
}

// Save implements AudioStorage interface.
// TODO: implement in Task 3.1.
func (r *R2Storage) Save(data []byte, path string) error {
	return nil
}

// GetURL implements AudioStorage interface.
// TODO: implement in Task 3.1.
func (r *R2Storage) GetURL(path string, expiresAt time.Time) (string, error) {
	return "", nil
}

// Delete implements AudioStorage interface.
// TODO: implement in Task 3.1.
func (r *R2Storage) Delete(path string) error {
	return nil
}

// Stub implementations - will be replaced in future tasks
func newR2Storage(config *StorageConfig) (AudioStorage, error) {
	return &R2Storage{}, nil // TODO: implement in Task 3.1
}
