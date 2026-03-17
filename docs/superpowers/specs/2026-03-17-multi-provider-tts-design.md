# Multi-Provider TTS Support Design

**Date:** 2026-03-17
**Status:** Approved
**Author:** Claude Sonnet 4.5

## Overview

This design extends the Miniflux TTS feature to support multiple Text-to-Speech providers: OpenAI, Aliyun (Qwen TTS), and ElevenLabs. The current implementation uses a generic HTTP approach with a single endpoint configuration. The new design uses a provider abstraction pattern to support provider-specific APIs, authentication methods, and response formats.

## Goals

1. Support three TTS providers: OpenAI, Aliyun (Qwen TTS), and ElevenLabs
2. Support both streaming (binary audio) and URL-based response patterns
3. Use streaming by default for lower latency and simpler data flow
4. Provide provider-specific configuration for advanced features
5. Maintain backward compatibility through clear migration path
6. Ensure all providers work with the existing cache and rate-limiting infrastructure

## Non-Goals

- Supporting multiple providers simultaneously (one provider per instance)
- Auto-detecting provider from endpoint URL
- Backward compatibility with old generic configuration

## Architecture

### Provider Interface

The core abstraction is a `Provider` interface that encapsulates all provider-specific behavior:

```go
type Provider interface {
    Generate(text, language string) (*ProviderResult, error)
}

type ProviderResult struct {
    AudioData []byte      // For streaming providers (direct audio bytes)
    AudioURL  string      // For URL-based providers (download link)
    ExpiresAt time.Time   // When the URL/audio expires (if applicable)
}
```

**Key Design Decisions:**

- `ProviderResult` supports both streaming (AudioData) and URL-based (AudioURL) patterns
- Each provider implementation is in a separate file for isolation and testability
- Factory pattern creates the appropriate provider based on `TTS_PROVIDER` config
- Cache layer handles both result types transparently

### File Structure

```
internal/tts/
├── provider.go               # Provider interface and factory
├── provider_openai.go        # OpenAI implementation
├── provider_aliyun.go        # Aliyun/Qwen implementation
├── provider_elevenlabs.go    # ElevenLabs implementation
├── provider_test.go          # Factory and interface tests
├── provider_openai_test.go   # OpenAI provider tests
├── provider_aliyun_test.go   # Aliyun provider tests
├── provider_elevenlabs_test.go # ElevenLabs provider tests
├── client.go                 # Simplified wrapper + shared HTTP utilities
├── cache.go                  # Updated to handle both AudioData and AudioURL
└── (existing files remain)
```

## Configuration

### Core Configuration (All Providers)

- `TTS_ENABLED` - Boolean (existing, unchanged)
- `TTS_PROVIDER` - String: "openai", "aliyun", or "elevenlabs" (**new, required**)
- `TTS_API_KEY` / `TTS_API_KEY_FILE` - Authentication token (existing, all providers use this)
- `TTS_DEFAULT_LANGUAGE` - Fallback language (existing, unchanged)
- `TTS_STORAGE_PATH`, `TTS_CACHE_DURATION`, `TTS_RATE_LIMIT_PER_HOUR` - Cache/limits (existing, unchanged)

### OpenAI-Specific Configuration

- `TTS_OPENAI_ENDPOINT` - Default: `https://api.openai.com/v1/audio/speech`
- `TTS_OPENAI_MODEL` - Default: `gpt-4o-mini-tts` (options: `gpt-4o-mini-tts`, `tts-1`, `tts-1-hd`)
- `TTS_OPENAI_VOICE` - Default: `alloy` (13 voices: alloy, coral, echo, nova, onyx, sage, shimmer, marin, cedar, etc.)
- `TTS_OPENAI_SPEED` - Default: `1.0` (range: 0.25-4.0)
- `TTS_OPENAI_RESPONSE_FORMAT` - Default: `mp3` (options: mp3, opus, aac, flac, wav, pcm)
- `TTS_OPENAI_INSTRUCTIONS` - Optional (only for gpt-4o-mini-tts, controls accent, tone, speed, etc.)

**Language Support:** 80+ languages following Whisper model (English, Spanish, French, Chinese, Japanese, Arabic, etc.)

### Aliyun-Specific Configuration

- `TTS_ALIYUN_ENDPOINT` - Default: `https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation`
- `TTS_ALIYUN_MODEL` - Default: `qwen3-tts-flash` (options: `qwen3-tts-flash`, `qwen3-tts-instruct-flash`, `qwen-tts`)
- `TTS_ALIYUN_VOICE` - Required, no default (options: Cherry, Ethan, Chelsie, Serena, Dylan, Jada, Sunny, etc.)
- `TTS_ALIYUN_LANGUAGE_TYPE` - Optional, auto-detected (options: Chinese, English, Japanese, Korean, German, French, Russian, Portuguese, Spanish, Italian)
- `TTS_ALIYUN_STREAM` - Default: `true` (boolean, set to false for URL-based responses)

**Language Support:** 10 major languages via full language names (Chinese, English, Japanese, Korean, German, French, Russian, Portuguese, Spanish, Italian)

### ElevenLabs-Specific Configuration

- `TTS_ELEVENLABS_ENDPOINT` - Default: `https://api.elevenlabs.io/v1/text-to-speech`
- `TTS_ELEVENLABS_VOICE_ID` - Required, no default (unique voice identifier)
- `TTS_ELEVENLABS_MODEL_ID` - Default: `eleven_multilingual_v2`
- `TTS_ELEVENLABS_LANGUAGE_CODE` - Optional (ISO 639-1 code to enforce language, e.g., "en", "es", "zh")
- `TTS_ELEVENLABS_STABILITY` - Default: `0.5` (range: 0-1)
- `TTS_ELEVENLABS_SIMILARITY_BOOST` - Default: `0.75` (range: 0-1)
- `TTS_ELEVENLABS_STYLE` - Default: `0` (range: 0-1)
- `TTS_ELEVENLABS_SPEED` - Default: `1.0` (speech speed multiplier)
- `TTS_ELEVENLABS_USE_SPEAKER_BOOST` - Default: `true` (boolean)
- `TTS_ELEVENLABS_OUTPUT_FORMAT` - Default: `mp3_44100_128` (many options: mp3/pcm/opus variants)
- `TTS_ELEVENLABS_OPTIMIZE_LATENCY` - Default: `0` (range: 0-4 for streaming latency optimization)

**Language Support:** Multilingual via ISO 639-1 codes (en, es, fr, de, it, pt, pl, zh, ja, ko, nl, tr, sv, id, fil, etc.)

### Removed/Deprecated Configuration

- `TTS_ENDPOINT_URL` - **Removed** (replaced by provider-specific endpoints)
- `TTS_VOICE` - **Removed** (replaced by provider-specific voice configs)

## Provider Implementations

### OpenAI Provider

**Endpoint:** `https://api.openai.com/v1/audio/speech`

**Request Format (POST):**
```json
{
  "model": "gpt-4o-mini-tts",
  "input": "text content here",
  "voice": "alloy",
  "speed": 1.0,
  "response_format": "mp3",
  "instructions": "Speak with a calm, professional tone"
}
```

**Authentication:** `Authorization: Bearer {api_key}`

**Response:** Binary audio stream via chunked transfer encoding. Content-Type indicates format (e.g., `audio/mpeg`).

**Implementation Notes:**
- Uses streaming response by default (no URL-based mode)
- Reads binary audio directly into `ProviderResult.AudioData`
- `instructions` parameter only sent when model is `gpt-4o-mini-tts`

### Aliyun Provider

**Endpoint:** `https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation`

**Request Format (POST):**
```json
{
  "model": "qwen3-tts-flash",
  "input": {
    "text": "text content here",
    "voice": "Cherry",
    "language_type": "Chinese"
  }
}
```

**Authentication:**
- `Authorization: Bearer {api_key}`
- `Content-Type: application/json`

**Streaming Mode (Default - `TTS_ALIYUN_STREAM=true`):**

Additional header: `X-DashScope-SSE: enable`

Response: Server-Sent Events (SSE) stream with base64-encoded audio chunks:
```
data: {"output":{"audio":{"data":"base64encodedaudiochunk..."}}}

data: {"output":{"audio":{"data":"base64encodedaudiochunk..."}}}
```

**Non-Streaming Mode (`TTS_ALIYUN_STREAM=false`):**

Response:
```json
{
  "output": {
    "audio": {
      "url": "https://...",
      "data": null
    }
  }
}
```

**Implementation Notes:**
- Streaming mode (default): Parse SSE events, decode base64 chunks, accumulate into `ProviderResult.AudioData`
- Non-streaming mode: Extract `output.audio.url` into `ProviderResult.AudioURL`
- Language mapping: Convert detected ISO codes to full names (e.g., "en" → "English", "zh" → "Chinese")

### ElevenLabs Provider

**Endpoint:** `https://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream?output_format={format}`

**Request Format (POST):**
```json
{
  "text": "text content here",
  "model_id": "eleven_multilingual_v2",
  "language_code": "en",
  "voice_settings": {
    "stability": 0.5,
    "similarity_boost": 0.75,
    "style": 0,
    "speed": 1.0,
    "use_speaker_boost": true
  }
}
```

**Authentication:** `xi-api-key: {api_key}` header

**Response:** Binary audio stream (application/octet-stream)

**Implementation Notes:**
- Uses streaming endpoint (with `/stream` suffix)
- Reads binary audio directly into `ProviderResult.AudioData`
- `language_code` parameter is optional; if provided, enforces that language
- URL includes output format as query parameter (e.g., `?output_format=mp3_44100_128`)

## Data Flow

### Request Flow (Streaming-First)

1. **User requests TTS for an entry** → Handler checks cache for existing audio
2. **Cache miss** → Call `GetOrGenerateAudio()`
3. **Factory creates provider** → Based on `TTS_PROVIDER` config
4. **Provider formats request** → With provider-specific params and makes HTTP call
5. **Provider reads streaming response:**
   - **OpenAI:** Reads chunked binary audio stream directly into `ProviderResult.AudioData`
   - **Aliyun** (streaming mode, default): Parses SSE events, decodes base64 audio chunks, accumulates into `ProviderResult.AudioData`
   - **Aliyun** (non-streaming, opt-in): Parses JSON response, extracts URL into `ProviderResult.AudioURL`
   - **ElevenLabs:** Reads binary audio stream directly into `ProviderResult.AudioData`
6. **Cache layer handles result:**
   - If `AudioData` is populated (most common): Write bytes directly to disk cache file
   - If `AudioURL` is populated (Aliyun non-streaming only): Download audio using existing `DownloadAudio()` logic, then save to disk
7. **Return file path to user**

### Provider Factory

```go
func NewProvider(providerType string, config *ProviderConfig) (Provider, error) {
    switch providerType {
    case "openai":
        return NewOpenAIProvider(config), nil
    case "aliyun":
        return NewAliyunProvider(config), nil
    case "elevenlabs":
        return NewElevenLabsProvider(config), nil
    default:
        return nil, fmt.Errorf("unsupported TTS provider: %s", providerType)
    }
}
```

### Language Detection and Mapping

The existing `DetectLanguage()` function remains unchanged. Each provider will:

- **OpenAI:** Use the detected ISO 639-1 language code directly (supports 80+ languages)
- **ElevenLabs:** Use the detected ISO 639-1 code in the `language_code` parameter if configured
- **Aliyun:** Map detected ISO codes to full language names:
  - `en` → `English`
  - `zh` → `Chinese`
  - `ja` → `Japanese`
  - `ko` → `Korean`
  - `de` → `German`
  - `fr` → `French`
  - `ru` → `Russian`
  - `pt` → `Portuguese`
  - `es` → `Spanish`
  - `it` → `Italian`

All providers fall back to `TTS_DEFAULT_LANGUAGE` config if detection fails.

### Benefits of Streaming-First Approach

- **Lower latency:** Audio generation starts immediately
- **Simpler flow:** No separate download step for most providers
- **Less network overhead:** Single request instead of request + download
- **Better resource usage:** Audio written directly to disk without intermediate storage

## Error Handling

### Provider-Specific Error Mapping

Each provider maps HTTP status codes to meaningful errors:

**OpenAI:**
- 400 → "Invalid request parameters"
- 401 → "OpenAI authentication failed - check API key"
- 429 → "OpenAI rate limit exceeded"
- 500/502/503 → "OpenAI service unavailable"

**Aliyun:**
- 400 → "Invalid Aliyun request parameters"
- 401/403 → "Aliyun authentication failed - check API key"
- 429 → "Aliyun rate limit exceeded"
- 500/502/503 → "Aliyun service unavailable"

**ElevenLabs:**
- 400/422 → Parse validation error JSON for detailed field errors
- 401 → "ElevenLabs authentication failed - check API key"
- 429 → "ElevenLabs rate limit exceeded (check quota/character limits)"
- 500/502/503 → "ElevenLabs service unavailable"

### Streaming Error Handling

- **SSE parsing errors (Aliyun):** Wrap with context: "failed to parse SSE event: {error}"
- **Incomplete streams:** "audio stream ended prematurely"
- **Base64 decode errors (Aliyun):** "failed to decode audio chunk: {error}"
- **Network errors during streaming:** Include context about which chunk failed

### Configuration Validation

On startup, validate required configs:

- `TTS_PROVIDER` must be one of: "openai", "aliyun", "elevenlabs"
- Provider-specific required fields:
  - Aliyun: `TTS_ALIYUN_VOICE` must be set
  - ElevenLabs: `TTS_ELEVENLABS_VOICE_ID` must be set
- Return clear error message if required config is missing:
  - Example: "TTS_PROVIDER 'elevenlabs' requires TTS_ELEVENLABS_VOICE_ID to be set"
- Log fatal error and exit if `TTS_ENABLED=true` but `TTS_PROVIDER` is not set

### Text Validation

- Check text length before API call (50KB limit)
- Return user-friendly error: "Entry content too large for TTS ({actual} > {max} characters)"

### Deprecated Configuration Warnings

- If `TTS_ENDPOINT_URL` or `TTS_VOICE` are set, log warning about deprecation with migration instructions
- Example: "TTS_ENDPOINT_URL is deprecated. Please use TTS_PROVIDER and provider-specific endpoint configs. See migration guide."

## Testing Strategy

### Unit Tests for Each Provider

**Test Files:**
- `provider_openai_test.go`
- `provider_aliyun_test.go`
- `provider_elevenlabs_test.go`

**Each provider test suite covers:**

1. **Success case (streaming):** Mock HTTP server returns audio stream, verify `AudioData` is populated correctly
2. **Success case (non-streaming):** For Aliyun with streaming disabled, verify `AudioURL` extraction
3. **Authentication:** Test with missing/invalid API key, verify proper error
4. **HTTP error codes:** Test 400, 401, 429, 500 responses, verify error messages
5. **Text length validation:** Test with oversized text (>50KB), verify rejection before API call
6. **Parameter formatting:** Verify request body has correct structure for each provider
7. **Header validation:** Verify correct auth headers and content-type

### Streaming-Specific Tests

1. **Aliyun SSE parsing:** Mock SSE stream with multiple chunks, verify assembly
2. **Base64 decoding:** Test Aliyun base64 chunk decoding
3. **Chunked transfer encoding:** Test OpenAI/ElevenLabs binary streaming
4. **Incomplete streams:** Test early connection termination, verify error handling
5. **Malformed SSE events:** Test Aliyun with invalid SSE format, verify error

### Factory Tests

**Test File:** `provider_test.go`

1. Verify factory creates correct provider type based on config
2. Test with unsupported provider name, verify error
3. Test with missing required config, verify validation error
4. Test config validation for each provider type

### Cache Integration Tests

**Test File:** `cache_test.go` (extend existing)

1. Update existing tests to work with new Provider interface
2. Test streaming provider flow: `AudioData` → direct file write
3. Test URL provider flow: `AudioURL` → download → file write
4. Verify cache hits skip provider calls
5. Test cache behavior with different providers

### Mock HTTP Servers

All provider tests use `httptest.NewServer` to mock provider APIs:
- Return realistic response formats (binary audio, SSE streams, JSON)
- Simulate various error conditions
- Verify request headers and body structure

## Migration Guide

### Breaking Changes

The following environment variables are **removed**:
- `TTS_ENDPOINT_URL` → Use provider-specific endpoints
- `TTS_VOICE` → Use provider-specific voice configs

### New Required Configuration

- `TTS_PROVIDER` - Must be set to "openai", "aliyun", or "elevenlabs"

### Migration Examples

**For OpenAI users:**
```bash
# Old config:
TTS_ENABLED=true
TTS_ENDPOINT_URL=https://api.openai.com/v1/audio/speech
TTS_API_KEY=sk-...
TTS_VOICE=alloy

# New config:
TTS_ENABLED=true
TTS_PROVIDER=openai
TTS_API_KEY=sk-...
TTS_OPENAI_VOICE=alloy
# Optional: TTS_OPENAI_MODEL=gpt-4o-mini-tts (defaults to this)
```

**For Aliyun/Qwen users:**
```bash
# New config (no old equivalent):
TTS_ENABLED=true
TTS_PROVIDER=aliyun
TTS_API_KEY=sk-...
TTS_ALIYUN_MODEL=qwen3-tts-flash
TTS_ALIYUN_VOICE=Cherry
# Optional: TTS_ALIYUN_STREAM=true (default)
```

**For ElevenLabs users:**
```bash
# New config (no old equivalent):
TTS_ENABLED=true
TTS_PROVIDER=elevenlabs
TTS_API_KEY=...
TTS_ELEVENLABS_VOICE_ID=21m00Tcm4TlvDq8ikWAM
# Optional: TTS_ELEVENLABS_MODEL_ID=eleven_multilingual_v2 (default)
```

### Startup Validation and User Guidance

On startup, the application will:

1. **Check if `TTS_ENABLED=true` but `TTS_PROVIDER` is not set:**
   - Log fatal error: "TTS_PROVIDER must be set when TTS_ENABLED=true. Supported values: openai, aliyun, elevenlabs. See migration guide at [URL]"
   - Exit with non-zero status

2. **Check if deprecated vars (`TTS_ENDPOINT_URL`, `TTS_VOICE`) are set:**
   - Log warning: "TTS_ENDPOINT_URL and TTS_VOICE are deprecated and will be ignored. Please use TTS_PROVIDER and provider-specific configs. See migration guide at [URL]"

3. **Validate provider-specific required configs:**
   - Log clear error if missing: "TTS_PROVIDER 'aliyun' requires TTS_ALIYUN_VOICE to be set"
   - Exit with non-zero status

### Documentation Updates

The following documentation will be updated:

1. **Environment variables reference:**
   - Mark `TTS_ENDPOINT_URL` and `TTS_VOICE` as deprecated/removed
   - Add all new provider-specific variables with descriptions
   - Include examples for each provider

2. **TTS feature guide:**
   - Add section for each provider with setup examples
   - Include voice selection guidance
   - Document language support for each provider

3. **Migration guide:**
   - Dedicated section for users upgrading from previous versions
   - Clear before/after configuration examples
   - Troubleshooting common migration issues

## Implementation Phases

### Phase 1: Core Infrastructure
1. Create Provider interface and ProviderResult types
2. Implement factory pattern
3. Update cache.go to handle both AudioData and AudioURL patterns
4. Add configuration parsing for all new env vars
5. Implement startup validation

### Phase 2: Provider Implementations
1. Implement OpenAI provider with tests
2. Implement Aliyun provider (streaming + non-streaming) with tests
3. Implement ElevenLabs provider with tests
4. Add language mapping utilities

### Phase 3: Integration and Testing
1. Update cache integration tests
2. Add end-to-end tests with all providers
3. Test error handling paths
4. Validate configuration examples

### Phase 4: Documentation and Migration
1. Update environment variable documentation
2. Write migration guide
3. Add provider setup guides
4. Update README with new examples

## Security Considerations

1. **API Key Handling:**
   - All providers use existing `TTS_API_KEY` / `TTS_API_KEY_FILE` mechanism
   - Keys are never logged or exposed in error messages
   - Support for file-based key storage prevents keys in environment

2. **Input Validation:**
   - Text content is validated for length before sending to providers
   - Provider-specific parameter validation prevents injection attacks
   - URL validation for Aliyun non-streaming mode

3. **Network Security:**
   - All providers use HTTPS endpoints by default
   - HTTP client uses existing security settings (timeouts, certificate validation)
   - No private network access (existing `BlockPrivateNetworks: false` preserved)

4. **Error Information Disclosure:**
   - Error messages include provider name but not sensitive configuration
   - API responses are sanitized before logging
   - Stack traces only in debug mode

## Performance Considerations

1. **Streaming Benefits:**
   - Lower memory usage (audio written directly to disk)
   - Faster response to users (no download step)
   - Reduced latency (chunked transfer starts immediately)

2. **Provider-Specific Optimizations:**
   - **OpenAI:** Uses chunked transfer encoding for immediate streaming
   - **Aliyun:** SSE streaming with base64 chunks (slightly higher CPU for decode)
   - **ElevenLabs:** Binary streaming with latency optimization parameter (0-4)

3. **Cache Hit Rate:**
   - Same caching behavior as before (by entry ID and user ID)
   - Streaming vs URL mode doesn't affect cache effectiveness
   - Cache duration configurable via `TTS_CACHE_DURATION`

4. **Rate Limiting:**
   - Existing rate limiter works with all providers
   - Provider-specific rate limits handled via HTTP 429 responses
   - Clear error messages guide users to adjust `TTS_RATE_LIMIT_PER_HOUR`

## Open Questions

None - all design questions resolved during brainstorming phase.

## References

- [OpenAI Text-to-Speech API](https://developers.openai.com/api/docs/guides/text-to-speech)
- [Aliyun Qwen TTS API](https://help.aliyun.com/zh/model-studio/qwen-tts-api)
- [ElevenLabs Text-to-Speech Streaming API](https://elevenlabs.io/docs/api-reference/text-to-speech/stream)
- [DashScope Python SDK - Text-to-Speech](https://deepwiki.com/dashscope/dashscope-sdk-python/4.1-text-to-speech)
- [DashScope Python SDK - Qwen TTS](https://deepwiki.com/dashscope/dashscope-sdk-python/3.2.1-qwen-tts)
