// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"fmt"
)

type elevenLabsProvider struct {
	config *ProviderConfig
}

func newElevenLabsProvider(config *ProviderConfig) *elevenLabsProvider {
	return &elevenLabsProvider{config: config}
}

func (p *elevenLabsProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}

func (p *elevenLabsProvider) buildRequestURL() string {
	return fmt.Sprintf("%s/%s/stream?output_format=%s&optimize_streaming_latency=%d",
		p.config.Endpoint,
		p.config.VoiceID,
		p.config.OutputFormat,
		p.config.OptimizeLatency,
	)
}

func (p *elevenLabsProvider) buildRequestBody(text string) []byte {
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

	bodyBytes, _ := json.Marshal(reqBody)
	return bodyBytes
}
