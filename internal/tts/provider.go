// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"net/http"
	"time"
)

// Provider is the interface that all TTS providers must implement.
type Provider interface {
	// Generate converts text to speech and returns audio data or URL.
	// language is the ISO 639-1 language code (e.g., "en", "zh").
	Generate(text, language string) (*ProviderResult, error)
}

// ProviderResult contains the result from a TTS provider.
type ProviderResult struct {
	// AudioData contains the audio bytes for streaming providers.
	// Populated by OpenAI, Aliyun (streaming mode), and ElevenLabs.
	AudioData []byte

	// AudioURL contains the download URL for URL-based providers.
	// Populated by Aliyun (non-streaming mode).
	AudioURL string

	// ExpiresAt indicates when the AudioURL expires (if applicable).
	ExpiresAt time.Time
}

// ProviderConfig contains configuration for creating a provider.
type ProviderConfig struct {
	// Common config
	APIKey      string
	HTTPClient  *http.Client

	// Provider-specific config
	ProviderType string
	Endpoint     string
	Model        string
	Voice        string

	// OpenAI-specific
	Speed        float64
	Format       string
	Instructions string

	// Aliyun-specific
	LanguageType string
	Stream       bool

	// ElevenLabs-specific
	VoiceID          string
	LanguageCode     string
	Stability        float64
	SimilarityBoost  float64
	Style            float64
	SpeakerBoost     bool
	OutputFormat     string
	OptimizeLatency  int
}

// NewProvider creates a new TTS provider based on the provider type.
func NewProvider(config *ProviderConfig) (Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	switch config.ProviderType {
	case "openai":
		return newOpenAIProvider(config), nil
	case "aliyun":
		return newAliyunProvider(config), nil
	case "elevenlabs":
		return newElevenLabsProvider(config), nil
	default:
		return nil, fmt.Errorf("unsupported TTS provider: %s", config.ProviderType)
	}
}
