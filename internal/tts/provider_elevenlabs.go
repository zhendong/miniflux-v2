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
	"net/url"
	"time"

	"miniflux.app/v2/internal/version"
)

type elevenLabsProvider struct {
	config *ProviderConfig
}

func newElevenLabsProvider(config *ProviderConfig) *elevenLabsProvider {
	return &elevenLabsProvider{config: config}
}

func (p *elevenLabsProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Build request
	url := p.buildRequestURL()
	bodyBytes, err := p.buildRequestBody(text)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", p.config.APIKey)
	req.Header.Set("User-Agent", "Miniflux/"+version.Version)

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs request failed: %w", err)
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

func (p *elevenLabsProvider) buildRequestURL() string {
	return fmt.Sprintf("%s/%s/stream?output_format=%s&optimize_streaming_latency=%d",
		p.config.Endpoint,
		url.PathEscape(p.config.VoiceID),
		url.QueryEscape(p.config.OutputFormat),
		p.config.OptimizeLatency,
	)
}

func (p *elevenLabsProvider) buildRequestBody(text string) ([]byte, error) {
	reqBody := map[string]interface{}{
		"text":     text,
		"model_id": p.config.Model,
		"voice_settings": map[string]interface{}{
			"stability":         p.config.Stability,
			"similarity_boost":  p.config.SimilarityBoost,
			"style":             p.config.Style,
			"use_speaker_boost": p.config.SpeakerBoost,
		},
	}

	// Only include language_code if specified
	if p.config.LanguageCode != "" {
		reqBody["language_code"] = p.config.LanguageCode
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bodyBytes, nil
}

func (p *elevenLabsProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400, 422:
		return fmt.Errorf("invalid ElevenLabs request parameters")
	case 401:
		return fmt.Errorf("ElevenLabs authentication failed")
	case 429:
		return fmt.Errorf("ElevenLabs rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("ElevenLabs service unavailable")
	default:
		return fmt.Errorf("ElevenLabs request failed: HTTP %d", statusCode)
	}
}
