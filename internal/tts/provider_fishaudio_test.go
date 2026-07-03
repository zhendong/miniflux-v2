// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFishAudio_BuildRequestBody(t *testing.T) {
	config := &ProviderConfig{
		Temperature: 0.7,
		TopP:        0.8,
		Speed:       1.2,
		Format:      "mp3",
	}

	provider := newFishAudioProvider(config)
	body, err := provider.buildRequestBody("Hello world")
	if err != nil {
		t.Fatalf("Failed to build request body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	if parsed["text"] != "Hello world" {
		t.Errorf("Expected text 'Hello world', got %v", parsed["text"])
	}

	if parsed["temperature"] != 0.7 {
		t.Errorf("Expected temperature 0.7, got %v", parsed["temperature"])
	}

	if parsed["top_p"] != 0.8 {
		t.Errorf("Expected top_p 0.8, got %v", parsed["top_p"])
	}

	if parsed["format"] != "mp3" {
		t.Errorf("Expected format mp3, got %v", parsed["format"])
	}

	prosody, ok := parsed["prosody"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected prosody object, got %v", parsed["prosody"])
	}
	if prosody["speed"] != 1.2 {
		t.Errorf("Expected prosody.speed 1.2, got %v", prosody["speed"])
	}

	if _, exists := parsed["reference_id"]; exists {
		t.Error("reference_id field should not be present when empty")
	}
}

func TestFishAudio_BuildRequestBody_WithReferenceID(t *testing.T) {
	config := &ProviderConfig{
		ReferenceID: "voice-123",
		Format:      "mp3",
	}

	provider := newFishAudioProvider(config)
	body, err := provider.buildRequestBody("Test")
	if err != nil {
		t.Fatalf("Failed to build request body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	if parsed["reference_id"] != "voice-123" {
		t.Errorf("Expected reference_id voice-123, got %v", parsed["reference_id"])
	}
}

func TestFishAudio_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-api-key" {
			t.Errorf("Expected auth header, got %s", authHeader)
		}

		modelHeader := r.Header.Get("model")
		if modelHeader != "s2.1-pro-free" {
			t.Errorf("Expected model header s2.1-pro-free, got %s", modelHeader)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("mock audio data"))
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Model:      "s2.1-pro-free",
		Format:     "mp3",
		HTTPClient: server.Client(),
	}

	provider := newFishAudioProvider(config)
	result, err := provider.Generate("Hello world", "en")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(result.AudioData) != "mock audio data" {
		t.Errorf("Expected 'mock audio data', got %q", string(result.AudioData))
	}

	if result.AudioURL != "" {
		t.Error("AudioURL should be empty for streaming provider")
	}
}

func TestFishAudio_Generate_HTTPErrors(t *testing.T) {
	tests := []struct {
		statusCode  int
		expectedMsg string
	}{
		{401, "Fish Audio authentication failed"},
		{402, "Fish Audio payment required (free tier limit reached or model requires billing)"},
		{422, "invalid Fish Audio request parameters"},
		{429, "Fish Audio rate limit exceeded"},
		{500, "Fish Audio service unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedMsg, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			config := &ProviderConfig{
				Endpoint:   server.URL,
				APIKey:     "test-api-key",
				HTTPClient: server.Client(),
			}

			provider := newFishAudioProvider(config)
			_, err := provider.Generate("Test", "en")

			if err == nil {
				t.Fatalf("Expected error for %d response", tt.statusCode)
			}

			if err.Error() != tt.expectedMsg {
				t.Errorf("Expected error %q, got %q", tt.expectedMsg, err.Error())
			}
		})
	}
}

func TestFishAudio_Generate_TextTooLarge(t *testing.T) {
	config := &ProviderConfig{
		Endpoint: "http://example.com",
		APIKey:   "test-api-key",
	}

	provider := newFishAudioProvider(config)
	_, err := provider.Generate(strings.Repeat("a", maxContentLength+1), "en")

	if err == nil {
		t.Fatal("Expected error for text exceeding max content length")
	}
}
