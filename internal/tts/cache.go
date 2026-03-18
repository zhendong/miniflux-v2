// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"miniflux.app/v2/internal/model"
)

// providerFactory is a hook for tests to override provider creation.
var providerFactory = NewProvider

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
	storage AudioStorage,
	providerConfig *ProviderConfig,
	entry *model.Entry,
	userID int64,
	cacheDuration time.Duration,
	defaultLanguage string,
) (*AudioResult, error) {
	// Acquire lock for this entry/user pair
	lock := getLock(entry.ID, userID)
	lock.Lock()
	defer lock.Unlock()

	// Check cache
	cached, err := store.GetTTSCache(entry.ID, userID)
	if err == nil && cached.ExpiresAt.After(time.Now()) {
		// Cache hit - return cached file path
		return &AudioResult{
			FilePath:  cached.FilePath,
			ExpiresAt: cached.ExpiresAt,
		}, nil
	}

	// Cache miss - generate new audio
	// Validate content length
	text := entry.Title + "\n\n" + entry.Content
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("entry content too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Detect language
	language := DetectLanguage(entry, defaultLanguage)

	// Create provider
	provider, err := providerFactory(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS provider: %w", err)
	}

	// Generate audio
	result, err := provider.Generate(text, language)
	if err != nil {
		return nil, fmt.Errorf("TTS generation failed: %w", err)
	}

	// Handle both AudioData and AudioURL results
	var audioData []byte

	if len(result.AudioData) > 0 {
		// Streaming provider (OpenAI, Aliyun streaming, ElevenLabs)
		audioData = result.AudioData
	} else if result.AudioURL != "" {
		// URL-based provider (Aliyun non-streaming)
		// Use existing DownloadAudio from client.go
		ttsClient := NewClient("", "", "")
		audioData, err = ttsClient.DownloadAudio(result.AudioURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download audio: %w", err)
		}
	} else {
		return nil, fmt.Errorf("provider returned no audio data or URL")
	}

	// Save audio using storage backend
	filename := fmt.Sprintf("%d_%d_%d.mp3", entry.ID, userID, time.Now().Unix())
	relPath := filepath.Join("tts_audio", filename)

	if err := storage.Save(audioData, relPath); err != nil {
		return nil, fmt.Errorf("failed to save audio: %w", err)
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
		slog.Warn("Failed to cache TTS metadata",
			slog.Int64("entry_id", entry.ID),
			slog.Int64("user_id", userID),
			slog.Any("error", err),
		)
	}

	return &AudioResult{
		FilePath:  relPath,
		ExpiresAt: expiresAt,
	}, nil
}
