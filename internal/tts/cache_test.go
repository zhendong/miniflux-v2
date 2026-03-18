// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

type mockStorage struct {
	cache     *model.TTSCache
	cacheErr  error
	createErr error
}

func (m *mockStorage) GetTTSCache(entryID, userID int64) (*model.TTSCache, error) {
	if m.cacheErr != nil {
		return nil, m.cacheErr
	}
	return m.cache, nil
}

func (m *mockStorage) CreateTTSCache(cache *model.TTSCache) error {
	m.cache = cache
	return m.createErr
}

// mockCacheAudioStorage is a test implementation of the AudioStorage interface for cache testing.
type mockCacheAudioStorage struct {
	savedData map[string][]byte
	saveErr   error
}

func (m *mockCacheAudioStorage) Save(data []byte, path string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.savedData == nil {
		m.savedData = make(map[string][]byte)
	}
	m.savedData[path] = data
	return nil
}

func (m *mockCacheAudioStorage) GetURL(path string, expiresAt time.Time) (string, error) {
	return path, nil
}

func (m *mockCacheAudioStorage) Delete(path string) error {
	if m.savedData != nil {
		delete(m.savedData, path)
	}
	return nil
}

// mockProvider is a test implementation of the Provider interface.
type mockProvider struct {
	result *ProviderResult
	err    error
}

func (m *mockProvider) Generate(text, language string) (*ProviderResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// newMockProvider creates a mock provider that returns the given result.
func newMockProvider(config *ProviderConfig, result *ProviderResult, err error) Provider {
	return &mockProvider{
		result: result,
		err:    err,
	}
}

func TestGetOrGenerateAudio_CacheHit(t *testing.T) {
	store := &mockStorage{
		cache: &model.TTSCache{
			ID:        1,
			EntryID:   123,
			UserID:    456,
			FilePath:  "tts_audio/123_456_789.mp3",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	audioStorage := &mockCacheAudioStorage{}

	entry := &model.Entry{
		ID:      123,
		Title:   "Test",
		Content: "Test content",
		Feed:    &model.Feed{},
	}

	// Create a minimal provider config (not used since cache hit)
	providerConfig := &ProviderConfig{
		ProviderType: "openai",
		APIKey:       "test-key",
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}

	result, err := GetOrGenerateAudio(store, audioStorage, providerConfig, entry, 456, 24*time.Hour, "en")
	if err != nil {
		t.Fatalf("GetOrGenerateAudio failed: %v", err)
	}

	if !strings.Contains(result.FilePath, "123_456_789") {
		t.Errorf("Expected cached file path, got %s", result.FilePath)
	}
}

func TestGetOrGenerateAudio_CacheMiss(t *testing.T) {
	// Mock audio data
	audioData := []byte("generated audio data")

	// Override provider factory for this test
	originalFactory := providerFactory
	defer func() { providerFactory = originalFactory }()

	providerFactory = func(config *ProviderConfig) (Provider, error) {
		return newMockProvider(config, &ProviderResult{
			AudioData: audioData,
		}, nil), nil
	}

	store := &mockStorage{
		cacheErr: os.ErrNotExist, // Cache miss
	}

	audioStorage := &mockCacheAudioStorage{}

	entry := &model.Entry{
		ID:      123,
		Title:   "Test Title",
		Content: "Test content",
		Feed:    &model.Feed{},
	}

	// Create provider config for testing
	providerConfig := &ProviderConfig{
		ProviderType: "openai",
		APIKey:       "test-key",
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}

	_, err := GetOrGenerateAudio(store, audioStorage, providerConfig, entry, 456, 24*time.Hour, "en")
	if err != nil {
		t.Fatalf("GetOrGenerateAudio failed: %v", err)
	}

	// Verify audio was saved using storage backend
	if len(audioStorage.savedData) == 0 {
		t.Error("Expected audio to be saved")
	}

	// Verify saved content
	var savedContent []byte
	for _, data := range audioStorage.savedData {
		savedContent = data
		break
	}
	if string(savedContent) != string(audioData) {
		t.Errorf("Expected %s, got %s", audioData, savedContent)
	}

	// Verify cache was stored
	if store.cache == nil {
		t.Error("Expected cache to be stored")
	}
}

func TestGetOrGenerateAudio_ContentTooLarge(t *testing.T) {
	store := &mockStorage{
		cacheErr: os.ErrNotExist,
	}

	audioStorage := &mockCacheAudioStorage{}

	// Create entry with content > 50KB
	largeContent := strings.Repeat("a", 51000)
	entry := &model.Entry{
		ID:      123,
		Title:   "Test",
		Content: largeContent,
		Feed:    &model.Feed{},
	}

	// Create a minimal provider config
	providerConfig := &ProviderConfig{
		ProviderType: "openai",
		APIKey:       "test-key",
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}

	_, err := GetOrGenerateAudio(store, audioStorage, providerConfig, entry, 456, 24*time.Hour, "en")
	if err == nil {
		t.Error("Expected error for content too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}
