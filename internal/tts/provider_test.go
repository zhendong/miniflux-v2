// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"
	"time"
)

func TestProviderResult_HasAudioData(t *testing.T) {
	result := &ProviderResult{
		AudioData: []byte("test audio"),
	}

	if len(result.AudioData) == 0 {
		t.Error("Expected AudioData to be populated")
	}
}

func TestProviderResult_HasAudioURL(t *testing.T) {
	expiresAt := time.Now().Add(1 * time.Hour)
	result := &ProviderResult{
		AudioURL:  "https://example.com/audio.mp3",
		ExpiresAt: expiresAt,
	}

	if result.AudioURL == "" {
		t.Error("Expected AudioURL to be populated")
	}

	if result.ExpiresAt.IsZero() {
		t.Error("Expected ExpiresAt to be set")
	}
}

func TestNewProvider_UnsupportedProvider(t *testing.T) {
	config := &ProviderConfig{
		ProviderType: "invalid",
	}

	_, err := NewProvider(config)
	if err == nil {
		t.Fatal("Expected error for unsupported provider")
	}

	expected := "unsupported TTS provider: invalid"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	config := &ProviderConfig{
		ProviderType: "openai",
		APIKey:       "test-key",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
}

func TestNewProvider_Aliyun(t *testing.T) {
	config := &ProviderConfig{
		ProviderType: "aliyun",
		APIKey:       "test-key",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
}

func TestNewProvider_ElevenLabs(t *testing.T) {
	config := &ProviderConfig{
		ProviderType: "elevenlabs",
		APIKey:       "test-key",
		VoiceID:      "test-voice-id",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}
}
