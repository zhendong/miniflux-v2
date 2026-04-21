// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

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
	"miniflux.app/v2/internal/tts"
)

func (h *handler) getTTSAudio(w http.ResponseWriter, r *http.Request) {
	if !config.Opts.TTSEnabled() {
		response.JSONForbidden(w, r)
		return
	}

	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")

	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryID(entryID)

	entry, err := builder.GetEntry()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if entry == nil {
		response.JSONNotFound(w, r)
		return
	}

	rateLimitPerHour := config.Opts.TTSRateLimitPerHour()
	if !tts.AllowRequest(userID, rateLimitPerHour) {
		response.NewBuilder(w, r).
			WithStatus(http.StatusTooManyRequests).
			WithHeader("Content-Type", "application/json").
			WithHeader("X-RateLimit-Remaining", "0").
			WithBodyAsBytes([]byte(`{"error_message":"rate limit exceeded"}`)).
			Write()
		return
	}

	providerConfig := tts.NewProviderConfigFromLoader(config.Opts)

	storageConfig := tts.NewStorageConfigFromLoader(config.Opts)
	storage, err := tts.NewAudioStorage(storageConfig)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	cacheDuration := config.Opts.TTSCacheDuration()
	defaultLanguage := config.Opts.TTSDefaultLanguage()

	result, err := tts.GetOrGenerateAudio(
		h.store,
		storage,
		providerConfig,
		entry,
		userID,
		cacheDuration,
		defaultLanguage,
	)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	filename := filepath.Base(result.FilePath)

	response.JSON(w, r, map[string]any{
		"audio_url":  h.routePath("/entry/tts/audio/%s", filename),
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *handler) serveTTSAudio(w http.ResponseWriter, r *http.Request) {
	if !config.Opts.TTSEnabled() {
		response.JSONForbidden(w, r)
		return
	}

	userID := request.UserID(r)
	filename := request.RouteStringParam(r, "filename")

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		response.JSONBadRequest(w, r, errors.New("invalid filename"))
		return
	}

	parts := strings.Split(strings.TrimSuffix(filename, ".mp3"), "_")
	if len(parts) != 3 {
		response.JSONBadRequest(w, r, errors.New("invalid filename format"))
		return
	}

	entryID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		response.JSONBadRequest(w, r, errors.New("invalid entry ID"))
		return
	}

	fileUserID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		response.JSONBadRequest(w, r, errors.New("invalid user ID"))
		return
	}

	if userID != fileUserID {
		response.JSONForbidden(w, r)
		return
	}

	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryID(entryID)

	entry, err := builder.GetEntry()
	if err != nil || entry == nil {
		response.JSONForbidden(w, r)
		return
	}

	// For R2 storage, redirect to presigned URL
	if config.Opts.TTSStorageBackend() == "r2" {
		storageConfig := tts.NewStorageConfigFromLoader(config.Opts)
		storage, err := tts.NewAudioStorage(storageConfig)
		if err != nil {
			response.JSONServerError(w, r, err)
			return
		}

		cache, err := h.store.GetTTSCache(entryID, userID)
		if err != nil {
			response.JSONNotFound(w, r)
			return
		}

		relPath := filepath.Join("tts_audio", filename)
		url, err := storage.GetURL(relPath, cache.ExpiresAt)
		if err != nil {
			response.JSONServerError(w, r, err)
			return
		}

		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	// For local storage, serve file directly
	storagePath := config.Opts.TTSStoragePath()
	relPath := filepath.Join("tts_audio", filename)
	fullPath := filepath.Join(storagePath, relPath)

	_, err = os.Stat(fullPath)
	if err != nil {
		response.JSONNotFound(w, r)
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	defer file.Close()

	response.NewBuilder(w, r).WithCaching(filename, 24*time.Hour, func(b *response.Builder) {
		b.WithHeader("Content-Type", "audio/mpeg")
		b.WithInline(filename)
		b.WithBodyAsReader(file)
		b.WithoutCompression()
		b.Write()
	})
}
