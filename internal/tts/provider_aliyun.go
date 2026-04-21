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
	maxSSEStreamSize    = 50 << 20 // 50MB total audio
	aliyunMaxChunkChars = 15000    // Aliyun limit is 16000 chars, leave margin
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

	// Split into chunks if text exceeds Aliyun's per-request limit
	if len([]rune(text)) > aliyunMaxChunkChars {
		return p.generateChunked(text, language)
	}

	return p.generateSingle(text, language)
}

// generateChunked splits text into chunks and concatenates audio results.
func (p *aliyunProvider) generateChunked(text, language string) (*ProviderResult, error) {
	chunks := splitTextIntoChunks(text, aliyunMaxChunkChars)
	var allAudio []byte

	for _, chunk := range chunks {
		result, err := p.generateSingle(chunk, language)
		if err != nil {
			return nil, err
		}
		allAudio = append(allAudio, result.AudioData...)
	}

	return &ProviderResult{AudioData: allAudio}, nil
}

// splitTextIntoChunks splits text on sentence boundaries to fit within maxChars (in runes).
func splitTextIntoChunks(text string, maxChars int) []string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return []string{text}
	}

	var chunks []string
	for len(runes) > 0 {
		end := maxChars
		if end > len(runes) {
			end = len(runes)
		}

		// Try to split on sentence boundary
		if end < len(runes) {
			chunk := string(runes[:end])
			// Look for last sentence-ending punctuation
			bestSplit := -1
			for _, sep := range []string{"。", ".", "！", "!", "？", "?", "；", ";", "\n"} {
				if idx := strings.LastIndex(chunk, sep); idx > len(chunk)/2 {
					if idx+len(sep) > bestSplit {
						bestSplit = idx + len(sep)
					}
				}
			}
			if bestSplit > 0 {
				end = len([]rune(chunk[:bestSplit]))
			}
		}

		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

func (p *aliyunProvider) generateSingle(text, language string) (*ProviderResult, error) {
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

	// Create HTTP request — allow 3 minutes for long text chunks
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Aliyun request failed: HTTP %d, body: %s", resp.StatusCode, string(body))
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
	var currentEvent string

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read SSE stream: %w", err)
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and comment lines (starting with ':')
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Track event type
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		// Process data lines
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		// Extract JSON data after "data:"
		jsonData := strings.TrimPrefix(line, "data:")
		jsonData = strings.TrimSpace(jsonData)

		// Check for error events
		if currentEvent == "error" {
			return nil, fmt.Errorf("Aliyun SSE error: %s", jsonData)
		}

		// Parse JSON response
		var sseEvent struct {
			Output struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"output"`
		}

		if err := json.Unmarshal([]byte(jsonData), &sseEvent); err != nil {
			return nil, fmt.Errorf("failed to parse SSE event JSON: %w (data: %s)", err, jsonData)
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
