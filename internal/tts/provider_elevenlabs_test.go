// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
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
	body := provider.buildRequestBody("Hello world")

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
	body := provider.buildRequestBody("Test")

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// language_code should not be present if empty
	if _, exists := parsed["language_code"]; exists {
		t.Error("language_code should not be present when empty")
	}
}
