# Multi-Provider TTS Support Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add support for OpenAI, Aliyun (Qwen TTS), and ElevenLabs TTS providers with streaming-first architecture.

**Architecture:** Provider interface pattern with factory for creating provider implementations. Each provider handles its own request formatting, authentication, and response parsing. Streaming responses by default for lower latency.

**Tech Stack:** Go 1.x, existing miniflux HTTP client, SSE parsing for Aliyun, base64 decoding for Aliyun streaming.

**Spec Reference:** `docs/superpowers/specs/2026-03-17-multi-provider-tts-design.md`

---

## File Structure

### New Files to Create

**Provider Core:**
- `internal/tts/provider.go` - Provider interface, ProviderResult type, ProviderConfig type, factory function
- `internal/tts/provider_test.go` - Factory tests, config validation tests

**Provider Implementations:**
- `internal/tts/provider_openai.go` - OpenAI TTS implementation (streaming binary audio)
- `internal/tts/provider_openai_test.go` - OpenAI provider unit tests
- `internal/tts/provider_aliyun.go` - Aliyun/Qwen TTS implementation (SSE streaming + URL mode)
- `internal/tts/provider_aliyun_test.go` - Aliyun provider unit tests
- `internal/tts/provider_elevenlabs.go` - ElevenLabs TTS implementation (streaming binary audio)
- `internal/tts/provider_elevenlabs_test.go` - ElevenLabs provider unit tests

### Files to Modify

**Configuration:**
- `internal/config/options.go` - Add TTS_PROVIDER and all provider-specific config options

**Core TTS:**
- `internal/tts/client.go` - Simplify to wrap provider, keep DownloadAudio for Aliyun non-streaming
- `internal/tts/cache.go` - Update GetOrGenerateAudio to handle both AudioData and AudioURL
- `internal/tts/cache_test.go` - Update tests for new provider interface

**Models (if needed):**
- `internal/model/tts.go` - May need ProviderConfig struct (check during implementation)

---

## Chunk 1: Core Infrastructure

### Task 1.1: Provider Interface and Types

**Files:**
- Create: `internal/tts/provider.go`

- [ ] **Step 1: Write the failing test for ProviderResult**

Create file `internal/tts/provider_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestProviderResult`
Expected: FAIL with "undefined: ProviderResult"

- [ ] **Step 3: Write Provider interface and types**

Create file `internal/tts/provider.go`:

```go
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
	switch config.ProviderType {
	case "openai":
		return newOpenAIProvider(config), nil
	case "aliyun":
		return newAliyunProvider(config), nil
	case "elevenlabs":
		return newElevenLabsProvider(config), nil
	default:
		return nil, fmt.Errorf("unsupported TTS provider: %s (supported: openai, aliyun, elevenlabs)", config.ProviderType)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tts -v -run TestProviderResult`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider.go internal/tts/provider_test.go
git commit -m "feat(tts): add Provider interface and types

Add core Provider interface for pluggable TTS implementations.
Includes ProviderResult for both streaming and URL-based responses,
and ProviderConfig for provider configuration."
```

### Task 1.2: Factory Function Tests

**Files:**
- Modify: `internal/tts/provider_test.go`

- [ ] **Step 1: Write failing test for unsupported provider**

Add to `internal/tts/provider_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it passes** (already implemented in previous task)

Run: `go test ./internal/tts -v -run TestNewProvider_UnsupportedProvider`
Expected: PASS

- [ ] **Step 3: Write test for each provider type requiring implementations**

Add to `internal/tts/provider_test.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/tts -v -run TestNewProvider`
Expected: FAIL with "undefined: newOpenAIProvider", "undefined: newAliyunProvider", "undefined: newElevenLabsProvider"

- [ ] **Step 5: Create stub provider implementations**

These will be properly implemented in subsequent chunks. For now, create minimal stubs to make tests pass.

Create file `internal/tts/provider_openai.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

type openAIProvider struct {
	config *ProviderConfig
}

func newOpenAIProvider(config *ProviderConfig) *openAIProvider {
	return &openAIProvider{config: config}
}

func (p *openAIProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}
```

Create file `internal/tts/provider_aliyun.go`:

```go
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
```

Create file `internal/tts/provider_elevenlabs.go`:

```go
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
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestNewProvider`
Expected: PASS (all factory tests pass with stub implementations)

- [ ] **Step 7: Commit**

```bash
git add internal/tts/provider_test.go internal/tts/provider_openai.go internal/tts/provider_aliyun.go internal/tts/provider_elevenlabs.go
git commit -m "feat(tts): add provider factory tests and stubs

Add factory function tests for all three providers.
Create stub implementations to be filled in subsequent tasks."
```

### Task 1.3: Configuration Options

**Files:**
- Modify: `internal/config/options.go`

- [ ] **Step 1: Add TTS_PROVIDER configuration option**

Find the TTS configuration section in `internal/config/options.go` (around line 660-700 in the options map) and add after `TTS_ENABLED`:

```go
"TTS_PROVIDER": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
	validator:         validateTTSProvider,
},
```

Note: The config structure uses `parsedStringValue`, `rawValue`, `valueType`, and `validator` fields (not `description`, `defaultValue`, etc.). The `parsedStringValue` field will be set during config parsing based on the environment variable value or the default in `rawValue`.

- [ ] **Step 2: Add validation function for TTS_PROVIDER**

Add this function near other validation functions in `internal/config/options.go`:

```go
func validateTTSProvider(value string) error {
	if value == "" {
		return nil // Optional when TTS_ENABLED is false
	}

	validProviders := map[string]bool{
		"openai":     true,
		"aliyun":     true,
		"elevenlabs": true,
	}

	if !validProviders[value] {
		return fmt.Errorf("invalid TTS provider %q (must be: openai, aliyun, or elevenlabs)", value)
	}

	return nil
}
```

- [ ] **Step 3: Add TTSProvider() accessor method**

Add this method near other TTS accessor methods in `internal/config/options.go`:

```go
func (c *configOptions) TTSProvider() string {
	return c.options["TTS_PROVIDER"].parsedStringValue
}
```

- [ ] **Step 4: Verify no references to deprecated config before removal**

Check if deprecated configs are referenced elsewhere:

Run: `grep -r "TTSEndpointURL\|TTSVoice" internal/ --exclude-dir=config`
Expected: Will likely find references in `internal/tts/client.go`

Note: `internal/tts/client.go` currently uses these methods but will be updated in Chunk 4 to use the new provider system. We remove the config definitions now because new provider-specific configs replace them. The client.go file will compile but won't function correctly until Chunk 4 updates it - this is acceptable for incremental implementation.

- [ ] **Step 4b: Remove deprecated configuration options**

Find and delete from `internal/config/options.go`:
- `TTS_ENDPOINT_URL` config entry
- `TTS_VOICE` config entry
- `TTSEndpointURL()` accessor method
- `TTSVoice()` accessor method

- [ ] **Step 5: Add OpenAI-specific configuration**

Note: The codebase doesn't have `floatType` support. For float values (like `TTS_OPENAI_SPEED`), use `stringType` and parse with `strconv.ParseFloat` in the accessor method.

Add these options in the TTS section of `internal/config/options.go`:

```go
"TTS_OPENAI_ENDPOINT": {
	parsedStringValue: "https://api.openai.com/v1/audio/speech",
	rawValue:          "https://api.openai.com/v1/audio/speech",
	valueType:         stringType,
},
"TTS_OPENAI_MODEL": {
	parsedStringValue: "gpt-4o-mini-tts",
	rawValue:          "gpt-4o-mini-tts",
	valueType:         stringType,
},
"TTS_OPENAI_VOICE": {
	parsedStringValue: "alloy",
	rawValue:          "alloy",
	valueType:         stringType,
},
"TTS_OPENAI_SPEED": {
	parsedStringValue: "1.0",
	rawValue:          "1.0",
	valueType:         stringType, // Will parse as float in accessor
},
"TTS_OPENAI_RESPONSE_FORMAT": {
	parsedStringValue: "mp3",
	rawValue:          "mp3",
	valueType:         stringType,
},
"TTS_OPENAI_INSTRUCTIONS": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
},
```

- [ ] **Step 6: Add OpenAI accessor methods**

First, ensure `strconv` is imported at the top of the file. Add to the import block if not present:

```go
import (
	// ... existing imports ...
	"strconv"
)
```

Then add these accessor methods:

```go
func (c *configOptions) TTSOpenAIEndpoint() string {
	return c.options["TTS_OPENAI_ENDPOINT"].parsedStringValue
}

func (c *configOptions) TTSOpenAIModel() string {
	return c.options["TTS_OPENAI_MODEL"].parsedStringValue
}

func (c *configOptions) TTSOpenAIVoice() string {
	return c.options["TTS_OPENAI_VOICE"].parsedStringValue
}

func (c *configOptions) TTSOpenAISpeed() float64 {
	// Parse float from string since codebase doesn't have floatType
	value, err := strconv.ParseFloat(c.options["TTS_OPENAI_SPEED"].parsedStringValue, 64)
	if err != nil {
		// Log warning about invalid value (add slog import if needed)
		// slog.Warn("invalid TTS_OPENAI_SPEED value, using default", "error", err)
		return 1.0 // Default value on parse error
	}
	return value
}

func (c *configOptions) TTSOpenAIResponseFormat() string {
	return c.options["TTS_OPENAI_RESPONSE_FORMAT"].parsedStringValue
}

func (c *configOptions) TTSOpenAIInstructions() string {
	return c.options["TTS_OPENAI_INSTRUCTIONS"].parsedStringValue
}
```

- [ ] **Step 7: Add Aliyun-specific configuration**

Add these options (using same structure as OpenAI):

```go
"TTS_ALIYUN_ENDPOINT": {
	parsedStringValue: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
	rawValue:          "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
	valueType:         stringType,
},
"TTS_ALIYUN_MODEL": {
	parsedStringValue: "qwen3-tts-flash",
	rawValue:          "qwen3-tts-flash",
	valueType:         stringType,
},
"TTS_ALIYUN_VOICE": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
},
"TTS_ALIYUN_LANGUAGE_TYPE": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
},
"TTS_ALIYUN_STREAM": {
	parsedBoolValue: true,
	rawValue:        "true",
	valueType:       boolType,
},
```

- [ ] **Step 8: Add Aliyun accessor methods**

```go
func (c *configOptions) TTSAliyunEndpoint() string {
	return c.options["TTS_ALIYUN_ENDPOINT"].parsedStringValue
}

func (c *configOptions) TTSAliyunModel() string {
	return c.options["TTS_ALIYUN_MODEL"].parsedStringValue
}

func (c *configOptions) TTSAliyunVoice() string {
	return c.options["TTS_ALIYUN_VOICE"].parsedStringValue
}

func (c *configOptions) TTSAliyunLanguageType() string {
	return c.options["TTS_ALIYUN_LANGUAGE_TYPE"].parsedStringValue
}

func (c *configOptions) TTSAliyunStream() bool {
	return c.options["TTS_ALIYUN_STREAM"].parsedBoolValue
}
```

- [ ] **Step 9: Add ElevenLabs-specific configuration**

Add these options (float values use stringType like OpenAI):

```go
"TTS_ELEVENLABS_ENDPOINT": {
	parsedStringValue: "https://api.elevenlabs.io/v1/text-to-speech",
	rawValue:          "https://api.elevenlabs.io/v1/text-to-speech",
	valueType:         stringType,
},
"TTS_ELEVENLABS_VOICE_ID": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
},
"TTS_ELEVENLABS_MODEL_ID": {
	parsedStringValue: "eleven_multilingual_v2",
	rawValue:          "eleven_multilingual_v2",
	valueType:         stringType,
},
"TTS_ELEVENLABS_LANGUAGE_CODE": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
},
"TTS_ELEVENLABS_STABILITY": {
	parsedStringValue: "0.5",
	rawValue:          "0.5",
	valueType:         stringType, // Will parse as float in accessor
},
"TTS_ELEVENLABS_SIMILARITY_BOOST": {
	parsedStringValue: "0.75",
	rawValue:          "0.75",
	valueType:         stringType, // Will parse as float in accessor
},
"TTS_ELEVENLABS_STYLE": {
	parsedStringValue: "0",
	rawValue:          "0",
	valueType:         stringType, // Will parse as float in accessor
},
"TTS_ELEVENLABS_USE_SPEAKER_BOOST": {
	parsedBoolValue: true,
	rawValue:        "true",
	valueType:       boolType,
},
"TTS_ELEVENLABS_OUTPUT_FORMAT": {
	parsedStringValue: "mp3_44100_128",
	rawValue:          "mp3_44100_128",
	valueType:         stringType,
},
"TTS_ELEVENLABS_OPTIMIZE_LATENCY": {
	parsedIntValue: 0,
	rawValue:       "0",
	valueType:      intType,
},
```

- [ ] **Step 10: Add ElevenLabs accessor methods**

```go
func (c *configOptions) TTSElevenLabsEndpoint() string {
	return c.options["TTS_ELEVENLABS_ENDPOINT"].parsedStringValue
}

func (c *configOptions) TTSElevenLabsVoiceID() string {
	return c.options["TTS_ELEVENLABS_VOICE_ID"].parsedStringValue
}

func (c *configOptions) TTSElevenLabsModelID() string {
	return c.options["TTS_ELEVENLABS_MODEL_ID"].parsedStringValue
}

func (c *configOptions) TTSElevenLabsLanguageCode() string {
	return c.options["TTS_ELEVENLABS_LANGUAGE_CODE"].parsedStringValue
}

func (c *configOptions) TTSElevenLabsStability() float64 {
	value, err := strconv.ParseFloat(c.options["TTS_ELEVENLABS_STABILITY"].parsedStringValue, 64)
	if err != nil {
		// Log warning about invalid value (add slog import if needed)
		// slog.Warn("invalid TTS_ELEVENLABS_STABILITY value, using default", "error", err)
		return 0.5 // Default value on parse error
	}
	return value
}

func (c *configOptions) TTSElevenLabsSimilarityBoost() float64 {
	value, err := strconv.ParseFloat(c.options["TTS_ELEVENLABS_SIMILARITY_BOOST"].parsedStringValue, 64)
	if err != nil {
		// Log warning about invalid value (add slog import if needed)
		// slog.Warn("invalid TTS_ELEVENLABS_SIMILARITY_BOOST value, using default", "error", err)
		return 0.75 // Default value on parse error
	}
	return value
}

func (c *configOptions) TTSElevenLabsStyle() float64 {
	value, err := strconv.ParseFloat(c.options["TTS_ELEVENLABS_STYLE"].parsedStringValue, 64)
	if err != nil {
		// Log warning about invalid value (add slog import if needed)
		// slog.Warn("invalid TTS_ELEVENLABS_STYLE value, using default", "error", err)
		return 0 // Default value on parse error
	}
	return value
}

func (c *configOptions) TTSElevenLabsUseSpeakerBoost() bool {
	return c.options["TTS_ELEVENLABS_USE_SPEAKER_BOOST"].parsedBoolValue
}

func (c *configOptions) TTSElevenLabsOutputFormat() string {
	return c.options["TTS_ELEVENLABS_OUTPUT_FORMAT"].parsedStringValue
}

func (c *configOptions) TTSElevenLabsOptimizeLatency() int {
	return c.options["TTS_ELEVENLABS_OPTIMIZE_LATENCY"].parsedIntValue
}
```

- [ ] **Step 11: Test configuration parsing**

Run: `go build ./internal/config`
Expected: BUILD SUCCESS

- [ ] **Step 12: Commit**

```bash
git add internal/config/options.go
git commit -m "feat(config): add multi-provider TTS configuration

Add TTS_PROVIDER option and provider-specific configs:
- OpenAI: endpoint, model, voice, speed, format, instructions
- Aliyun: endpoint, model, voice, language_type, stream
- ElevenLabs: endpoint, voice_id, model_id, language_code, voice settings

Remove deprecated TTS_ENDPOINT_URL and TTS_VOICE options."
```

---

End of Chunk 1
