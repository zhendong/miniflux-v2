// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
)

type openAIProvider struct {
	config *ProviderConfig
}

func newOpenAIProvider(config *ProviderConfig) *openAIProvider {
	return &openAIProvider{config: config}
}

func (p *openAIProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}

func (p *openAIProvider) buildRequestBody(text, language string) []byte {
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

	bodyBytes, _ := json.Marshal(reqBody)
	return bodyBytes
}
