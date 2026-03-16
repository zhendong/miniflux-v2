// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"miniflux.app/v2/internal/model"

	_ "github.com/lib/pq"
)

// Test helper: create test storage with database connection
func newTestStorage(t *testing.T) *Storage {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost/miniflux_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("Unable to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("Unable to ping database: %v", err)
	}

	return NewStorage(db)
}

// Test helper: cleanup test storage
func cleanupTestStorage(t *testing.T, store *Storage) {
	// Clean up test data
	_, err := store.db.Exec("DELETE FROM tts_audio_cache WHERE id > 0")
	if err != nil {
		t.Logf("Warning: failed to cleanup tts_audio_cache: %v", err)
	}

	_, err = store.db.Exec("DELETE FROM entries WHERE id > 0")
	if err != nil {
		t.Logf("Warning: failed to cleanup entries: %v", err)
	}

	_, err = store.db.Exec("DELETE FROM feeds WHERE id > 0")
	if err != nil {
		t.Logf("Warning: failed to cleanup feeds: %v", err)
	}

	_, err = store.db.Exec("DELETE FROM categories WHERE id > 0")
	if err != nil {
		t.Logf("Warning: failed to cleanup categories: %v", err)
	}

	_, err = store.db.Exec("DELETE FROM users WHERE id > 1")
	if err != nil {
		t.Logf("Warning: failed to cleanup users: %v", err)
	}

	store.db.Close()
}

// Test helper: create test user and entry
func createTestUserAndEntry(t *testing.T, store *Storage) (userID int64, entryID int64) {
	// Create test user
	username := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	err := store.db.QueryRow(`
		INSERT INTO users (username, password, is_admin)
		VALUES ($1, $2, $3)
		RETURNING id
	`, username, "password", false).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test category
	var categoryID int64
	err = store.db.QueryRow(`
		INSERT INTO categories (user_id, title)
		VALUES ($1, $2)
		RETURNING id
	`, userID, "Test Category").Scan(&categoryID)
	if err != nil {
		t.Fatalf("Failed to create test category: %v", err)
	}

	// Create test feed
	var feedID int64
	err = store.db.QueryRow(`
		INSERT INTO feeds (user_id, category_id, feed_url, site_url, title)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, userID, categoryID, "https://example.com/feed", "https://example.com", "Test Feed").Scan(&feedID)
	if err != nil {
		t.Fatalf("Failed to create test feed: %v", err)
	}

	// Create test entry
	err = store.db.QueryRow(`
		INSERT INTO entries (user_id, feed_id, title, url, content, hash, published_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, userID, feedID, "Test Entry", "https://example.com/entry", "Test content", "test_hash", time.Now(), "unread").Scan(&entryID)
	if err != nil {
		t.Fatalf("Failed to create test entry: %v", err)
	}

	return userID, entryID
}

func TestStorage_CreateTTSCache(t *testing.T) {
	// Note: This requires test database setup
	// Skip if DB not available
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	cache := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/test.mp3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.CreateTTSCache(cache)
	if err != nil {
		t.Fatalf("CreateTTSCache failed: %v", err)
	}

	if cache.ID == 0 {
		t.Error("Expected non-zero ID after insert")
	}
}

func TestStorage_GetTTSCache_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	// Create cache entry
	original := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/test.mp3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	store.CreateTTSCache(original)

	// Retrieve it
	retrieved, err := store.GetTTSCache(entryID, userID)
	if err != nil {
		t.Fatalf("GetTTSCache failed: %v", err)
	}

	if retrieved.FilePath != original.FilePath {
		t.Errorf("Expected %s, got %s", original.FilePath, retrieved.FilePath)
	}
}

func TestStorage_GetTTSCache_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	_, err := store.GetTTSCache(99999, 99999)
	if err == nil {
		t.Error("Expected error for non-existent cache")
	}
}

func TestStorage_GetExpiredTTSCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	// Create expired cache entry
	expiredCache := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/expired.mp3",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
	}
	store.CreateTTSCache(expiredCache)

	// Get expired caches
	expired, err := store.GetExpiredTTSCache()
	if err != nil {
		t.Fatalf("GetExpiredTTSCache failed: %v", err)
	}

	if len(expired) == 0 {
		t.Error("Expected at least one expired cache entry")
	}
}

func TestStorage_DeleteTTSCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	// Create and then delete
	cache := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/test.mp3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	store.CreateTTSCache(cache)

	err := store.DeleteTTSCache(cache.ID)
	if err != nil {
		t.Fatalf("DeleteTTSCache failed: %v", err)
	}

	// Verify deletion
	_, err = store.GetTTSCache(entryID, userID)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}
