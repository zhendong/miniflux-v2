// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"
)

func TestAliyun_MapLanguage(t *testing.T) {
	tests := []struct {
		isoCode  string
		expected string
	}{
		{"en", "English"},
		{"zh", "Chinese"},
		{"ja", "Japanese"},
		{"ko", "Korean"},
		{"de", "German"},
		{"fr", "French"},
		{"ru", "Russian"},
		{"pt", "Portuguese"},
		{"es", "Spanish"},
		{"it", "Italian"},
		{"ar", ""}, // Unsupported, returns empty
	}

	provider := newAliyunProvider(&ProviderConfig{})

	for _, tt := range tests {
		result := provider.mapLanguage(tt.isoCode)
		if result != tt.expected {
			t.Errorf("mapLanguage(%q) = %q, want %q", tt.isoCode, result, tt.expected)
		}
	}
}
