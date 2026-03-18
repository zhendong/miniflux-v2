// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage implements AudioStorage for local filesystem.
type LocalStorage struct {
	basePath string
}

// newLocalStorage creates a new local filesystem storage backend.
func newLocalStorage(config *StorageConfig) *LocalStorage {
	return &LocalStorage{
		basePath: config.BasePath,
	}
}

// Save stores audio data to the local filesystem.
func (s *LocalStorage) Save(data []byte, path string) error {
	fullPath := filepath.Join(s.basePath, path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetURL returns the local file path.
// For local storage, we return the relative path which will be served by the API handler.
func (s *LocalStorage) GetURL(path string, expiresAt time.Time) (string, error) {
	// Verify file exists
	fullPath := filepath.Join(s.basePath, path)
	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	return path, nil
}

// Delete removes the audio file from local filesystem.
func (s *LocalStorage) Delete(path string) error {
	fullPath := filepath.Join(s.basePath, path)

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
