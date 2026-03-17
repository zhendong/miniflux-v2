// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

type openAIProvider struct {
	config *ProviderConfig
}

func newOpenAIProvider(config *ProviderConfig) *openAIProvider {
	return &openAIProvider{config: config}
}

func (p *openAIProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}
