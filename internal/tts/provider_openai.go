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

type openAIProvider struct {
	config *ProviderConfig
}

func newOpenAIProvider(config *ProviderConfig) *openAIProvider {
	return &openAIProvider{config: config}
}

func (p *openAIProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Build request body
	bodyBytes, err := p.buildRequestBody(text, language)
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
	req.Header.Set("User-Agent", "Miniflux/"+version.Version)

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
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

func (p *openAIProvider) buildRequestBody(text, language string) ([]byte, error) {
	reqBody := map[string]interface{}{
		"model":           p.config.Model,
		"input":           text,
		"voice":           p.config.Voice,
		"speed":           p.config.Speed,
		"response_format": p.config.Format,
	}

	// Only include instructions for gpt-4o-mini-tts model
	if p.config.Model == "gpt-4o-mini-tts" && p.config.Instructions != "" {
		reqBody["instructions"] = p.config.Instructions
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bodyBytes, nil
}

func (p *openAIProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400:
		return fmt.Errorf("invalid OpenAI request parameters")
	case 401:
		return fmt.Errorf("OpenAI authentication failed")
	case 429:
		return fmt.Errorf("OpenAI rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("OpenAI service unavailable")
	default:
		return fmt.Errorf("OpenAI request failed: HTTP %d", statusCode)
	}
}
