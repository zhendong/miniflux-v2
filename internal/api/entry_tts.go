// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/http/response/json"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/tts"
)

// getTTSAudio generates or retrieves cached TTS audio for an entry.
func (h *handler) getTTSAudio(w http.ResponseWriter, r *http.Request) {
	// Check if TTS is enabled
	if !config.Opts.TTSEnabled() {
		json.Forbidden(w, r)
		return
	}

	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")

	// Fetch entry with feed details
	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryID(entryID)
	builder.WithoutStatus(model.EntryStatusRemoved)

	entry, err := builder.GetEntry()
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	if entry == nil {
		json.NotFound(w, r)
		return
	}

	// Check rate limit
	rateLimitPerHour := config.Opts.TTSRateLimitPerHour()
	if !tts.AllowRequest(userID, rateLimitPerHour) {
		response.New(w, r).
			WithStatus(http.StatusTooManyRequests).
			WithHeader("Content-Type", "application/json").
			WithHeader("X-RateLimit-Remaining", "0").
			WithBody([]byte(`{"error_message":"rate limit exceeded"}`)).
			Write()
		return
	}

	// Create TTS provider configuration
	providerConfig := tts.NewProviderConfigFromLoader(config.Opts)

	// Get or generate audio
	cacheDuration := config.Opts.TTSCacheDuration()
	storagePath := config.Opts.TTSStoragePath()
	defaultLanguage := config.Opts.TTSDefaultLanguage()

	result, err := tts.GetOrGenerateAudio(
		h.store,
		providerConfig,
		entry,
		userID,
		cacheDuration,
		storagePath,
		defaultLanguage,
	)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	// Extract filename from result FilePath
	filename := filepath.Base(result.FilePath)

	// Return audio file URL (will be served via separate endpoint)
	json.OK(w, r, map[string]any{
		"audio_url":  "/tts/audio/" + filename,
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
	})
}

// serveTTSAudioFile serves the cached TTS audio file.
func (h *handler) serveTTSAudioFile(w http.ResponseWriter, r *http.Request) {
	// Check if TTS is enabled
	if !config.Opts.TTSEnabled() {
		json.Forbidden(w, r)
		return
	}

	userID := request.UserID(r)
	filename := request.RouteStringParam(r, "filename")

	// Parse entry_id and user_id from filename format: {entry_id}_{user_id}_{timestamp}.mp3
	// Validate filename to prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		json.BadRequest(w, r, errors.New("invalid filename"))
		return
	}

	// Parse entry and user IDs from filename
	parts := strings.Split(strings.TrimSuffix(filename, ".mp3"), "_")
	if len(parts) != 3 {
		json.BadRequest(w, r, errors.New("invalid filename format"))
		return
	}

	entryID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		json.BadRequest(w, r, errors.New("invalid entry ID"))
		return
	}

	fileUserID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		json.BadRequest(w, r, errors.New("invalid user ID"))
		return
	}

	// Verify requesting user matches file owner
	if userID != fileUserID {
		json.Forbidden(w, r)
		return
	}

	// Verify user still has access to entry
	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryID(entryID)
	builder.WithoutStatus(model.EntryStatusRemoved)

	entry, err := builder.GetEntry()
	if err != nil || entry == nil {
		json.Forbidden(w, r)
		return
	}

	// Build full file path
	storagePath := config.Opts.TTSStoragePath()
	relPath := filepath.Join("tts_audio", filename)
	fullPath := filepath.Join(storagePath, relPath)

	// Check file exists
	_, err = os.Stat(fullPath)
	if err != nil {
		json.NotFound(w, r)
		return
	}

	// Open file
	file, err := os.Open(fullPath)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}
	defer file.Close()

	// Serve file
	response.New(w, r).WithCaching(filename, 24*time.Hour, func(b *response.Builder) {
		b.WithHeader("Content-Type", "audio/mpeg")
		b.WithHeader("Content-Disposition", `inline; filename="`+filename+`"`)
		b.WithBody(file)
		b.WithoutCompression()
		b.Write()
	})
}
