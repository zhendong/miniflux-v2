# Text-to-Speech (TTS) Feature Design

**Date:** 2026-03-16
**Status:** Draft
**Author:** Claude (with user guidance)

## Overview

Add Text-to-Speech functionality to Miniflux, allowing users to listen to feed entries using configurable external TTS services (OpenAI, ElevenLabs, cloud providers, etc.). The feature will cache generated audio locally, provide an inline audio player, and include rate limiting to prevent abuse.

## Goals

- Enable users to listen to entry content (title + body) via TTS
- Support any HTTP-based TTS service through a generic API pattern
- Cache generated audio files locally to reduce API costs and improve performance
- Provide simple, clean UI that fits Miniflux's minimalist design
- Prevent abuse through per-user rate limiting
- Maintain Miniflux's opinionated, lightweight architecture

## Non-Goals

- Built-in TTS engine (external services only)
- Real-time streaming audio generation
- Custom voice training or management
- Transcript/subtitle generation
- Multi-language UI for TTS controls (uses existing i18n system)

## Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Content to read** | Title + Content | Most useful for complete article listening |
| **UI placement** | Both list and detail views | Accessible from both browsing and reading contexts |
| **Audio delivery** | Temporary URL (cached locally) | Multi-device access, simple implementation |
| **Generation strategy** | On-demand with caching | Balances storage and performance |
| **TTS services** | Generic HTTP API pattern | Maximum flexibility, supports any provider |
| **Configuration** | Global admin settings | Simple, aligns with Miniflux philosophy |
| **API contract** | Simple JSON request/response | Clean, predictable integration |
| **Voice/Language** | Auto-detect from feed metadata | Smart defaults, minimal configuration |
| **Cache duration** | Configurable (default 24h) | Admin control over storage/cost tradeoff |
| **Error handling** | Fail fast, no auto-retry | Simple, user-initiated retries |
| **Rate limiting** | In-memory per-user throttle | Prevents abuse, survives restarts |
| **Playback** | Inline `<audio>` player | Keeps users in reading flow |
| **Storage** | Local filesystem | Simple, consistent with favicons |

## Architecture

### Package Structure

```
internal/tts/
├── client.go       # Generic HTTP TTS client
├── cache.go        # Cache lookup/storage logic
├── ratelimit.go    # Per-user request throttling
├── language.go     # Language detection from feed metadata
└── tts.go          # Main orchestrator
```

### Request Flow

```
User clicks TTS button
    ↓
Frontend: handleTTS(entryID)
    ↓
API: GET /v1/entries/{entryID}/tts
    ↓
Check rate limit for user
    ↓
Check cache for existing audio
    ↓
[Cache Hit] → Return cached audio URL
    ↓
[Cache Miss] → Call TTS service
    ↓
Download audio from service URL
    ↓
Save file locally
    ↓
Store cache record in DB
    ↓
Return audio URL to frontend
    ↓
Frontend: Load into <audio> player and play
```

### Integration Points

- **API:** New endpoint `internal/api/entry_tts.go`
- **Storage:** New table `tts_audio_cache` + storage methods
- **Config:** New options in `internal/config/options.go`
- **UI:** Template modifications in `internal/template/templates/views/`
- **Scheduler:** Cleanup job in `internal/worker/scheduler.go`
- **File serving:** New endpoint for serving audio files

## Database Schema

### tts_audio_cache Table

```sql
CREATE TABLE tts_audio_cache (
    id BIGSERIAL PRIMARY KEY,
    entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,  -- Relative path: "tts_audio/{entry_id}_{user_id}_{timestamp}.mp3"
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(entry_id, user_id)
);

CREATE INDEX tts_audio_cache_expires_at_idx ON tts_audio_cache(expires_at);
CREATE INDEX tts_audio_cache_user_id_idx ON tts_audio_cache(user_id);
CREATE INDEX tts_audio_cache_entry_id_idx ON tts_audio_cache(entry_id);
```

**Design notes:**
- `UNIQUE(entry_id, user_id)`: Each user gets their own cache entry
- `ON DELETE CASCADE`: Auto-cleanup when entries/users deleted
- Indexes support fast cleanup queries and user lookups
- Only stores file path, not audio data (keeps DB lean)

## Configuration

### Environment Variables

```bash
# Enable TTS feature
TTS_ENABLED=1

# TTS service endpoint URL
TTS_ENDPOINT_URL=https://api.openai.com/v1/audio/speech

# API key for authentication
TTS_API_KEY=sk-...
TTS_API_KEY_FILE=/path/to/secret

# Voice identifier (passed to TTS service)
TTS_VOICE=alloy

# Default language if feed has no language metadata
TTS_DEFAULT_LANGUAGE=en

# Local storage path for audio files
TTS_STORAGE_PATH=./data/tts_audio

# Cache duration in hours (default: 24)
TTS_CACHE_DURATION=24

# Rate limit: max requests per user per hour
TTS_RATE_LIMIT_PER_HOUR=20
```

### Config Accessor Methods

```go
func (c *configOptions) TTSEnabled() bool
func (c *configOptions) TTSEndpointURL() string
func (c *configOptions) TTSAPIKey() string
func (c *configOptions) TTSVoice() string
func (c *configOptions) TTSDefaultLanguage() string
func (c *configOptions) TTSStoragePath() string
func (c *configOptions) TTSCacheDuration() time.Duration
func (c *configOptions) TTSRateLimitPerHour() int
```

### Secret File Reading

**TTS_API_KEY_FILE** follows Miniflux's existing secret file pattern (same as DATABASE_URL_FILE, ADMIN_PASSWORD_FILE):

**Implementation:**
- File path is read from `TTS_API_KEY_FILE` environment variable
- File is read once at startup during config initialization
- File content (trimmed of whitespace) is stored in `TTS_API_KEY` config option
- File format: Plain text, single line, API key only
- Error handling:
  - File not found → Fatal error, app won't start
  - File not readable → Fatal error (permission denied)
  - Empty file → Fatal error (invalid configuration)
- No hot-reload: Changes require app restart

**Example file:**
```
sk-1234567890abcdef1234567890abcdef
```

**Usage:**
```bash
echo "sk-..." > /run/secrets/tts_api_key
chmod 400 /run/secrets/tts_api_key
export TTS_API_KEY_FILE=/run/secrets/tts_api_key
miniflux
```

## TTS Client Implementation

### HTTP API Contract

**IMPORTANT:** Miniflux requires a TTS service that implements this specific contract. Since most TTS providers (OpenAI, ElevenLabs, etc.) have different APIs and return audio directly rather than URLs, **users must deploy a wrapper service** that translates between Miniflux's contract and their chosen TTS provider.

See "Appendix: Wrapper Service Implementation" for reference implementations.

**Request to TTS wrapper service:**
```http
POST {TTS_ENDPOINT_URL}
Content-Type: application/json
Authorization: Bearer {TTS_API_KEY}

{
  "text": "{title}\n\n{content}",
  "language": "{auto-detected}",
  "voice": "{TTS_VOICE}"
}
```

**Required response from TTS wrapper service:**
```json
{
  "audio_url": "https://tts-service.example.com/audio/xyz123.mp3",
  "expires_at": "2026-03-17T10:00:00Z"
}
```

**Response fields:**
- `audio_url` (string, required): Publicly accessible URL to download the MP3 audio file
- `expires_at` (string, required): ISO 8601 timestamp when the URL expires

**Contract requirements:**
- Audio file must be in MP3 format (`Content-Type: audio/mpeg`)
- Audio URL must be accessible without authentication
- Audio URL must remain valid until `expires_at`
- Wrapper service should return errors as HTTP status codes (4xx/5xx)

### Client Interface

```go
type Client struct {
    endpointURL string
    apiKey      string
    voice       string
    httpClient  *http.Client
}

func NewClient(endpointURL, apiKey, voice string) *Client

func (c *Client) Generate(text string, language string) (*AudioResult, error)

func (c *Client) DownloadAudio(url string) ([]byte, error)

type AudioResult struct {
    AudioURL  string
    ExpiresAt time.Time
}
```

### Client Implementation Details

**Generate() method:**
- HTTP timeout: 30 seconds for TTS service response
- Maximum text length: 50,000 characters (validated before request)
- Content-Type: `application/json`
- Authorization: `Bearer {apiKey}` header

**DownloadAudio() method:**
- HTTP timeout: 60 seconds (larger files need more time)
- Maximum file size: 50MB
- Validates Content-Type is `audio/mpeg` or `audio/mp3`
- Returns raw audio bytes for filesystem storage
- Follows redirects (max 5 hops)
- Does not retry on failure (user can retry manually)

### Error Handling

- **HTTP 400:** Invalid request → return error to user
- **HTTP 401/403:** Authentication failed → log for admin, show error to user
- **HTTP 429:** Rate limit from service → show rate limit message
- **HTTP 500/502/503:** Service unavailable → show error, user can retry
- **Timeout (30s for generate, 60s for download):** Return timeout error
- **Invalid JSON:** Return parse error
- **File too large (>50MB):** Return error, don't download
- **Wrong Content-Type:** Return error (must be audio/mpeg)
- **No automatic retries:** Fail fast, user clicks to retry

## Cache Management

### Cache Logic

```go
func GetOrGenerateAudio(
    store *storage.Storage,
    client *Client,
    entry *model.Entry,
    userID int64,
    cacheDuration time.Duration,
    storagePath string,
) (*AudioResult, error)
```

**Flow:**
1. Acquire lock for this (entry_id, user_id) pair (see Concurrent Request Handling below)
2. Check database for cached entry (entry_id + user_id)
3. If found and not expired, verify file exists → return cached result
4. If cache miss:
   - Validate content length (< 50,000 chars)
   - Detect language from feed metadata
   - Generate audio via TTS service (get temporary URL)
   - Download audio file from URL using client.DownloadAudio()
   - Save locally: `{storagePath}/tts_audio/{entry_id}_{user_id}_{timestamp}.mp3`
   - Create DB cache record with expiry
5. Release lock
6. Return audio file path

### Concurrent Request Handling

To prevent duplicate TTS generation when multiple requests for the same entry arrive simultaneously:

```go
// In internal/tts/cache.go
var (
    generationLocks = make(map[string]*sync.Mutex)
    locksMapMutex   sync.Mutex
)

func getLock(entryID, userID int64) *sync.Mutex {
    key := fmt.Sprintf("%d:%d", entryID, userID)

    locksMapMutex.Lock()
    defer locksMapMutex.Unlock()

    if lock, exists := generationLocks[key]; exists {
        return lock
    }

    lock := &sync.Mutex{}
    generationLocks[key] = lock
    return lock
}
```

**Implementation:**
- GetOrGenerateAudio() acquires lock at start, releases at end
- First request generates audio, subsequent requests wait and get cached result
- Locks are never deleted (acceptable memory overhead for active entries)
- Alternative: Use short-lived locks with cleanup if memory is a concern

### File Storage

- **Directory structure:** `{TTS_STORAGE_PATH}/tts_audio/`
- **Filename format:** `{entry_id}_{user_id}_{timestamp}.mp3`
- **Permissions:** 0755 for directories, 0644 for files
- **Storage location:** Configurable via `TTS_STORAGE_PATH` (default: `./data/tts_audio`)

### Cleanup

**Daily cleanup job (runs at 2 AM):**
1. Query expired cache records (`expires_at < NOW()`)
2. Delete corresponding audio files from filesystem
3. Delete DB records
4. Find orphaned files (files without DB records) and delete
5. Log cleanup statistics

## Rate Limiting

### In-Memory Implementation

```go
type RateLimiter struct {
    mu         sync.RWMutex
    limits     map[int64]*userLimit  // userID → limit info
    maxPerHour int
}

type userLimit struct {
    count       int
    windowStart time.Time
}
```

**Algorithm:**
- Sliding window: Track requests per user within 1-hour windows
- Reset count when window expires (> 1 hour old)
- Background cleanup: Remove stale entries every hour
- Returns `true` if allowed, `false` if limit exceeded

**API Integration:**
- Check `rateLimiter.Allow(userID)` before processing
- Return HTTP 429 if denied
- Include `X-RateLimit-Remaining` header in responses

## API Endpoints

### GET /v1/entries/{entryID}/tts

Generate or retrieve cached TTS audio for an entry.

**Request:**
```http
GET /v1/entries/12345/tts
X-Auth-Token: {user_session_token}
```

**Response (Success):**
```json
HTTP 200 OK

{
  "audio_url": "https://miniflux.example.com/tts/audio/12345_67_1710583200.mp3",
  "expires_at": "2026-03-17T10:00:00Z"
}
```

**Response (Rate Limited):**
```http
HTTP 429 Too Many Requests
X-RateLimit-Remaining: 0

{
  "error_message": "Rate limit exceeded. Please try again later."
}
```

**Response (Error):**
```http
HTTP 500 Internal Server Error

{
  "error_message": "TTS generation failed"
}
```

### GET /tts/audio/{filename}

Serve cached audio files.

**Security:**
- Validate filename (prevent path traversal)
- Parse `entry_id` and `user_id` from filename
- Verify requesting user matches `user_id` in filename
- Verify user still has access to the entry
- Return 403 if unauthorized, 404 if not found

**Response:**
```http
HTTP 200 OK
Content-Type: audio/mpeg
Cache-Control: private, max-age=86400

{binary audio data}
```

## UI Implementation

### Button States

| State | Icon | Style | Behavior |
|-------|------|-------|----------|
| **Ready** | 🔊 | Default border/background | Click to generate audio |
| **Loading** | ⏳... | Orange border, cream bg | Disabled, generating |
| **Cached** | ▶️ | Green border, light green bg | Click to play |
| **Error** | 🔊✗ | Red border, light red bg | Click to retry |
| **Rate Limited** | 🔊 | Grayed out, disabled | Cannot use (limit reached) |

### Template Context

**hasTTS Flag:**

The `hasTTS` boolean flag is added to template context in UI handlers:

```go
// In internal/ui/entry_*.go handlers
func (h *handler) showEntry(w http.ResponseWriter, r *http.Request) {
    // ... fetch entry ...

    view := view.New(h.tpl, r, sess)
    view.Set("entry", entry)
    view.Set("hasTTS", config.Opts.TTSEnabled())
    // ...
}
```

**Logic:**
- `hasTTS = config.Opts.TTSEnabled()`
- Simple global check: if TTS is enabled for the instance, show buttons
- All authenticated users see TTS buttons (rate limiting handles abuse)

### Template Modifications

**Entry list templates** (unread_entries.html, entry_feed.html, etc.):
```html
{{ if .hasTTS }}
<button
  class="entry-tts-button"
  data-entry-id="{{ .ID }}"
  data-tts-status="ready"
  title="{{ t "entry.tts.read_aloud" }}"
  onclick="handleTTS(this, {{ .ID }})">
  🔊
</button>

<div id="tts-player-{{ .ID }}" class="tts-audio-player" style="display: none;">
  <audio controls preload="none">
    <source id="tts-audio-src-{{ .ID }}" src="" type="audio/mpeg">
  </audio>
</div>
{{ end }}
```

**Entry detail template** (entry.html):
```html
{{ if .hasTTS }}
<button
  class="entry-tts-button-detail"
  data-entry-id="{{ .ID }}"
  data-tts-status="ready"
  onclick="handleTTS(this, {{ .ID }})">
  <span class="icon">🔊</span>
  <span class="text">{{ t "entry.tts.read_aloud" }}</span>
</button>

<div id="tts-player-{{ .ID }}" class="tts-audio-player" style="display: none; margin: 1rem 0;">
  <audio controls preload="none" style="width: 100%;">
    <source id="tts-audio-src-{{ .ID }}" src="" type="audio/mpeg">
  </audio>
</div>
{{ end }}
```

### JavaScript Implementation

**Key functions:**
- `handleTTS(button, entryID)`: Main click handler
- `setTTSButtonState(button, state)`: Update button appearance
- `toggleAudioPlayer(entryID)`: Show/hide inline audio player

**Flow:**
1. User clicks button
2. If cached: Show player, play audio
3. If not cached: Set loading state, call API
4. On success: Load audio into player, set cached state, play
5. On error: Set error state, show notification

## Language Detection

### Implementation

```go
func DetectLanguage(entry *model.Entry) string {
    // Use feed language metadata (already parsed by Miniflux)
    if entry.Feed != nil && entry.Feed.Language != "" {
        return normalizeLanguageCode(entry.Feed.Language)
    }

    // Fallback to configured default
    defaultLang := config.Opts.TTSDefaultLanguage()
    if defaultLang != "" {
        return defaultLang
    }

    return "en"
}

func normalizeLanguageCode(code string) string {
    code = strings.ToLower(strings.TrimSpace(code))

    // Map common language code variants to standard codes
    switch {
    case strings.HasPrefix(code, "en"):
        return "en"
    case code == "zh" || code == "cmn" || strings.HasPrefix(code, "zh-cn"):
        return "zh-CN"
    case strings.HasPrefix(code, "zh-tw") || strings.HasPrefix(code, "zh-hk"):
        return "zh-TW"
    case strings.HasPrefix(code, "ja"):
        return "ja"
    case strings.HasPrefix(code, "ko"):
        return "ko"
    case strings.HasPrefix(code, "es"):
        return "es"
    case strings.HasPrefix(code, "fr"):
        return "fr"
    case strings.HasPrefix(code, "de"):
        return "de"
    case strings.HasPrefix(code, "it"):
        return "it"
    case strings.HasPrefix(code, "pt"):
        return "pt"
    case strings.HasPrefix(code, "ru"):
        return "ru"
    case strings.HasPrefix(code, "ar"):
        return "ar"
    case strings.HasPrefix(code, "hi"):
        return "hi"
    default:
        // Pass through unknown codes as-is (TTS service handles or errors)
        return code
    }
}
```

**Behavior:**
- Normalizes common variants to standard codes (e.g., "en-US", "en-GB" → "en")
- Handles Chinese simplified/traditional distinction (zh-CN vs zh-TW)
- Unknown/unsupported codes are passed through unchanged
- TTS service is responsible for handling or rejecting unknown language codes

**Design rationale:**
- Trust feed metadata (most reliable)
- Simple normalization for common variants
- Fallback to admin-configured default when feed has no language
- Pass normalized code to TTS service

## Error Handling and Edge Cases

### Error Scenarios

**1. TTS Service Errors:**
- Authentication failure → Log for admin, show error to user
- Rate limit from service → Show message, user waits
- Service unavailable → Show error, user can retry
- Timeout → Show timeout error

**2. File Storage Errors:**
- Disk full → Return error, log for admin monitoring
- Permission errors → Check on startup, fail early
- File already exists → Overwrite (reuse filename pattern)

**3. Content Too Large:**
- Limit: 50KB of text (~50,000 characters)
- Validation before calling TTS service
- Return error if exceeded

**4. Concurrent Requests:**
- Same entry, same user, multiple clicks → First request wins, subsequent get cached result
- Use mutex per entry to prevent duplicate generation

**5. Missing Feed Language:**
- Fallback to `TTS_DEFAULT_LANGUAGE` config

**6. Audio Download Failures:**
- TTS service returns URL but download fails → Return error, don't cache partial data
- User can retry manually

## Testing Strategy

### Unit Tests

**`internal/tts/client_test.go`:**
- Successful TTS generation with mock HTTP server
- HTTP error codes (400, 401, 429, 500)
- Timeout handling
- Malformed JSON responses

**`internal/tts/cache_test.go`:**
- Cache hit/miss scenarios
- Expired cache detection
- File storage and retrieval
- Concurrent access

**`internal/tts/ratelimit_test.go`:**
- Rate limit enforcement
- Window reset
- Concurrent requests from same user
- Cleanup of old entries

**`internal/tts/language_test.go`:**
- Language code normalization
- Fallback behavior
- Feed metadata extraction

### Integration Tests

**`internal/api/entry_tts_test.go`:**
- Full request flow end-to-end
- Authentication/authorization
- Rate limiting via API
- File serving with access control

**`internal/storage/tts_cache_test.go`:**
- CRUD operations on tts_audio_cache table
- Cascade deletion
- Cleanup of expired records

### Manual Testing Checklist

- [ ] TTS button appears in entry list and detail views
- [ ] Button states transition correctly (ready → loading → cached/error)
- [ ] Audio plays inline when ready
- [ ] Rate limiting works (reject after N requests/hour)
- [ ] Cache persists across restarts
- [ ] Cleanup job removes expired files and DB records
- [ ] Different languages detected and sent to TTS service
- [ ] File serving respects user permissions
- [ ] Works with various TTS services (OpenAI, ElevenLabs, custom)
- [ ] Proper error messages for all failure scenarios

## Performance Considerations

**Storage:**
- Typical 5-minute article → 2-5MB MP3 file
- Default 24h cache → Storage grows with active users
- Cleanup job manages disk usage
- Optional: Add max cache size limit in future

**Database:**
- Indexes optimize cache lookups and cleanup queries
- CASCADE deletes keep data consistent
- Lightweight records (only metadata, not audio data)

**Concurrency:**
- Rate limiter prevents abuse
- In-memory map with mutex (minimal overhead)
- Background cleanup prevents memory leaks

**Network:**
- One-time download per cache entry
- Serves files locally after caching
- No continuous streaming overhead

## Future Enhancements (Out of Scope)

- Per-user TTS service configuration (override global settings)
- Per-feed voice/language mapping
- Transcript generation alongside audio
- Playback speed control
- Download button for offline listening
- Sharing audio URLs publicly
- Integration with podcast players
- Advanced language detection (ML-based)
- Progress tracking (resume from last position)
- Queue multiple entries for sequential listening

## Migration Path

**Version 1 (Initial Release):**
- All features described in this document
- Global admin configuration only
- Basic language detection from feed metadata

**Version 2 (Potential Future):**
- Per-user configuration overrides
- Advanced voice/language mapping
- Usage analytics

## Security Considerations

**Authentication:**
- All endpoints require valid user session
- API key stored securely (support for FILE-based secrets)

**Authorization:**
- Users can only access TTS for entries they can read
- File serving validates user ownership before serving

**Rate Limiting:**
- Prevents individual user abuse
- Protects against cost escalation

**Input Validation:**
- Filename sanitization (prevent path traversal)
- Content length limits
- Language code validation

**Privacy:**
- Audio files are user-specific (entry_id + user_id)
- Cache respects user permissions
- Files deleted when user/entry deleted (CASCADE)

## Rollout Plan

**Phase 1: Core Implementation**
1. Database migration
2. TTS client and cache logic
3. API endpoints
4. Basic UI (buttons only, no player)

**Phase 2: UI Polish**
5. Inline audio player
6. Button state management
7. Error messages and notifications

**Phase 3: Operations**
8. Cleanup scheduler job
9. Rate limiting
10. Monitoring and logging

**Phase 4: Testing & Documentation**
11. Unit and integration tests
12. User documentation
13. Admin configuration guide

## Documentation Requirements

**User Documentation:**
- How to use TTS feature
- Troubleshooting common issues

**Admin Documentation:**
- Configuration options (environment variables)
- Setting up TTS service integration
- Monitoring disk usage
- Rate limit tuning

**Developer Documentation:**
- API contract for TTS services
- Adding new TTS providers
- Testing guide

## Success Metrics

- Users generate TTS audio without errors
- Cache hit rate > 50% (reduces API costs)
- Rate limiting prevents abuse (< 1% of requests blocked)
- Disk usage remains manageable (cleanup works correctly)
- No security vulnerabilities (proper access control)

## Open Questions

None - all design decisions have been made.

## Appendix: Wrapper Service Implementation

### Why a Wrapper Service?

Most TTS providers (OpenAI, ElevenLabs, Google Cloud TTS, AWS Polly) have different APIs and return audio data directly, not URLs. To keep Miniflux simple and provider-agnostic, we require a thin wrapper service that:

1. Implements Miniflux's standard HTTP contract
2. Calls the actual TTS provider
3. Stores generated audio temporarily
4. Returns a URL with expiry

### Reference Implementation (Python/Flask)

```python
from flask import Flask, request, jsonify
import openai
import boto3
from datetime import datetime, timedelta
import hashlib
import os

app = Flask(__name__)

# Configuration
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
S3_BUCKET = os.getenv("S3_BUCKET")
S3_PREFIX = "tts-audio/"
EXPIRY_HOURS = 24

openai.api_key = OPENAI_API_KEY
s3 = boto3.client('s3')

@app.route('/tts', methods=['POST'])
def generate_tts():
    data = request.json
    text = data.get('text')
    voice = data.get('voice', 'alloy')
    language = data.get('language', 'en')

    # Generate audio using OpenAI TTS
    response = openai.audio.speech.create(
        model="tts-1",
        voice=voice,
        input=text
    )

    # Generate unique filename
    content_hash = hashlib.sha256(text.encode()).hexdigest()[:16]
    filename = f"{S3_PREFIX}{content_hash}.mp3"

    # Upload to S3
    s3.put_object(
        Bucket=S3_BUCKET,
        Key=filename,
        Body=response.content,
        ContentType='audio/mpeg'
    )

    # Generate presigned URL
    expires_at = datetime.utcnow() + timedelta(hours=EXPIRY_HOURS)
    audio_url = s3.generate_presigned_url(
        'get_object',
        Params={'Bucket': S3_BUCKET, 'Key': filename},
        ExpiresIn=EXPIRY_HOURS * 3600
    )

    return jsonify({
        'audio_url': audio_url,
        'expires_at': expires_at.isoformat() + 'Z'
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
```

### Alternative: Local Storage Wrapper

For simpler setups without S3:

```python
from flask import Flask, request, jsonify, send_file
import openai
from datetime import datetime, timedelta
import hashlib
import os

app = Flask(__name__)
STORAGE_PATH = "/var/tts-audio"
BASE_URL = os.getenv("BASE_URL", "http://localhost:8080")

@app.route('/tts', methods=['POST'])
def generate_tts():
    data = request.json
    text = data.get('text')
    voice = data.get('voice', 'alloy')

    # Generate audio
    response = openai.audio.speech.create(
        model="tts-1",
        voice=voice,
        input=text
    )

    # Save locally
    content_hash = hashlib.sha256(text.encode()).hexdigest()[:16]
    filename = f"{content_hash}.mp3"
    filepath = os.path.join(STORAGE_PATH, filename)

    with open(filepath, 'wb') as f:
        f.write(response.content)

    # Return URL to local file
    expires_at = datetime.utcnow() + timedelta(hours=24)
    return jsonify({
        'audio_url': f"{BASE_URL}/audio/{filename}",
        'expires_at': expires_at.isoformat() + 'Z'
    })

@app.route('/audio/<filename>')
def serve_audio(filename):
    return send_file(
        os.path.join(STORAGE_PATH, filename),
        mimetype='audio/mpeg'
    )
```

### Docker Compose Example

```yaml
version: '3.8'
services:
  miniflux:
    image: miniflux/miniflux:latest
    environment:
      - TTS_ENABLED=1
      - TTS_ENDPOINT_URL=http://tts-wrapper:8080/tts
      - TTS_API_KEY=shared-secret
      - TTS_VOICE=alloy

  tts-wrapper:
    build: ./tts-wrapper
    environment:
      - OPENAI_API_KEY=sk-...
      - BASE_URL=http://tts-wrapper:8080
    volumes:
      - tts-audio:/var/tts-audio

volumes:
  tts-audio:
```

### Deployment Recommendations

- Deploy wrapper service close to Miniflux (low latency)
- Use S3/object storage for scalability
- Add authentication between Miniflux and wrapper (API key)
- Set up cleanup job to delete expired audio files
- Monitor costs (TTS API usage)
- Consider caching at wrapper level (dedup identical requests)
