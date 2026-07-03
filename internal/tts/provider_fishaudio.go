// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"miniflux.app/v2/internal/version"
)

type fishAudioProvider struct {
	config *ProviderConfig
}

func newFishAudioProvider(config *ProviderConfig) *fishAudioProvider {
	return &fishAudioProvider{config: config}
}

func (p *fishAudioProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Build request body
	bodyBytes, err := p.buildRequestBody(text)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("model", p.config.Model)
	req.Header.Set("User-Agent", "Miniflux/"+version.Version)

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Fish Audio request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp.StatusCode)
	}

	// Read streaming audio response
	limitedReader := io.LimitReader(resp.Body, maxFileSize+1)
	audioData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio stream: %w", err)
	}

	// Check size limit
	if len(audioData) > maxFileSize {
		return nil, fmt.Errorf("audio stream exceeds size limit (%d bytes)", maxFileSize)
	}

	return &ProviderResult{
		AudioData: audioData,
	}, nil
}

func (p *fishAudioProvider) buildRequestBody(text string) ([]byte, error) {
	reqBody := map[string]interface{}{
		"text":        text,
		"temperature": p.config.Temperature,
		"top_p":       p.config.TopP,
		"prosody": map[string]interface{}{
			"speed": p.config.Speed,
		},
		"format": p.config.Format,
	}

	// Only include reference_id when set (otherwise use Fish Audio's default voice)
	if p.config.ReferenceID != "" {
		reqBody["reference_id"] = p.config.ReferenceID
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bodyBytes, nil
}

func (p *fishAudioProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 401:
		return fmt.Errorf("Fish Audio authentication failed")
	case 402:
		return fmt.Errorf("Fish Audio payment required (free tier limit reached or model requires billing)")
	case 422:
		return fmt.Errorf("invalid Fish Audio request parameters")
	case 429:
		return fmt.Errorf("Fish Audio rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("Fish Audio service unavailable")
	default:
		return fmt.Errorf("Fish Audio request failed: HTTP %d", statusCode)
	}
}
