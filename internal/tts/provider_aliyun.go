// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

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
