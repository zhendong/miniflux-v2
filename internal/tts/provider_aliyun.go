// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	maxSSEStreamSize = 50 << 20 // 50MB total audio
)

type aliyunProvider struct {
	config *ProviderConfig
}

func newAliyunProvider(config *ProviderConfig) *aliyunProvider {
	return &aliyunProvider{config: config}
}

func (p *aliyunProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
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

	return audioData, nil
}
