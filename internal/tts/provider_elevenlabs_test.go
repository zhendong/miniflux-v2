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

func TestElevenLabs_BuildRequestURL(t *testing.T) {
	config := &ProviderConfig{
		Endpoint:     "https://api.elevenlabs.io/v1/text-to-speech",
		VoiceID:      "voice-abc-123",
		OutputFormat: "mp3_44100_128",
	}

	provider := newElevenLabsProvider(config)
	url := provider.buildRequestURL()

	expectedURL := "https://api.elevenlabs.io/v1/text-to-speech/voice-abc-123/stream?output_format=mp3_44100_128&optimize_streaming_latency=0"
	if url != expectedURL {
		t.Errorf("Expected URL %q, got %q", expectedURL, url)
	}
}

func TestElevenLabs_BuildRequestBody(t *testing.T) {
	config := &ProviderConfig{
		Model:           "eleven_multilingual_v2",
		LanguageCode:    "en",
		Stability:       0.6,
		SimilarityBoost: 0.8,
		Style:           0.2,
		SpeakerBoost:    true,
	}

	provider := newElevenLabsProvider(config)
	body, err := provider.buildRequestBody("Hello world")
	if err != nil {
		t.Fatalf("buildRequestBody failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	if parsed["text"] != "Hello world" {
		t.Errorf("Expected text 'Hello world', got %v", parsed["text"])
	}

	if parsed["model_id"] != "eleven_multilingual_v2" {
		t.Errorf("Expected model_id, got %v", parsed["model_id"])
	}

	if parsed["language_code"] != "en" {
		t.Errorf("Expected language_code en, got %v", parsed["language_code"])
	}

	settings, ok := parsed["voice_settings"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected voice_settings object")
	}

	if settings["stability"] != 0.6 {
		t.Errorf("Expected stability 0.6, got %v", settings["stability"])
	}

	if settings["similarity_boost"] != 0.8 {
		t.Errorf("Expected similarity_boost 0.8, got %v", settings["similarity_boost"])
	}

	if settings["style"] != 0.2 {
		t.Errorf("Expected style 0.2, got %v", settings["style"])
	}

	if settings["use_speaker_boost"] != true {
		t.Errorf("Expected use_speaker_boost true, got %v", settings["use_speaker_boost"])
	}
}

func TestElevenLabs_BuildRequestBody_NoLanguageCode(t *testing.T) {
	config := &ProviderConfig{
		Model:        "eleven_multilingual_v2",
		LanguageCode: "", // Empty
	}

	provider := newElevenLabsProvider(config)
	body, err := provider.buildRequestBody("Test")
	if err != nil {
		t.Fatalf("buildRequestBody failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// language_code should not be present if empty
	if _, exists := parsed["language_code"]; exists {
		t.Error("language_code should not be present when empty")
	}
}

func TestElevenLabs_BuildRequestURL_SpecialCharacters(t *testing.T) {
	config := &ProviderConfig{
		Endpoint:        "https://api.elevenlabs.io/v1/text-to-speech",
		VoiceID:         "voice with spaces",
		OutputFormat:    "mp3&test=value",
		OptimizeLatency: 0,
	}

	provider := newElevenLabsProvider(config)
	url := provider.buildRequestURL()

	// VoiceID should be path-escaped
	if !strings.Contains(url, "voice%20with%20spaces") {
		t.Errorf("VoiceID not properly escaped: %s", url)
	}

	// OutputFormat should be query-escaped
	if !strings.Contains(url, "mp3%26test%3Dvalue") {
		t.Errorf("OutputFormat not properly escaped: %s", url)
	}
}

func TestElevenLabs_BuildRequestURL_NonZeroLatency(t *testing.T) {
	config := &ProviderConfig{
		Endpoint:        "https://api.elevenlabs.io/v1/text-to-speech",
		VoiceID:         "voice-123",
		OutputFormat:    "mp3_44100_128",
		OptimizeLatency: 3,
	}

	provider := newElevenLabsProvider(config)
	url := provider.buildRequestURL()

	if !strings.Contains(url, "optimize_streaming_latency=3") {
		t.Errorf("Expected optimize_streaming_latency=3 in URL: %s", url)
	}
}

func TestElevenLabs_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		apiKey := r.Header.Get("xi-api-key")
		if apiKey != "test-api-key" {
			t.Errorf("Expected xi-api-key header, got %s", apiKey)
		}

		// Verify URL contains voice_id
		if !strings.Contains(r.URL.Path, "voice-123") {
			t.Errorf("Expected voice_id in URL, got %s", r.URL.Path)
		}

		// Return mock audio data
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("mock elevenlabs audio"))
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:        server.URL,
		APIKey:          "test-api-key",
		VoiceID:         "voice-123",
		Model:           "eleven_multilingual_v2",
		OutputFormat:    "mp3_44100_128",
		OptimizeLatency: 0,
		Stability:       0.5,
		SimilarityBoost: 0.75,
		Style:           0.0,
		SpeakerBoost:    true,
		HTTPClient:      server.Client(),
	}

	provider := newElevenLabsProvider(config)
	result, err := provider.Generate("Hello world", "en")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.AudioData) == 0 {
		t.Error("Expected AudioData to be populated")
	}

	if string(result.AudioData) != "mock elevenlabs audio" {
		t.Errorf("Expected 'mock elevenlabs audio', got %q", string(result.AudioData))
	}
}

func TestElevenLabs_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:     server.URL,
		APIKey:       "invalid-key",
		VoiceID:      "voice-123",
		OutputFormat: "mp3_44100_128",
		HTTPClient:   server.Client(),
	}

	provider := newElevenLabsProvider(config)
	_, err := provider.Generate("Test", "en")

	if err == nil {
		t.Fatal("Expected error for 401 response")
	}

	expectedMsg := "ElevenLabs authentication failed"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error %q, got %q", expectedMsg, err.Error())
	}
}
