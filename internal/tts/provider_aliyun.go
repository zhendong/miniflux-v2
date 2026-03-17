// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"miniflux.app/v2/internal/version"
)

const (
	maxSSEStreamSize = 50 << 20   // 50MB total audio
)

type aliyunProvider struct {
	config *ProviderConfig
}

func newAliyunProvider(config *ProviderConfig) *aliyunProvider {
	return &aliyunProvider{config: config}
}

func (p *aliyunProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Map language code to full name
	languageType := p.config.LanguageType
	if languageType == "" {
		languageType = p.mapLanguage(language)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"model": p.config.Model,
		"input": map[string]string{
			"text": text,
		},
	}

	if p.config.Voice != "" {
		reqBody["input"].(map[string]string)["voice"] = p.config.Voice
	}

	if languageType != "" {
		reqBody["input"].(map[string]string)["language_type"] = languageType
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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

	// Enable SSE for streaming mode
	if p.config.Stream {
		req.Header.Set("X-DashScope-SSE", "enable")
	}

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Aliyun request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp.StatusCode)
	}

	// Handle streaming vs non-streaming response
	if p.config.Stream {
		// Parse SSE stream
		reader := bufio.NewReader(resp.Body)
		audioData, err := p.parseSSEStream(reader)
		if err != nil {
			return nil, err
		}
		return &ProviderResult{
			AudioData: audioData,
		}, nil
	} else {
		// Parse JSON response with URL
		var jsonResp struct {
			Output struct {
				Audio struct {
					URL string `json:"url"`
				} `json:"audio"`
			} `json:"output"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			return nil, fmt.Errorf("failed to parse Aliyun response: %w", err)
		}

		if jsonResp.Output.Audio.URL == "" {
			return nil, fmt.Errorf("Aliyun response contains empty audio URL")
		}

		return &ProviderResult{
			AudioURL: jsonResp.Output.Audio.URL,
		}, nil
	}
}

func (p *aliyunProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400:
		return fmt.Errorf("invalid Aliyun request parameters")
	case 401, 403:
		return fmt.Errorf("Aliyun authentication failed")
	case 429:
		return fmt.Errorf("Aliyun rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("Aliyun service unavailable")
	default:
		return fmt.Errorf("Aliyun request failed: HTTP %d", statusCode)
	}
}

// mapLanguage converts ISO 639-1 codes to Aliyun full language names
func (p *aliyunProvider) mapLanguage(isoCode string) string {
	languageMap := map[string]string{
		"en": "English",
		"zh": "Chinese",
		"ja": "Japanese",
		"ko": "Korean",
		"de": "German",
		"fr": "French",
		"ru": "Russian",
		"pt": "Portuguese",
		"es": "Spanish",
		"it": "Italian",
	}
	return languageMap[isoCode]
}

// parseSSEStream reads SSE events and accumulates base64-decoded audio chunks
func (p *aliyunProvider) parseSSEStream(reader *bufio.Reader) ([]byte, error) {
	var audioData []byte

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read SSE stream: %w", err)
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and non-data lines
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		// Extract JSON data after "data: "
		jsonData := strings.TrimPrefix(line, "data:")
		jsonData = strings.TrimSpace(jsonData)

		// Parse JSON response
		var sseEvent struct {
			Output struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"output"`
		}

		if err := json.Unmarshal([]byte(jsonData), &sseEvent); err != nil {
			return nil, fmt.Errorf("failed to parse SSE event JSON: %w", err)
		}

		// Decode base64 audio chunk
		if sseEvent.Output.Audio.Data != "" {
			chunk, err := base64.StdEncoding.DecodeString(sseEvent.Output.Audio.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode audio chunk: %w", err)
			}
			audioData = append(audioData, chunk...)

			// Check size limit
			if len(audioData) > maxSSEStreamSize {
				return nil, fmt.Errorf("audio stream exceeds size limit (%d bytes)", maxSSEStreamSize)
			}
		}
	}

	// Ensure we received audio data from the stream
	if len(audioData) == 0 {
		return nil, fmt.Errorf("no audio data received from SSE stream")
	}

	return audioData, nil
}
