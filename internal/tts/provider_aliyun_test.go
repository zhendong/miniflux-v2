// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bufio"
	"encoding/base64"
	"strings"
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

func TestAliyun_ParseSSEStream(t *testing.T) {
	// Mock SSE stream with base64-encoded audio chunks
	chunk1 := base64.StdEncoding.EncodeToString([]byte("audio chunk 1"))
	chunk2 := base64.StdEncoding.EncodeToString([]byte("audio chunk 2"))

	sseData := "data: {\"output\":{\"audio\":{\"data\":\"" + chunk1 + "\"}}}\n\n" +
		"data: {\"output\":{\"audio\":{\"data\":\"" + chunk2 + "\"}}}\n\n"

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	audioData, err := provider.parseSSEStream(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "audio chunk 1audio chunk 2"
	if string(audioData) != expected {
		t.Errorf("Expected %q, got %q", expected, string(audioData))
	}
}

func TestAliyun_ParseSSEStream_InvalidJSON(t *testing.T) {
	sseData := "data: invalid json\n\n"

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	_, err := provider.parseSSEStream(reader)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestAliyun_ParseSSEStream_InvalidBase64(t *testing.T) {
	sseData := "data: {\"output\":{\"audio\":{\"data\":\"invalid!!!base64\"}}}\n\n"

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	_, err := provider.parseSSEStream(reader)
	if err == nil {
		t.Fatal("Expected error for invalid base64")
	}
}
