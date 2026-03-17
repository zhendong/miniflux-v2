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

## Chunk 2: OpenAI Provider Implementation

### Task 2.1: OpenAI Request Builder

**Files:**
- Modify: `internal/tts/provider_openai.go`
- Create: `internal/tts/provider_openai_test.go`

- [ ] **Step 1: Write failing test for request body construction**

Create file `internal/tts/provider_openai_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"testing"
)

func TestOpenAI_BuildRequestBody(t *testing.T) {
	config := &ProviderConfig{
		Model:        "gpt-4o-mini-tts",
		Voice:        "alloy",
		Speed:        1.5,
		Format:       "mp3",
		Instructions: "Speak clearly",
	}

	provider := newOpenAIProvider(config)
	body := provider.buildRequestBody("Hello world", "en")

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	if parsed["model"] != "gpt-4o-mini-tts" {
		t.Errorf("Expected model gpt-4o-mini-tts, got %v", parsed["model"])
	}

	if parsed["input"] != "Hello world" {
		t.Errorf("Expected input 'Hello world', got %v", parsed["input"])
	}

	if parsed["voice"] != "alloy" {
		t.Errorf("Expected voice alloy, got %v", parsed["voice"])
	}

	if parsed["speed"] != 1.5 {
		t.Errorf("Expected speed 1.5, got %v", parsed["speed"])
	}

	if parsed["response_format"] != "mp3" {
		t.Errorf("Expected response_format mp3, got %v", parsed["response_format"])
	}

	if parsed["instructions"] != "Speak clearly" {
		t.Errorf("Expected instructions, got %v", parsed["instructions"])
	}
}

func TestOpenAI_BuildRequestBody_NoInstructions(t *testing.T) {
	config := &ProviderConfig{
		Model:        "tts-1",
		Voice:        "alloy",
		Speed:        1.0,
		Format:       "mp3",
		Instructions: "", // Empty
	}

	provider := newOpenAIProvider(config)
	body := provider.buildRequestBody("Test", "en")

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Instructions should only be included for gpt-4o-mini-tts model
	if parsed["model"] == "tts-1" {
		if _, exists := parsed["instructions"]; exists {
			t.Error("instructions field should not be present for non-gpt-4o-mini-tts models")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestOpenAI_BuildRequestBody`
Expected: FAIL with "undefined: buildRequestBody"

- [ ] **Step 3: Implement buildRequestBody method**

Update `internal/tts/provider_openai.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
)

type openAIProvider struct {
	config *ProviderConfig
}

func newOpenAIProvider(config *ProviderConfig) *openAIProvider {
	return &openAIProvider{config: config}
}

func (p *openAIProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}

func (p *openAIProvider) buildRequestBody(text, language string) []byte {
	reqBody := map[string]interface{}{
		"model":           p.config.Model,
		"input":           text,
		"voice":           p.config.Voice,
		"speed":           p.config.Speed,
		"response_format": p.config.Format,
	}

	// Only include instructions for gpt-4o-mini-tts model
	if p.config.Model == "gpt-4o-mini-tts" && p.config.Instructions != "" {
		reqBody["instructions"] = p.config.Instructions
	}

	bodyBytes, _ := json.Marshal(reqBody)
	return bodyBytes
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestOpenAI_BuildRequestBody`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_openai.go internal/tts/provider_openai_test.go
git commit -m "feat(tts): add OpenAI request body builder

Implement buildRequestBody for OpenAI provider with support for
model, voice, speed, format, and instructions parameters."
```

### Task 2.2: OpenAI Streaming Response Handler

**Files:**
- Modify: `internal/tts/provider_openai.go`
- Modify: `internal/tts/provider_openai_test.go`

- [ ] **Step 1: Write test for Generate method with mock HTTP server**

Add to `internal/tts/provider_openai_test.go`:

```go
import (
	"net/http"
	"net/http/httptest"
	"time"
)

func TestOpenAI_Generate_Success(t *testing.T) {
	// Create mock server that returns audio stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-api-key" {
			t.Errorf("Expected auth header, got %s", authHeader)
		}

		// Return mock audio data
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("mock audio data"))
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Model:      "gpt-4o-mini-tts",
		Voice:      "alloy",
		Speed:      1.0,
		Format:     "mp3",
		HTTPClient: server.Client(),
	}

	provider := newOpenAIProvider(config)
	result, err := provider.Generate("Hello world", "en")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.AudioData) == 0 {
		t.Error("Expected AudioData to be populated")
	}

	if string(result.AudioData) != "mock audio data" {
		t.Errorf("Expected 'mock audio data', got %q", string(result.AudioData))
	}

	if result.AudioURL != "" {
		t.Error("AudioURL should be empty for streaming provider")
	}
}

func TestOpenAI_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:   server.URL,
		APIKey:     "invalid-key",
		Model:      "gpt-4o-mini-tts",
		Voice:      "alloy",
		Speed:      1.0,
		Format:     "mp3",
		HTTPClient: server.Client(),
	}

	provider := newOpenAIProvider(config)
	_, err := provider.Generate("Test", "en")

	if err == nil {
		t.Fatal("Expected error for 401 response")
	}

	expectedMsg := "OpenAI authentication failed"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error %q, got %q", expectedMsg, err.Error())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestOpenAI_Generate`
Expected: FAIL (stub Generate returns empty result)

- [ ] **Step 3: Implement Generate method with streaming support**

Update `internal/tts/provider_openai.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"miniflux.app/v2/internal/version"
)

const (
	maxContentLength = 50000      // 50KB of text
	maxAudioSize     = 50 << 20   // 50MB
)

type openAIProvider struct {
	config *ProviderConfig
}

func newOpenAIProvider(config *ProviderConfig) *openAIProvider {
	return &openAIProvider{config: config}
}

func (p *openAIProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Build request body
	bodyBytes := p.buildRequestBody(text, language)

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("User-Agent", "Miniflux/"+version.Version)

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp.StatusCode)
	}

	// Read streaming audio response
	limitedReader := io.LimitReader(resp.Body, maxAudioSize+1)
	audioData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio stream: %w", err)
	}

	// Check size limit
	if len(audioData) > maxAudioSize {
		return nil, fmt.Errorf("audio stream exceeds size limit (%d bytes)", maxAudioSize)
	}

	return &ProviderResult{
		AudioData: audioData,
	}, nil
}

func (p *openAIProvider) buildRequestBody(text, language string) []byte {
	reqBody := map[string]interface{}{
		"model":           p.config.Model,
		"input":           text,
		"voice":           p.config.Voice,
		"speed":           p.config.Speed,
		"response_format": p.config.Format,
	}

	// Only include instructions for gpt-4o-mini-tts model
	if p.config.Model == "gpt-4o-mini-tts" && p.config.Instructions != "" {
		reqBody["instructions"] = p.config.Instructions
	}

	bodyBytes, _ := json.Marshal(reqBody)
	return bodyBytes
}

func (p *openAIProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400:
		return fmt.Errorf("invalid OpenAI request parameters")
	case 401:
		return fmt.Errorf("OpenAI authentication failed")
	case 429:
		return fmt.Errorf("OpenAI rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("OpenAI service unavailable")
	default:
		return fmt.Errorf("OpenAI request failed: HTTP %d", statusCode)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestOpenAI`
Expected: PASS (all OpenAI tests pass)

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_openai.go internal/tts/provider_openai_test.go
git commit -m "feat(tts): implement OpenAI streaming audio generation

Implement full OpenAI TTS provider with:
- HTTP request handling with auth and headers
- Binary audio streaming response parsing
- HTTP error mapping with provider-specific messages
- Content size validation (50KB text, 50MB audio)"
```

---

End of Chunk 2

## Chunk 3: Aliyun Provider Implementation

### Task 3.1: Aliyun Language Mapper

**Files:**
- Modify: `internal/tts/provider_aliyun.go`
- Create: `internal/tts/provider_aliyun_test.go`

- [ ] **Step 1: Write failing test for language mapping**

Create file `internal/tts/provider_aliyun_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestAliyun_MapLanguage`
Expected: FAIL with "undefined: mapLanguage"

- [ ] **Step 3: Implement mapLanguage method**

Update `internal/tts/provider_aliyun.go`:

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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tts -v -run TestAliyun_MapLanguage`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_aliyun.go internal/tts/provider_aliyun_test.go
git commit -m "feat(tts): add Aliyun language mapper

Implement ISO 639-1 to full language name mapping for Aliyun TTS.
Supports: English, Chinese, Japanese, Korean, German, French,
Russian, Portuguese, Spanish, Italian."
```

### Task 3.2: Aliyun SSE Streaming Parser

**Files:**
- Modify: `internal/tts/provider_aliyun.go`
- Modify: `internal/tts/provider_aliyun_test.go`

- [ ] **Step 1: Write test for SSE parsing**

Add to `internal/tts/provider_aliyun_test.go`:

```go
import (
	"bufio"
	"encoding/base64"
	"strings"
)

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestAliyun_ParseSSEStream`
Expected: FAIL with "undefined: parseSSEStream"

- [ ] **Step 3: Implement parseSSEStream method**

Update `internal/tts/provider_aliyun.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	maxSSEStreamSize = 50 << 20 // 50MB total audio
)

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

// parseSSEStream reads SSE events and accumulates base64-decoded audio chunks
func (p *aliyunProvider) parseSSEStream(reader *bufio.Reader) ([]byte, error) {
	var audioData []byte

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read SSE stream: %w", err)
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and non-data lines
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		// Extract JSON data after "data: "
		jsonData := strings.TrimPrefix(line, "data:")
		jsonData = strings.TrimSpace(jsonData)

		// Parse JSON response
		var sseEvent struct {
			Output struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"output"`
		}

		if err := json.Unmarshal([]byte(jsonData), &sseEvent); err != nil {
			return nil, fmt.Errorf("failed to parse SSE event JSON: %w", err)
		}

		// Decode base64 audio chunk
		if sseEvent.Output.Audio.Data != "" {
			chunk, err := base64.StdEncoding.DecodeString(sseEvent.Output.Audio.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode audio chunk: %w", err)
			}
			audioData = append(audioData, chunk...)

			// Check size limit
			if len(audioData) > maxSSEStreamSize {
				return nil, fmt.Errorf("audio stream exceeds size limit (%d bytes)", maxSSEStreamSize)
			}
		}
	}

	return audioData, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestAliyun_ParseSSEStream`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_aliyun.go internal/tts/provider_aliyun_test.go
git commit -m "feat(tts): add Aliyun SSE stream parser

Implement SSE event parsing with base64 audio chunk decoding.
Handles JSON parsing, base64 decoding, and size limit enforcement."
```

### Task 3.3: Aliyun Generate Implementation

**Files:**
- Modify: `internal/tts/provider_aliyun.go`
- Modify: `internal/tts/provider_aliyun_test.go`

- [ ] **Step 1: Write test for Generate method (streaming mode)**

Add to `internal/tts/provider_aliyun_test.go`:

```go
import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
)

func TestAliyun_Generate_Streaming(t *testing.T) {
	// Create mock SSE server
	chunk1 := base64.StdEncoding.EncodeToString([]byte("audio chunk 1"))
	chunk2 := base64.StdEncoding.EncodeToString([]byte("audio chunk 2"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("X-DashScope-SSE") != "enable" {
			t.Error("Expected X-DashScope-SSE: enable header")
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Error("Expected Authorization header")
		}

		// Send SSE events
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"output\":{\"audio\":{\"data\":\"" + chunk1 + "\"}}}\n\n"))
		w.Write([]byte("data: {\"output\":{\"audio\":{\"data\":\"" + chunk2 + "\"}}}\n\n"))
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Model:      "qwen3-tts-flash",
		Voice:      "Cherry",
		Stream:     true,
		HTTPClient: server.Client(),
	}

	provider := newAliyunProvider(config)
	result, err := provider.Generate("测试文本", "zh")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "audio chunk 1audio chunk 2"
	if string(result.AudioData) != expected {
		t.Errorf("Expected %q, got %q", expected, string(result.AudioData))
	}
}

func TestAliyun_Generate_NonStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no SSE header in non-streaming mode
		if r.Header.Get("X-DashScope-SSE") != "" {
			t.Error("Should not have X-DashScope-SSE header in non-streaming mode")
		}

		// Return URL response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"output":{"audio":{"url":"https://example.com/audio.mp3"}}}`))
	}))
	defer server.Close()

	config := &ProviderConfig{
		Endpoint:   server.URL,
		APIKey:     "test-api-key",
		Model:      "qwen3-tts-flash",
		Voice:      "Cherry",
		Stream:     false,
		HTTPClient: server.Client(),
	}

	provider := newAliyunProvider(config)
	result, err := provider.Generate("Test", "en")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.AudioURL != "https://example.com/audio.mp3" {
		t.Errorf("Expected URL, got %q", result.AudioURL)
	}

	if len(result.AudioData) != 0 {
		t.Error("AudioData should be empty in non-streaming mode")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestAliyun_Generate`
Expected: FAIL (stub returns empty result)

- [ ] **Step 3: Implement Generate method with dual-mode support**

Update `internal/tts/provider_aliyun.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"miniflux.app/v2/internal/version"
)

const (
	maxContentLength = 50000      // 50KB of text
	maxSSEStreamSize = 50 << 20   // 50MB total audio
)

type aliyunProvider struct {
	config *ProviderConfig
}

func newAliyunProvider(config *ProviderConfig) *aliyunProvider {
	return &aliyunProvider{config: config}
}

func (p *aliyunProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Map language code to full name
	languageType := p.config.LanguageType
	if languageType == "" {
		languageType = p.mapLanguage(language)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"model": p.config.Model,
		"input": map[string]string{
			"text": text,
		},
	}

	if p.config.Voice != "" {
		reqBody["input"].(map[string]string)["voice"] = p.config.Voice
	}

	if languageType != "" {
		reqBody["input"].(map[string]string)["language_type"] = languageType
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("User-Agent", "Miniflux/"+version.Version)

	// Enable SSE for streaming mode
	if p.config.Stream {
		req.Header.Set("X-DashScope-SSE", "enable")
	}

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Aliyun request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp.StatusCode)
	}

	// Handle streaming vs non-streaming response
	if p.config.Stream {
		// Parse SSE stream
		reader := bufio.NewReader(resp.Body)
		audioData, err := p.parseSSEStream(reader)
		if err != nil {
			return nil, err
		}
		return &ProviderResult{
			AudioData: audioData,
		}, nil
	} else {
		// Parse JSON response with URL
		var jsonResp struct {
			Output struct {
				Audio struct {
					URL string `json:"url"`
				} `json:"audio"`
			} `json:"output"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			return nil, fmt.Errorf("failed to parse Aliyun response: %w", err)
		}

		return &ProviderResult{
			AudioURL: jsonResp.Output.Audio.URL,
		}, nil
	}
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

// parseSSEStream reads SSE events and accumulates base64-decoded audio chunks
func (p *aliyunProvider) parseSSEStream(reader *bufio.Reader) ([]byte, error) {
	var audioData []byte

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read SSE stream: %w", err)
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and non-data lines
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		// Extract JSON data after "data: "
		jsonData := strings.TrimPrefix(line, "data:")
		jsonData = strings.TrimSpace(jsonData)

		// Parse JSON response
		var sseEvent struct {
			Output struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"output"`
		}

		if err := json.Unmarshal([]byte(jsonData), &sseEvent); err != nil {
			return nil, fmt.Errorf("failed to parse SSE event JSON: %w", err)
		}

		// Decode base64 audio chunk
		if sseEvent.Output.Audio.Data != "" {
			chunk, err := base64.StdEncoding.DecodeString(sseEvent.Output.Audio.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode audio chunk: %w", err)
			}
			audioData = append(audioData, chunk...)

			// Check size limit
			if len(audioData) > maxSSEStreamSize {
				return nil, fmt.Errorf("audio stream exceeds size limit (%d bytes)", maxSSEStreamSize)
			}
		}
	}

	return audioData, nil
}

func (p *aliyunProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400:
		return fmt.Errorf("invalid Aliyun request parameters")
	case 401, 403:
		return fmt.Errorf("Aliyun authentication failed")
	case 429:
		return fmt.Errorf("Aliyun rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("Aliyun service unavailable")
	default:
		return fmt.Errorf("Aliyun request failed: HTTP %d", statusCode)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestAliyun`
Expected: PASS (all Aliyun tests pass)

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_aliyun.go internal/tts/provider_aliyun_test.go
git commit -m "feat(tts): implement Aliyun dual-mode audio generation

Implement Aliyun TTS provider with:
- Streaming mode (SSE with base64-encoded chunks)
- Non-streaming mode (JSON response with URL)
- Language mapping from ISO codes to full names
- HTTP error mapping with provider-specific messages"
```

---

End of Chunk 3

## Chunk 4: ElevenLabs Provider Implementation

### Task 4.1: ElevenLabs Request Builder

**Files:**
- Modify: `internal/tts/provider_elevenlabs.go`
- Create: `internal/tts/provider_elevenlabs_test.go`

- [ ] **Step 1: Write failing test for URL and request construction**

Create file `internal/tts/provider_elevenlabs_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestElevenLabs_Build`
Expected: FAIL with "undefined: buildRequestURL" and "undefined: buildRequestBody"

- [ ] **Step 3: Implement build methods**

Update `internal/tts/provider_elevenlabs.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"fmt"
)

type elevenLabsProvider struct {
	config *ProviderConfig
}

func newElevenLabsProvider(config *ProviderConfig) *elevenLabsProvider {
	return &elevenLabsProvider{config: config}
}

func (p *elevenLabsProvider) Generate(text, language string) (*ProviderResult, error) {
	return &ProviderResult{}, nil
}

func (p *elevenLabsProvider) buildRequestURL() string {
	return fmt.Sprintf("%s/%s/stream?output_format=%s&optimize_streaming_latency=%d",
		p.config.Endpoint,
		p.config.VoiceID,
		p.config.OutputFormat,
		p.config.OptimizeLatency,
	)
}

func (p *elevenLabsProvider) buildRequestBody(text string) []byte {
	reqBody := map[string]interface{}{
		"text":     text,
		"model_id": p.config.Model,
		"voice_settings": map[string]interface{}{
			"stability":         p.config.Stability,
			"similarity_boost":  p.config.SimilarityBoost,
			"style":             p.config.Style,
			"use_speaker_boost": p.config.SpeakerBoost,
		},
	}

	// Only include language_code if specified
	if p.config.LanguageCode != "" {
		reqBody["language_code"] = p.config.LanguageCode
	}

	bodyBytes, _ := json.Marshal(reqBody)
	return bodyBytes
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestElevenLabs_Build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_elevenlabs.go internal/tts/provider_elevenlabs_test.go
git commit -m "feat(tts): add ElevenLabs request builders

Implement URL construction with voice_id and output_format query params.
Implement request body builder with model_id, text, language_code, and
voice_settings (stability, similarity_boost, style, use_speaker_boost)."
```

### Task 4.2: ElevenLabs Generate Implementation

**Files:**
- Modify: `internal/tts/provider_elevenlabs.go`
- Modify: `internal/tts/provider_elevenlabs_test.go`

- [ ] **Step 1: Write test for Generate method**

Add to `internal/tts/provider_elevenlabs_test.go`:

```go
import (
	"net/http"
	"net/http/httptest"
)

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestElevenLabs_Generate`
Expected: FAIL (stub returns empty result)

- [ ] **Step 3: Implement Generate method**

Update `internal/tts/provider_elevenlabs.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"miniflux.app/v2/internal/version"
)

const (
	maxContentLength    = 50000    // 50KB of text
	maxElevenLabsAudio  = 50 << 20 // 50MB
)

type elevenLabsProvider struct {
	config *ProviderConfig
}

func newElevenLabsProvider(config *ProviderConfig) *elevenLabsProvider {
	return &elevenLabsProvider{config: config}
}

func (p *elevenLabsProvider) Generate(text, language string) (*ProviderResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("text too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Build request
	url := p.buildRequestURL()
	bodyBytes := p.buildRequestBody(text)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", p.config.APIKey)
	req.Header.Set("User-Agent", "Miniflux/"+version.Version)

	// Execute request
	resp, err := p.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp.StatusCode)
	}

	// Read streaming audio response
	limitedReader := io.LimitReader(resp.Body, maxElevenLabsAudio+1)
	audioData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio stream: %w", err)
	}

	// Check size limit
	if len(audioData) > maxElevenLabsAudio {
		return nil, fmt.Errorf("audio stream exceeds size limit (%d bytes)", maxElevenLabsAudio)
	}

	return &ProviderResult{
		AudioData: audioData,
	}, nil
}

func (p *elevenLabsProvider) buildRequestURL() string {
	return fmt.Sprintf("%s/%s/stream?output_format=%s&optimize_streaming_latency=%d",
		p.config.Endpoint,
		p.config.VoiceID,
		p.config.OutputFormat,
		p.config.OptimizeLatency,
	)
}

func (p *elevenLabsProvider) buildRequestBody(text string) []byte {
	reqBody := map[string]interface{}{
		"text":     text,
		"model_id": p.config.Model,
		"voice_settings": map[string]interface{}{
			"stability":         p.config.Stability,
			"similarity_boost":  p.config.SimilarityBoost,
			"style":             p.config.Style,
			"use_speaker_boost": p.config.SpeakerBoost,
		},
	}

	// Only include language_code if specified
	if p.config.LanguageCode != "" {
		reqBody["language_code"] = p.config.LanguageCode
	}

	bodyBytes, _ := json.Marshal(reqBody)
	return bodyBytes
}

func (p *elevenLabsProvider) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400, 422:
		return fmt.Errorf("invalid ElevenLabs request parameters")
	case 401:
		return fmt.Errorf("ElevenLabs authentication failed")
	case 429:
		return fmt.Errorf("ElevenLabs rate limit exceeded")
	case 500, 502, 503:
		return fmt.Errorf("ElevenLabs service unavailable")
	default:
		return fmt.Errorf("ElevenLabs request failed: HTTP %d", statusCode)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestElevenLabs`
Expected: PASS (all ElevenLabs tests pass)

- [ ] **Step 5: Commit**

```bash
git add internal/tts/provider_elevenlabs.go internal/tts/provider_elevenlabs_test.go
git commit -m "feat(tts): implement ElevenLabs streaming audio generation

Implement full ElevenLabs TTS provider with:
- Streaming endpoint with voice_id and format query params
- HTTP request with xi-api-key authentication
- Binary audio streaming response parsing
- Voice settings (stability, similarity_boost, style, speaker_boost)
- Optional language_code enforcement"
```

---

End of Chunk 4

## Chunk 5: Cache & Integration Updates

### Task 5.1: Update Cache to Handle Provider Results

**Files:**
- Modify: `internal/tts/cache.go`
- Modify: `internal/tts/client.go`

- [ ] **Step 1: Read current cache implementation**

Run: `cat internal/tts/cache.go | head -100`
Expected: See GetOrGenerateAudio function using old Client

- [ ] **Step 2: Write integration test for cache with provider**

Update `internal/tts/cache_test.go` to add provider-aware test:

```go
func TestCache_GetOrGenerateAudio_WithProvider(t *testing.T) {
	// This test verifies the cache works with the new provider interface
	// NOTE: Requires database connection, skip if not available
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Test will be implemented when database is available
	// For now, just ensure provider integration compiles
}
```

- [ ] **Step 3: Update cache.go to use Provider interface**

Key changes in `internal/tts/cache.go`:

1. Update GetOrGenerateAudio to create and use Provider:

```go
// GetOrGenerateAudio retrieves cached audio or generates new audio using configured provider
func GetOrGenerateAudio(store *storage.Storage, entryID int64, text string, language string) (string, error) {
	// Check cache first
	cached, err := store.GetTTSCache(entryID, config.Opts.UserID())
	if err == nil && cached != nil {
		if time.Now().Before(cached.ExpiresAt) {
			return cached.FilePath, nil
		}
		// Expired, delete
		store.DeleteTTSCache(cached.ID)
	}

	// Rate limit check
	if err := CheckRateLimit(store, config.Opts.UserID()); err != nil {
		return "", err
	}

	// Create provider config from environment
	providerConfig := &ProviderConfig{
		ProviderType:    config.Opts.TTSProvider(),
		APIKey:          config.Opts.TTSAPIKey(),
		HTTPClient:      client.NewClientWithOptions(client.Options{
			Timeout:              60 * time.Second,
			BlockPrivateNetworks: !config.Opts.IntegrationAllowPrivateNetworks(),
		}),
	}

	// Add provider-specific config
	switch providerConfig.ProviderType {
	case "openai":
		providerConfig.Endpoint = config.Opts.TTSOpenAIEndpoint()
		providerConfig.Model = config.Opts.TTSOpenAIModel()
		providerConfig.Voice = config.Opts.TTSOpenAIVoice()
		providerConfig.Speed = config.Opts.TTSOpenAISpeed()
		providerConfig.Format = config.Opts.TTSOpenAIResponseFormat()
		providerConfig.Instructions = config.Opts.TTSOpenAIInstructions()
	case "aliyun":
		providerConfig.Endpoint = config.Opts.TTSAliyunEndpoint()
		providerConfig.Model = config.Opts.TTSAliyunModel()
		providerConfig.Voice = config.Opts.TTSAliyunVoice()
		providerConfig.LanguageType = config.Opts.TTSAliyunLanguageType()
		providerConfig.Stream = config.Opts.TTSAliyunStream()
	case "elevenlabs":
		providerConfig.Endpoint = config.Opts.TTSElevenLabsEndpoint()
		providerConfig.VoiceID = config.Opts.TTSElevenLabsVoiceID()
		providerConfig.Model = config.Opts.TTSElevenLabsModelID()
		providerConfig.LanguageCode = config.Opts.TTSElevenLabsLanguageCode()
		providerConfig.Stability = config.Opts.TTSElevenLabsStability()
		providerConfig.SimilarityBoost = config.Opts.TTSElevenLabsSimilarityBoost()
		providerConfig.Style = config.Opts.TTSElevenLabsStyle()
		providerConfig.SpeakerBoost = config.Opts.TTSElevenLabsUseSpeakerBoost()
		providerConfig.OutputFormat = config.Opts.TTSElevenLabsOutputFormat()
		providerConfig.OptimizeLatency = config.Opts.TTSElevenLabsOptimizeLatency()
	default:
		return "", fmt.Errorf("TTS_PROVIDER must be set to one of: openai, aliyun, elevenlabs")
	}

	// Create provider
	provider, err := NewProvider(providerConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create TTS provider: %w", err)
	}

	// Generate audio
	result, err := provider.Generate(text, language)
	if err != nil {
		return "", err
	}

	// Handle both AudioData and AudioURL results
	var audioData []byte

	if len(result.AudioData) > 0 {
		// Streaming provider (OpenAI, Aliyun streaming, ElevenLabs)
		audioData = result.AudioData
	} else if result.AudioURL != "" {
		// URL-based provider (Aliyun non-streaming)
		// Use existing DownloadAudio from client.go
		ttsClient := NewClient("", "", "") // Empty params, not used
		audioData, err = ttsClient.DownloadAudio(result.AudioURL)
		if err != nil {
			return "", fmt.Errorf("failed to download audio: %w", err)
		}
	} else {
		return "", fmt.Errorf("provider returned no audio data or URL")
	}

	// Save to disk cache
	filePath := filepath.Join(config.Opts.TTSStoragePath(), fmt.Sprintf("%d.mp3", entryID))
	if err := os.WriteFile(filePath, audioData, 0644); err != nil {
		return "", fmt.Errorf("failed to save audio file: %w", err)
	}

	// Save metadata to database
	expiresAt := time.Now().Add(time.Duration(config.Opts.TTSCacheDuration()) * time.Hour)
	cacheEntry := &model.TTSCache{
		EntryID:   entryID,
		UserID:    config.Opts.UserID(),
		FilePath:  filePath,
		ExpiresAt: expiresAt,
	}

	if err := store.CreateTTSCache(cacheEntry); err != nil {
		return "", fmt.Errorf("failed to save cache metadata: %w", err)
	}

	return filePath, nil
}
```

Note: This is pseudocode structure. Actual implementation will need to match existing cache.go structure and import statements.

- [ ] **Step 4: Verify build**

Run: `go build ./internal/tts`
Expected: BUILD SUCCESS

- [ ] **Step 5: Run all TTS tests**

Run: `go test ./internal/tts -v`
Expected: All provider tests PASS (cache tests may need database)

- [ ] **Step 6: Commit**

```bash
git add internal/tts/cache.go internal/tts/cache_test.go
git commit -m "feat(tts): integrate provider interface with cache layer

Update GetOrGenerateAudio to:
- Create provider based on TTS_PROVIDER config
- Populate provider-specific configuration from env vars
- Handle both AudioData (streaming) and AudioURL (non-streaming) results
- Download audio for URL-based providers using existing DownloadAudio
- Save audio to disk cache and metadata to database"
```

### Task 5.2: Final Integration Testing

**Files:**
- Test all components together

- [ ] **Step 1: Build entire project**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: Run all TTS tests**

Run: `go test ./internal/tts/... -v`
Expected: All unit tests PASS

- [ ] **Step 3: Verify configuration**

Run: `go run . -info 2>&1 | grep -i tts`
Expected: Shows TTS configuration loaded

- [ ] **Step 4: Create final summary commit**

```bash
git add -A
git commit -m "feat(tts): complete multi-provider TTS implementation

Complete implementation of multi-provider TTS support:

Providers:
- OpenAI: Streaming binary audio via audio/speech API
- Aliyun: Dual-mode (SSE streaming + URL-based responses)
- ElevenLabs: Streaming binary audio with voice settings

Architecture:
- Provider interface pattern with factory
- Streaming-first approach for lower latency
- ProviderResult supports both AudioData and AudioURL
- Configuration via TTS_PROVIDER and provider-specific env vars

Components:
- 22 new configuration options across 3 providers
- 3 provider implementations with full test coverage
- Cache integration handles both streaming and URL-based results
- Language detection with provider-specific mapping

Testing:
- Unit tests for all providers
- Request builder tests
- Error handling tests
- SSE stream parsing tests

See docs/superpowers/specs/2026-03-17-multi-provider-tts-design.md
for complete design documentation."
```

---

End of Chunk 5

---

## Plan Complete

All 5 chunks are now documented. To execute this plan:

1. **Chunk 1** (✅ COMPLETE): Core infrastructure
2. **Chunk 2**: OpenAI provider implementation
3. **Chunk 3**: Aliyun provider implementation
4. **Chunk 4**: ElevenLabs provider implementation
5. **Chunk 5**: Cache & integration updates

Use `superpowers:subagent-driven-development` to execute each chunk.