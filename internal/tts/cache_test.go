// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestGetOrGenerateAudio_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a cached file
	cachedPath := filepath.Join(tmpDir, "tts_audio", "123_456_789.mp3")
	os.MkdirAll(filepath.Dir(cachedPath), 0755)
	os.WriteFile(cachedPath, []byte("cached audio"), 0644)

	store := &mockStorage{
		cache: &model.TTSCache{
			ID:        1,
			EntryID:   123,
			UserID:    456,
			FilePath:  "tts_audio/123_456_789.mp3",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	entry := &model.Entry{
		ID:      123,
		Title:   "Test",
		Content: "Test content",
		Feed:    &model.Feed{},
	}

	client := NewClient("", "", "")

	result, err := GetOrGenerateAudio(store, client, entry, 456, 24*time.Hour, tmpDir, "en")
	if err != nil {
		t.Fatalf("GetOrGenerateAudio failed: %v", err)
	}

	if !strings.Contains(result.FilePath, "123_456_789") {
		t.Errorf("Expected cached file path, got %s", result.FilePath)
	}
}

func TestGetOrGenerateAudio_CacheMiss(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)
	tmpDir := t.TempDir()

	// Mock TTS server
	audioData := []byte("generated audio data")
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tts" {
			// TTS generation endpoint
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"audio_url": "` + serverURL + `/audio.mp3", "expires_at": "2026-03-17T10:00:00Z"}`))
		} else {
			// Audio download endpoint
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write(audioData)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	store := &mockStorage{
		cacheErr: os.ErrNotExist, // Cache miss
	}

	entry := &model.Entry{
		ID:      123,
		Title:   "Test Title",
		Content: "Test content",
		Feed:    &model.Feed{},
	}

	client := NewClient(server.URL+"/tts", "test-key", "alloy")

	result, err := GetOrGenerateAudio(store, client, entry, 456, 24*time.Hour, tmpDir, "en")
	if err != nil {
		t.Fatalf("GetOrGenerateAudio failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, result.FilePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Error("Expected audio file to be created")
	}

	// Verify file content
	content, _ := os.ReadFile(fullPath)
	if string(content) != string(audioData) {
		t.Errorf("Expected %s, got %s", audioData, content)
	}

	// Verify cache was stored
	if store.cache == nil {
		t.Error("Expected cache to be stored")
	}
}

func TestGetOrGenerateAudio_ContentTooLarge(t *testing.T) {
	tmpDir := t.TempDir()

	store := &mockStorage{
		cacheErr: os.ErrNotExist,
	}

	// Create entry with content > 50KB
	largeContent := strings.Repeat("a", 51000)
	entry := &model.Entry{
		ID:      123,
		Title:   "Test",
		Content: largeContent,
		Feed:    &model.Feed{},
	}

	client := NewClient("", "", "")

	_, err := GetOrGenerateAudio(store, client, entry, 456, 24*time.Hour, tmpDir, "en")
	if err == nil {
		t.Error("Expected error for content too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}
