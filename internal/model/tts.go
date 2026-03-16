// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model // import "miniflux.app/v2/internal/model"

import (
	"time"
)

// TTSCache represents a cached TTS audio file.
type TTSCache struct {
	ID        int64
	EntryID   int64
	UserID    int64
	FilePath  string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// TTSAudioRequest represents a request to generate TTS audio.
type TTSAudioRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Voice    string `json:"voice"`
}

// TTSAudioResponse represents the response from TTS service.
type TTSAudioResponse struct {
	AudioURL  string `json:"audio_url"`
	ExpiresAt string `json:"expires_at"` // ISO 8601 timestamp
}
