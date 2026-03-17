// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"miniflux.app/v2/internal/model"
)

// Storage interface for cache operations.
type Storage interface {
	GetTTSCache(entryID, userID int64) (*model.TTSCache, error)
	CreateTTSCache(cache *model.TTSCache) error
}

// AudioResult contains the result of audio generation.
type AudioResult struct {
	FilePath  string
	ExpiresAt time.Time
}

var (
	generationLocks = make(map[string]*sync.Mutex)
	locksMapMutex   sync.Mutex
)

// getLock gets or creates a mutex for a specific entry/user combination.
func getLock(entryID, userID int64) *sync.Mutex {
	key := fmt.Sprintf("%d:%d", entryID, userID)

	locksMapMutex.Lock()
	defer locksMapMutex.Unlock()

	if lock, exists := generationLocks[key]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	generationLocks[key] = lock
	return lock
}

// GetOrGenerateAudio retrieves cached audio or generates new audio.
func GetOrGenerateAudio(
	store Storage,
	client *Client,
	entry *model.Entry,
	userID int64,
	cacheDuration time.Duration,
	storagePath string,
	defaultLanguage string,
) (*AudioResult, error) {
	// Acquire lock for this entry/user pair
	lock := getLock(entry.ID, userID)
	lock.Lock()
	defer lock.Unlock()

	// Check cache
	cached, err := store.GetTTSCache(entry.ID, userID)
	if err == nil && cached.ExpiresAt.After(time.Now()) {
		// Verify file exists
		fullPath := filepath.Join(storagePath, cached.FilePath)
		if _, err := os.Stat(fullPath); err == nil {
			return &AudioResult{
				FilePath:  cached.FilePath,
				ExpiresAt: cached.ExpiresAt,
			}, nil
		}
	}

	// Cache miss - generate new audio
	// Validate content length
	text := entry.Title + "\n\n" + entry.Content
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("entry content too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Detect language
	language := DetectLanguage(entry, defaultLanguage)

	// Call TTS service
	serviceResult, err := client.Generate(text, language)
	if err != nil {
		return nil, fmt.Errorf("TTS generation failed: %w", err)
	}

	// Download audio file
	audioData, err := client.DownloadAudio(serviceResult.AudioURL)
	if err != nil {
		return nil, fmt.Errorf("audio download failed: %w", err)
	}

	// Save locally
	filename := fmt.Sprintf("%d_%d_%d.mp3", entry.ID, userID, time.Now().Unix())
	relPath := filepath.Join("tts_audio", filename)
	fullPath := filepath.Join(storagePath, relPath)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, audioData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write audio file: %w", err)
	}

	// Store in cache DB
	expiresAt := time.Now().Add(cacheDuration)
	if err := store.CreateTTSCache(&model.TTSCache{
		EntryID:   entry.ID,
		UserID:    userID,
		FilePath:  relPath,
		ExpiresAt: expiresAt,
	}); err != nil {
		// Log but don't fail - file is saved
		fmt.Printf("Warning: failed to cache TTS metadata: %v\n", err)
	}

	return &AudioResult{
		FilePath:  relPath,
		ExpiresAt: expiresAt,
	}, nil
}
