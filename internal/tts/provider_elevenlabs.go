// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

type elevenLabsProvider struct {
	config *ProviderConfig
}

func newElevenLabsProvider(config *ProviderConfig) *elevenLabsProvider {
	return &elevenLabsProvider{config: config}
}

func (p *elevenLabsProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}
