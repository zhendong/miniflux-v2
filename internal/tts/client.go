// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"miniflux.app/v2/internal/model"
)

const (
	maxContentLength = 50000      // 50KB of text
	maxFileSize      = 50 << 20   // 50MB
)

// Client is HTTP client for TTS service.
type Client struct {
	endpointURL string
	apiKey      string
	voice       string
	httpClient  *http.Client
}

// NewClient creates a new TTS client.
func NewClient(endpointURL, apiKey, voice string) *Client {
	return &Client{
		endpointURL: endpointURL,
		apiKey:      apiKey,
		voice:       voice,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ServiceResult contains TTS service response.
type ServiceResult struct {
	AudioURL  string
	ExpiresAt time.Time
}

// Generate calls TTS service to generate audio.
func (c *Client) Generate(text string, language string) (*ServiceResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("entry content too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Prepare request
	reqBody := &model.TTSAudioRequest{
		Text:     text,
		Language: language,
		Voice:    c.voice,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpointURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS service request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleHTTPError(resp.StatusCode)
	}

	// Parse response
	var ttsResp model.TTSAudioResponse
	if err := json.NewDecoder(resp.Body).Decode(&ttsResp); err != nil {
		return nil, fmt.Errorf("failed to parse TTS response: %w", err)
	}

	// Parse expires_at timestamp
	expiresAt, err := time.Parse(time.RFC3339, ttsResp.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expires_at: %w", err)
	}

	return &ServiceResult{
		AudioURL:  ttsResp.AudioURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (c *Client) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400:
		return errors.New("invalid TTS request")
	case 401, 403:
		return errors.New("TTS authentication failed - check API key")
	case 429:
		return errors.New("TTS service rate limit exceeded")
	case 500, 502, 503:
		return errors.New("TTS service unavailable")
	default:
		return fmt.Errorf("TTS service error: HTTP %d", statusCode)
	}
}

// DownloadAudio downloads audio file from URL.
func (c *Client) DownloadAudio(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Create client with longer timeout for downloads
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("audio download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("audio download failed: HTTP %d", resp.StatusCode)
	}

	// Validate Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "audio/mpeg" && contentType != "audio/mp3" {
		return nil, fmt.Errorf("invalid content type: expected audio/mpeg, got %s", contentType)
	}

	// Check file size from Content-Length header
	if contentLengthStr := resp.Header.Get("Content-Length"); contentLengthStr != "" {
		contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
		if err == nil && contentLength > maxFileSize {
			return nil, fmt.Errorf("audio file too large: %d bytes (max %d)", contentLength, maxFileSize)
		}
	}

	// Download with size limit
	limitedReader := io.LimitReader(resp.Body, maxFileSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	// Check if size limit was exceeded
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("audio file too large: exceeds %d bytes", maxFileSize)
	}

	return data, nil
}
