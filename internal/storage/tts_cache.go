// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"

	"miniflux.app/v2/internal/model"
)

// CreateTTSCache creates a new TTS cache entry.
func (s *Storage) CreateTTSCache(cache *model.TTSCache) error {
	query := `
		INSERT INTO tts_audio_cache (entry_id, user_id, file_path, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entry_id, user_id)
		DO UPDATE SET file_path = $3, expires_at = $4, created_at = NOW()
		RETURNING id, created_at
	`

	err := s.db.QueryRow(
		query,
		cache.EntryID,
		cache.UserID,
		cache.FilePath,
		cache.ExpiresAt,
	).Scan(&cache.ID, &cache.CreatedAt)

	if err != nil {
		return fmt.Errorf("unable to create TTS cache: %w", err)
	}

	return nil
}

// GetTTSCache retrieves a TTS cache entry by entry and user ID.
func (s *Storage) GetTTSCache(entryID, userID int64) (*model.TTSCache, error) {
	query := `
		SELECT id, entry_id, user_id, file_path, expires_at, created_at
		FROM tts_audio_cache
		WHERE entry_id = $1 AND user_id = $2
	`

	var cache model.TTSCache
	err := s.db.QueryRow(query, entryID, userID).Scan(
		&cache.ID,
		&cache.EntryID,
		&cache.UserID,
		&cache.FilePath,
		&cache.ExpiresAt,
		&cache.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("TTS cache not found for entry %d, user %d", entryID, userID)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to fetch TTS cache: %w", err)
	}

	return &cache, nil
}

// GetExpiredTTSCache retrieves all expired TTS cache entries.
func (s *Storage) GetExpiredTTSCache() ([]*model.TTSCache, error) {
	query := `
		SELECT id, entry_id, user_id, file_path, expires_at, created_at
		FROM tts_audio_cache
		WHERE expires_at < NOW()
		ORDER BY expires_at ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch expired TTS caches: %w", err)
	}
	defer rows.Close()

	var caches []*model.TTSCache
	for rows.Next() {
		var cache model.TTSCache
		err := rows.Scan(
			&cache.ID,
			&cache.EntryID,
			&cache.UserID,
			&cache.FilePath,
			&cache.ExpiresAt,
			&cache.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to scan TTS cache row: %w", err)
		}
		caches = append(caches, &cache)
	}

	return caches, nil
}

// DeleteTTSCache deletes a TTS cache entry by ID.
func (s *Storage) DeleteTTSCache(id int64) error {
	query := `DELETE FROM tts_audio_cache WHERE id = $1`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("unable to delete TTS cache: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to get rows affected: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("TTS cache %d not found", id)
	}

	return nil
}
