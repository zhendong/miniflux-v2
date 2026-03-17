// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"net/http"
	"time"

	"miniflux.app/v2/internal/http/client"
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

// ConfigLoader is an interface for loading TTS configuration.
// This allows the TTS package to access config values without depending on the config package.
type ConfigLoader interface {
	TTSProvider() string
	TTSAPIKey() string
	TTSOpenAIEndpoint() string
	TTSOpenAIModel() string
	TTSOpenAIVoice() string
	TTSOpenAISpeed() float64
	TTSOpenAIResponseFormat() string
	TTSOpenAIInstructions() string
	TTSAliyunEndpoint() string
	TTSAliyunModel() string
	TTSAliyunVoice() string
	TTSAliyunLanguageType() string
	TTSAliyunStream() bool
	TTSElevenLabsEndpoint() string
	TTSElevenLabsVoiceID() string
	TTSElevenLabsModelID() string
	TTSElevenLabsLanguageCode() string
	TTSElevenLabsStability() float64
	TTSElevenLabsSimilarityBoost() float64
	TTSElevenLabsStyle() float64
	TTSElevenLabsUseSpeakerBoost() bool
	TTSElevenLabsOutputFormat() string
	TTSElevenLabsOptimizeLatency() int
	IntegrationAllowPrivateNetworks() bool
}

// NewProviderConfigFromLoader creates a ProviderConfig from a ConfigLoader.
func NewProviderConfigFromLoader(loader ConfigLoader) *ProviderConfig {
	providerType := loader.TTSProvider()

	config := &ProviderConfig{
		ProviderType: providerType,
		APIKey:       loader.TTSAPIKey(),
		HTTPClient: newHTTPClient(
			30*time.Second,
			!loader.IntegrationAllowPrivateNetworks(),
		),
	}

	switch providerType {
	case "openai":
		config.Endpoint = loader.TTSOpenAIEndpoint()
		config.Model = loader.TTSOpenAIModel()
		config.Voice = loader.TTSOpenAIVoice()
		config.Speed = loader.TTSOpenAISpeed()
		config.Format = loader.TTSOpenAIResponseFormat()
		config.Instructions = loader.TTSOpenAIInstructions()

	case "aliyun":
		config.Endpoint = loader.TTSAliyunEndpoint()
		config.Model = loader.TTSAliyunModel()
		config.Voice = loader.TTSAliyunVoice()
		config.LanguageType = loader.TTSAliyunLanguageType()
		config.Stream = loader.TTSAliyunStream()

	case "elevenlabs":
		config.Endpoint = loader.TTSElevenLabsEndpoint()
		config.Model = loader.TTSElevenLabsModelID()
		config.VoiceID = loader.TTSElevenLabsVoiceID()
		config.LanguageCode = loader.TTSElevenLabsLanguageCode()
		config.Stability = loader.TTSElevenLabsStability()
		config.SimilarityBoost = loader.TTSElevenLabsSimilarityBoost()
		config.Style = loader.TTSElevenLabsStyle()
		config.SpeakerBoost = loader.TTSElevenLabsUseSpeakerBoost()
		config.OutputFormat = loader.TTSElevenLabsOutputFormat()
		config.OptimizeLatency = loader.TTSElevenLabsOptimizeLatency()
	}

	return config
}

// newHTTPClient creates a new HTTP client with the specified timeout and security settings.
func newHTTPClient(timeout time.Duration, blockPrivateNetworks bool) *http.Client {
	return client.NewClientWithOptions(client.Options{
		Timeout:              timeout,
		BlockPrivateNetworks: blockPrivateNetworks,
	})
}
