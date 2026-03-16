# TTS Feature Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Text-to-Speech functionality to Miniflux allowing users to listen to feed entries using configurable external TTS services

**Architecture:** Dedicated `internal/tts/` subsystem with generic HTTP client, local file caching, in-memory rate limiting, and inline audio player UI. Requires external wrapper service that implements Miniflux's HTTP contract.

**Tech Stack:** Go, PostgreSQL, HTML templates, vanilla JavaScript, existing Miniflux infrastructure

---

## File Structure Overview

**New files to create:**
- `internal/tts/client.go` - HTTP client for TTS service communication
- `internal/tts/client_test.go` - Tests for TTS client
- `internal/tts/language.go` - Language detection and normalization
- `internal/tts/language_test.go` - Tests for language detection
- `internal/tts/ratelimit.go` - In-memory per-user rate limiting
- `internal/tts/ratelimit_test.go` - Tests for rate limiter
- `internal/tts/cache.go` - Cache management with file storage
- `internal/tts/cache_test.go` - Tests for cache logic
- `internal/model/tts.go` - TTS data models
- `internal/storage/tts_cache.go` - Database operations for TTS cache
- `internal/storage/tts_cache_test.go` - Tests for storage operations
- `internal/api/entry_tts.go` - API endpoints for TTS
- `internal/api/entry_tts_test.go` - Tests for API endpoints

**Files to modify:**
- `internal/database/migrations.go` - Add TTS cache table migration
- `internal/config/options.go` - Add TTS configuration options
- `internal/ui/entry_*.go` - Add hasTTS to template context
- `internal/template/templates/views/entry.html` - Add TTS button and player
- `internal/template/templates/views/unread_entries.html` - Add TTS button
- `internal/ui/static/js/app.js` - Add TTS JavaScript handlers
- `internal/ui/static/css/app.css` - Add TTS button styles
- `internal/worker/scheduler.go` - Add TTS cleanup job
- `internal/api/api.go` - Register TTS routes

---

## Chunk 1: Foundation (Config, Models, Migrations)

### Task 1.1: Add TTS Configuration Options

**Files:**
- Modify: `internal/config/options.go`
- Test: Manual verification via config output

- [ ] **Step 1: Add TTS config options to NewConfigOptions()**

In `internal/config/options.go`, add after existing options (around line 607):

```go
"TTS_ENABLED": {
	parsedBoolValue: false,
	rawValue:        "0",
	valueType:       boolType,
},
"TTS_ENDPOINT_URL": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
},
"TTS_API_KEY": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         stringType,
	secret:            true,
},
"TTS_API_KEY_FILE": {
	parsedStringValue: "",
	rawValue:          "",
	valueType:         secretFileType,
	targetKey:         "TTS_API_KEY",
},
"TTS_VOICE": {
	parsedStringValue: "alloy",
	rawValue:          "alloy",
	valueType:         stringType,
},
"TTS_DEFAULT_LANGUAGE": {
	parsedStringValue: "en",
	rawValue:          "en",
	valueType:         stringType,
},
"TTS_STORAGE_PATH": {
	parsedStringValue: "./data/tts_audio",
	rawValue:          "./data/tts_audio",
	valueType:         stringType,
},
"TTS_CACHE_DURATION": {
	parsedDuration: time.Hour * 24,
	rawValue:       "24",
	valueType:      hourType,
	validator: func(rawValue string) error {
		return validateGreaterOrEqualThan(rawValue, 1)
	},
},
"TTS_RATE_LIMIT_PER_HOUR": {
	parsedIntValue: 20,
	rawValue:       "20",
	valueType:      intType,
	validator: func(rawValue string) error {
		return validateGreaterOrEqualThan(rawValue, 1)
	},
},
```

- [ ] **Step 2: Add accessor methods**

Add after existing accessor methods (around line 1005):

```go
func (c *configOptions) TTSEnabled() bool {
	return c.options["TTS_ENABLED"].parsedBoolValue
}

func (c *configOptions) TTSEndpointURL() string {
	return c.options["TTS_ENDPOINT_URL"].parsedStringValue
}

func (c *configOptions) TTSAPIKey() string {
	return c.options["TTS_API_KEY"].parsedStringValue
}

func (c *configOptions) TTSVoice() string {
	return c.options["TTS_VOICE"].parsedStringValue
}

func (c *configOptions) TTSDefaultLanguage() string {
	return c.options["TTS_DEFAULT_LANGUAGE"].parsedStringValue
}

func (c *configOptions) TTSStoragePath() string {
	return c.options["TTS_STORAGE_PATH"].parsedStringValue
}

func (c *configOptions) TTSCacheDuration() time.Duration {
	return c.options["TTS_CACHE_DURATION"].parsedDuration
}

func (c *configOptions) TTSRateLimitPerHour() int {
	return c.options["TTS_RATE_LIMIT_PER_HOUR"].parsedIntValue
}
```

- [ ] **Step 3: Verify config compiles**

Run: `go build -o /dev/null ./internal/config`
Expected: No errors

- [ ] **Step 4: Test config output**

Run: `go build -o miniflux-test . && ./miniflux-test -info 2>&1 | grep TTS; rm miniflux-test`
Expected: Shows TTS config options with default values (TTS_ENABLED=0, TTS_VOICE=alloy, etc.)

- [ ] **Step 5: Commit**

```bash
git add internal/config/options.go
git commit -m "feat(config): add TTS configuration options

Add configuration for TTS feature:
- TTS_ENABLED: feature toggle
- TTS_ENDPOINT_URL: wrapper service endpoint
- TTS_API_KEY: authentication for wrapper service
- TTS_VOICE: default voice identifier
- TTS_DEFAULT_LANGUAGE: fallback language
- TTS_STORAGE_PATH: local audio file storage
- TTS_CACHE_DURATION: cache expiry time
- TTS_RATE_LIMIT_PER_HOUR: per-user throttle limit"
```

### Task 1.2: Create TTS Data Models

**Files:**
- Create: `internal/model/tts.go`
- Test: Manual verification via compilation

- [ ] **Step 1: Create TTS model file**

Create `internal/model/tts.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model // import "miniflux.app/v2/internal/model"

import (
	"time"
)

// TTSCache represents a cached TTS audio file.
type TTSCache struct {
	ID        int64
	EntryID   int64
	UserID    int64
	FilePath  string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// TTSAudioRequest represents a request to generate TTS audio.
type TTSAudioRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Voice    string `json:"voice"`
}

// TTSAudioResponse represents the response from TTS service.
type TTSAudioResponse struct {
	AudioURL  string `json:"audio_url"`
	ExpiresAt string `json:"expires_at"` // ISO 8601 timestamp
}
```

- [ ] **Step 2: Verify model compiles**

Run: `go build -o /dev/null ./internal/model`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/model/tts.go
git commit -m "feat(model): add TTS data models

Add models for TTS feature:
- TTSCache: cached audio file metadata
- TTSAudioRequest: TTS service request
- TTSAudioResponse: TTS service response"
```

### Task 1.3: Create Database Migration

**Files:**
- Modify: `internal/database/migrations.go`
- Test: Verify migration count increases

- [ ] **Step 1: Check current migration count**

Run: `grep -c "func(tx \*sql.Tx)" internal/database/migrations.go`
Expected: Shows current count (e.g., 127)
Note: New migration will be this number + 1

- [ ] **Step 2: Add migration function to migrations array**

In `internal/database/migrations.go`, add new migration at the END of the `migrations` array (before the closing `}`):

```go
func(tx *sql.Tx) (err error) {
	sql := `
		CREATE TABLE tts_audio_cache (
			id bigserial primary key,
			entry_id bigint not null references entries(id) on delete cascade,
			user_id bigint not null references users(id) on delete cascade,
			file_path text not null,
			expires_at timestamp with time zone not null,
			created_at timestamp with time zone not null default now(),
			unique(entry_id, user_id)
		);

		CREATE INDEX tts_audio_cache_expires_at_idx ON tts_audio_cache(expires_at);
		CREATE INDEX tts_audio_cache_user_id_idx ON tts_audio_cache(user_id);
		CREATE INDEX tts_audio_cache_entry_id_idx ON tts_audio_cache(entry_id);
	`

	_, err = tx.Exec(sql)
	return err
},
```

- [ ] **Step 3: Verify migration compiles**

Run: `go build -o /dev/null ./internal/database`
Expected: No errors

- [ ] **Step 4: Verify schema version increased**

Run: `go run . -info 2>&1 | grep -i schema`
Expected: Schema version shows one higher than Step 1 count

- [ ] **Step 5: Commit**

```bash
git add internal/database/migrations.go
git commit -m "feat(db): add TTS cache table migration

Add tts_audio_cache table:
- Stores cached TTS audio file metadata
- Links to entries and users with CASCADE delete
- Unique constraint per (entry_id, user_id)
- Indexes for efficient cleanup and lookups
- Uses timestamp with time zone for consistency"
```

---

## Chunk 2: TTS Core Components

### Task 2.1: Implement Language Detection

**Files:**
- Create: `internal/tts/language.go`
- Create: `internal/tts/language_test.go`

- [ ] **Step 1: Write test for DetectLanguage**

Create `internal/tts/language_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"

	"miniflux.app/v2/internal/model"
)

func TestDetectLanguage_FromFeedMetadata(t *testing.T) {
	entry := &model.Entry{
		Feed: &model.Feed{
			Language: "en-US",
		},
	}

	result := DetectLanguage(entry, "fr")
	expected := "en"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDetectLanguage_FallbackToDefault(t *testing.T) {
	entry := &model.Entry{
		Feed: &model.Feed{
			Language: "",
		},
	}

	result := DetectLanguage(entry, "de")
	expected := "de"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDetectLanguage_NoFeed(t *testing.T) {
	entry := &model.Entry{
		Feed: nil,
	}

	result := DetectLanguage(entry, "en")
	expected := "en"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestNormalizeLanguageCode_English(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en", "en"},
		{"en-US", "en"},
		{"en-GB", "en"},
		{"EN", "en"},
		{"  en-us  ", "en"},
	}

	for _, tc := range tests {
		result := normalizeLanguageCode(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeLanguageCode(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestNormalizeLanguageCode_Chinese(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"zh", "zh-CN"},
		{"zh-CN", "zh-CN"},
		{"cmn", "zh-CN"},
		{"zh-TW", "zh-TW"},
		{"zh-HK", "zh-TW"},
	}

	for _, tc := range tests {
		result := normalizeLanguageCode(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeLanguageCode(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestNormalizeLanguageCode_Other(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ja", "ja"},
		{"ko", "ko"},
		{"es", "es"},
		{"fr", "fr"},
		{"de", "de"},
		{"unknown-lang", "unknown-lang"}, // Pass through
	}

	for _, tc := range tests {
		result := normalizeLanguageCode(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeLanguageCode(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v`
Expected: FAIL with "no such file or directory" (language.go doesn't exist)

- [ ] **Step 3: Implement language detection**

Create `internal/tts/language.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"strings"

	"miniflux.app/v2/internal/model"
)

// DetectLanguage detects the language for TTS from entry metadata.
func DetectLanguage(entry *model.Entry, defaultLanguage string) string {
	// Use feed language metadata (already parsed by Miniflux)
	if entry.Feed != nil && entry.Feed.Language != "" {
		return normalizeLanguageCode(entry.Feed.Language)
	}

	// Fallback to configured default
	if defaultLanguage != "" {
		return defaultLanguage
	}

	return "en"
}

// normalizeLanguageCode standardizes language codes to common formats.
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
		// Pass through unknown codes as-is
		return code
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestDetectLanguage`
Expected: PASS (all language detection tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tts/language.go internal/tts/language_test.go
git commit -m "feat(tts): add language detection and normalization

Implement language detection from feed metadata:
- DetectLanguage(): extract and normalize feed language
- normalizeLanguageCode(): map variants to standard codes
- Fallback to configurable default language
- Pass-through unknown codes to TTS service"
```

### Task 2.2: Implement Rate Limiter

**Files:**
- Create: `internal/tts/ratelimit.go`
- Create: `internal/tts/ratelimit_test.go`

- [ ] **Step 1: Write test for rate limiter**

Create `internal/tts/ratelimit_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow_FirstRequest(t *testing.T) {
	rl := NewRateLimiter(10)
	userID := int64(123)

	if !rl.Allow(userID) {
		t.Error("First request should be allowed")
	}
}

func TestRateLimiter_Allow_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(3)
	userID := int64(123)

	// Make 3 requests (limit)
	for i := 0; i < 3; i++ {
		if !rl.Allow(userID) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_Deny_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(2)
	userID := int64(123)

	// Make 2 requests (limit)
	rl.Allow(userID)
	rl.Allow(userID)

	// 3rd request should be denied
	if rl.Allow(userID) {
		t.Error("Request exceeding limit should be denied")
	}
}

func TestRateLimiter_Remaining(t *testing.T) {
	rl := NewRateLimiter(5)
	userID := int64(123)

	// No requests yet
	if remaining := rl.Remaining(userID); remaining != 5 {
		t.Errorf("Expected 5 remaining, got %d", remaining)
	}

	// Make 2 requests
	rl.Allow(userID)
	rl.Allow(userID)

	if remaining := rl.Remaining(userID); remaining != 3 {
		t.Errorf("Expected 3 remaining, got %d", remaining)
	}
}

func TestRateLimiter_DifferentUsers(t *testing.T) {
	rl := NewRateLimiter(1)
	user1 := int64(123)
	user2 := int64(456)

	// User 1 uses their limit
	if !rl.Allow(user1) {
		t.Error("User 1 first request should be allowed")
	}
	if rl.Allow(user1) {
		t.Error("User 1 second request should be denied")
	}

	// User 2 should still have their limit
	if !rl.Allow(user2) {
		t.Error("User 2 first request should be allowed")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	rl := NewRateLimiter(1)
	userID := int64(123)

	// Use the limit
	if !rl.Allow(userID) {
		t.Error("First request should be allowed")
	}

	// Manually expire the window by setting windowStart to past
	rl.mu.Lock()
	if limit, exists := rl.limits[userID]; exists {
		limit.windowStart = time.Now().Add(-2 * time.Hour)
	}
	rl.mu.Unlock()

	// Should allow again after window reset
	if !rl.Allow(userID) {
		t.Error("Request after window reset should be allowed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestRateLimiter`
Expected: FAIL with "undefined: NewRateLimiter"

- [ ] **Step 3: Implement rate limiter**

Create `internal/tts/ratelimit.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"sync"
	"time"
)

// RateLimiter implements in-memory per-user rate limiting.
type RateLimiter struct {
	mu         sync.RWMutex
	limits     map[int64]*userLimit
	maxPerHour int
}

type userLimit struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a new rate limiter with specified max requests per hour.
func NewRateLimiter(maxPerHour int) *RateLimiter {
	rl := &RateLimiter{
		limits:     make(map[int64]*userLimit),
		maxPerHour: maxPerHour,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed for the given user.
func (rl *RateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	limit, exists := rl.limits[userID]

	// New window
	if !exists || now.Sub(limit.windowStart) > time.Hour {
		rl.limits[userID] = &userLimit{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Check limit
	if limit.count >= rl.maxPerHour {
		return false
	}

	// Increment and allow
	limit.count++
	return true
}

// Remaining returns the number of requests remaining for the user.
func (rl *RateLimiter) Remaining(userID int64) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limit, exists := rl.limits[userID]

	// No requests yet or window expired
	if !exists || time.Since(limit.windowStart) > time.Hour {
		return rl.maxPerHour
	}

	remaining := rl.maxPerHour - limit.count
	if remaining < 0 {
		return 0
	}

	return remaining
}

// cleanup removes stale entries periodically.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for userID, limit := range rl.limits {
			// Remove entries older than 2 hours
			if now.Sub(limit.windowStart) > 2*time.Hour {
				delete(rl.limits, userID)
			}
		}

		rl.mu.Unlock()
	}
}

// Global rate limiter instance
var globalRateLimiter *RateLimiter
var rateLimiterOnce sync.Once

// AllowRequest checks if a user can make a TTS request (singleton pattern).
func AllowRequest(userID int64, limitPerHour int) bool {
	rateLimiterOnce.Do(func() {
		globalRateLimiter = NewRateLimiter(limitPerHour)
	})
	return globalRateLimiter.Allow(userID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestRateLimiter`
Expected: PASS (all rate limiter tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tts/ratelimit.go internal/tts/ratelimit_test.go
git commit -m "feat(tts): add in-memory rate limiter

Implement per-user rate limiting:
- Sliding 1-hour windows
- Configurable max requests per hour
- Thread-safe with mutex
- Automatic cleanup of stale entries"
```

### Task 2.3: Implement TTS HTTP Client

**Files:**
- Create: `internal/tts/client.go`
- Create: `internal/tts/client_test.go`

- [ ] **Step 1: Write test for TTS client with mock server**

Create `internal/tts/client_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Generate_Success(t *testing.T) {
	// Mock TTS server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", auth)
		}

		// Return mock response
		response := map[string]string{
			"audio_url":  "https://example.com/audio.mp3",
			"expires_at": "2026-03-17T10:00:00Z",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "alloy")
	result, err := client.Generate("Hello world", "en")

	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.AudioURL != "https://example.com/audio.mp3" {
		t.Errorf("Expected audio URL, got %s", result.AudioURL)
	}

	if result.ExpiresAt.IsZero() {
		t.Error("Expected non-zero ExpiresAt")
	}
}

func TestClient_Generate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "alloy")
	_, err := client.Generate("test", "en")

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

func TestClient_Generate_HTTPError(t *testing.T) {
	tests := []struct {
		statusCode   int
		expectedErr  string
	}{
		{400, "invalid TTS request"},
		{401, "authentication failed"},
		{429, "rate limit exceeded"},
		{500, "unavailable"},
	}

	for _, tc := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.statusCode)
		}))

		client := NewClient(server.URL, "test-key", "alloy")
		_, err := client.Generate("test", "en")

		if err == nil {
			t.Errorf("Expected error for status %d", tc.statusCode)
		}
		if !strings.Contains(err.Error(), tc.expectedErr) {
			t.Errorf("Status %d: expected error containing %q, got %v", tc.statusCode, tc.expectedErr, err)
		}

		server.Close()
	}
}

func TestClient_DownloadAudio_Success(t *testing.T) {
	audioData := []byte("fake mp3 data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(audioData)
	}))
	defer server.Close()

	client := NewClient("", "", "")
	data, err := client.DownloadAudio(server.URL)

	if err != nil {
		t.Fatalf("DownloadAudio failed: %v", err)
	}

	if string(data) != string(audioData) {
		t.Errorf("Expected %s, got %s", audioData, data)
	}
}

func TestClient_DownloadAudio_FileTooLarge(t *testing.T) {
	// Create large data > 50MB
	largeData := make([]byte, 51*1024*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "53477376") // 51MB
		w.Write(largeData)
	}))
	defer server.Close()

	client := NewClient("", "", "")
	_, err := client.DownloadAudio(server.URL)

	if err == nil {
		t.Error("Expected error for file too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

func TestClient_DownloadAudio_WrongContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("not audio"))
	}))
	defer server.Close()

	client := NewClient("", "", "")
	_, err := client.DownloadAudio(server.URL)

	if err == nil {
		t.Error("Expected error for wrong content type")
	}
	if !strings.Contains(err.Error(), "audio/mpeg") {
		t.Errorf("Expected content-type error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestClient`
Expected: FAIL with "undefined: NewClient"

- [ ] **Step 3: Implement TTS client (part 1 - Generate)**

Create `internal/tts/client.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"miniflux.app/v2/internal/model"
)

const (
	maxContentLength = 50000   // 50KB of text
	maxFileSize      = 50 << 20 // 50MB
)

// Client is HTTP client for TTS service.
type Client struct {
	endpointURL string
	apiKey      string
	voice       string
	httpClient  *http.Client
}

// NewClient creates a new TTS client.
func NewClient(endpointURL, apiKey, voice string) *Client {
	return &Client{
		endpointURL: endpointURL,
		apiKey:      apiKey,
		voice:       voice,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ServiceResult contains TTS service response.
type ServiceResult struct {
	AudioURL  string
	ExpiresAt time.Time
}

// Generate calls TTS service to generate audio.
func (c *Client) Generate(text string, language string) (*ServiceResult, error) {
	// Validate content length
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("entry content too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Prepare request
	reqBody := &model.TTSAudioRequest{
		Text:     text,
		Language: language,
		Voice:    c.voice,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpointURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS service request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleHTTPError(resp.StatusCode)
	}

	// Parse response
	var ttsResp model.TTSAudioResponse
	if err := json.NewDecoder(resp.Body).Decode(&ttsResp); err != nil {
		return nil, fmt.Errorf("failed to parse TTS response: %w", err)
	}

	// Parse expires_at timestamp
	expiresAt, err := time.Parse(time.RFC3339, ttsResp.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expires_at: %w", err)
	}

	return &ServiceResult{
		AudioURL:  ttsResp.AudioURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (c *Client) handleHTTPError(statusCode int) error {
	switch statusCode {
	case 400:
		return errors.New("invalid TTS request")
	case 401, 403:
		return errors.New("TTS authentication failed - check API key")
	case 429:
		return errors.New("TTS service rate limit exceeded")
	case 500, 502, 503:
		return errors.New("TTS service unavailable")
	default:
		return fmt.Errorf("TTS service error: HTTP %d", statusCode)
	}
}
```

- [ ] **Step 4: Implement DownloadAudio method**

Add to `internal/tts/client.go`:

```go
// DownloadAudio downloads audio file from URL.
func (c *Client) DownloadAudio(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Create client with longer timeout for downloads
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("audio download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("audio download failed: HTTP %d", resp.StatusCode)
	}

	// Validate Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "audio/mpeg" && contentType != "audio/mp3" {
		return nil, fmt.Errorf("invalid content type: expected audio/mpeg, got %s", contentType)
	}

	// Check file size from Content-Length header
	if contentLengthStr := resp.Header.Get("Content-Length"); contentLengthStr != "" {
		contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
		if err == nil && contentLength > maxFileSize {
			return nil, fmt.Errorf("audio file too large: %d bytes (max %d)", contentLength, maxFileSize)
		}
	}

	// Download with size limit
	limitedReader := io.LimitReader(resp.Body, maxFileSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	// Check if size limit was exceeded
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("audio file too large: exceeds %d bytes", maxFileSize)
	}

	return data, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestClient`
Expected: PASS (all client tests)

- [ ] **Step 6: Commit**

```bash
git add internal/tts/client.go internal/tts/client_test.go
git commit -m "feat(tts): add HTTP client for TTS service

Implement TTS service communication:
- Generate(): call TTS service with text/language
- DownloadAudio(): fetch audio file from URL
- Content length validation (50KB text max)
- File size validation (50MB max)
- Content-Type validation (audio/mpeg)
- Timeout handling (30s generate, 60s download)
- HTTP error handling with specific messages"
```

---

## Chunk 3: Cache & Storage

### Task 3.1: Implement Storage Layer

**Files:**
- Create: `internal/storage/tts_cache.go`
- Create: `internal/storage/tts_cache_test.go`

- [ ] **Step 1: Write tests for storage operations**

Create `internal/storage/tts_cache_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestStorage_CreateTTSCache(t *testing.T) {
	// Note: This requires test database setup
	// Skip if DB not available
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	cache := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/test.mp3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.CreateTTSCache(cache)
	if err != nil {
		t.Fatalf("CreateTTSCache failed: %v", err)
	}

	if cache.ID == 0 {
		t.Error("Expected non-zero ID after insert")
	}
}

func TestStorage_GetTTSCache_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	// Create cache entry
	original := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/test.mp3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	store.CreateTTSCache(original)

	// Retrieve it
	retrieved, err := store.GetTTSCache(entryID, userID)
	if err != nil {
		t.Fatalf("GetTTSCache failed: %v", err)
	}

	if retrieved.FilePath != original.FilePath {
		t.Errorf("Expected %s, got %s", original.FilePath, retrieved.FilePath)
	}
}

func TestStorage_GetTTSCache_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	_, err := store.GetTTSCache(99999, 99999)
	if err == nil {
		t.Error("Expected error for non-existent cache")
	}
}

func TestStorage_GetExpiredTTSCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	// Create expired cache entry
	expiredCache := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/expired.mp3",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
	}
	store.CreateTTSCache(expiredCache)

	// Get expired caches
	expired, err := store.GetExpiredTTSCache()
	if err != nil {
		t.Fatalf("GetExpiredTTSCache failed: %v", err)
	}

	if len(expired) == 0 {
		t.Error("Expected at least one expired cache entry")
	}
}

func TestStorage_DeleteTTSCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	store := newTestStorage(t)
	defer cleanupTestStorage(t, store)

	userID, entryID := createTestUserAndEntry(t, store)

	// Create and then delete
	cache := &model.TTSCache{
		EntryID:   entryID,
		UserID:    userID,
		FilePath:  "tts_audio/test.mp3",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	store.CreateTTSCache(cache)

	err := store.DeleteTTSCache(cache.ID)
	if err != nil {
		t.Fatalf("DeleteTTSCache failed: %v", err)
	}

	// Verify deletion
	_, err = store.GetTTSCache(entryID, userID)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage -v -run TestStorage_.*TTSCache`
Expected: FAIL with "undefined" errors (tts_cache.go doesn't exist)

- [ ] **Step 3: Implement storage operations**

Create `internal/storage/tts_cache.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"

	"miniflux.app/v2/internal/model"
)

// CreateTTSCache creates a new TTS cache entry.
func (s *Storage) CreateTTSCache(cache *model.TTSCache) error {
	query := `
		INSERT INTO tts_audio_cache (entry_id, user_id, file_path, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entry_id, user_id)
		DO UPDATE SET file_path = $3, expires_at = $4, created_at = NOW()
		RETURNING id, created_at
	`

	err := s.db.QueryRow(
		query,
		cache.EntryID,
		cache.UserID,
		cache.FilePath,
		cache.ExpiresAt,
	).Scan(&cache.ID, &cache.CreatedAt)

	if err != nil {
		return fmt.Errorf("unable to create TTS cache: %w", err)
	}

	return nil
}

// GetTTSCache retrieves a TTS cache entry by entry and user ID.
func (s *Storage) GetTTSCache(entryID, userID int64) (*model.TTSCache, error) {
	query := `
		SELECT id, entry_id, user_id, file_path, expires_at, created_at
		FROM tts_audio_cache
		WHERE entry_id = $1 AND user_id = $2
	`

	var cache model.TTSCache
	err := s.db.QueryRow(query, entryID, userID).Scan(
		&cache.ID,
		&cache.EntryID,
		&cache.UserID,
		&cache.FilePath,
		&cache.ExpiresAt,
		&cache.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("TTS cache not found for entry %d, user %d", entryID, userID)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to fetch TTS cache: %w", err)
	}

	return &cache, nil
}

// GetExpiredTTSCache retrieves all expired TTS cache entries.
func (s *Storage) GetExpiredTTSCache() ([]*model.TTSCache, error) {
	query := `
		SELECT id, entry_id, user_id, file_path, expires_at, created_at
		FROM tts_audio_cache
		WHERE expires_at < NOW()
		ORDER BY expires_at ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch expired TTS caches: %w", err)
	}
	defer rows.Close()

	var caches []*model.TTSCache
	for rows.Next() {
		var cache model.TTSCache
		err := rows.Scan(
			&cache.ID,
			&cache.EntryID,
			&cache.UserID,
			&cache.FilePath,
			&cache.ExpiresAt,
			&cache.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to scan TTS cache row: %w", err)
		}
		caches = append(caches, &cache)
	}

	return caches, nil
}

// DeleteTTSCache deletes a TTS cache entry by ID.
func (s *Storage) DeleteTTSCache(id int64) error {
	query := `DELETE FROM tts_audio_cache WHERE id = $1`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("unable to delete TTS cache: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to get rows affected: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("TTS cache %d not found", id)
	}

	return nil
}
```

- [ ] **Step 4: Run tests (skip if no test DB)**

Run: `go test ./internal/storage -v -run TestStorage_.*TTSCache -short`
Expected: SKIP (tests skipped in short mode, or PASS if test DB available)

- [ ] **Step 5: Verify compilation**

Run: `go build -o /dev/null ./internal/storage`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/storage/tts_cache.go internal/storage/tts_cache_test.go
git commit -m "feat(storage): add TTS cache database operations

Implement TTS cache storage layer:
- CreateTTSCache(): upsert cache entry
- GetTTSCache(): retrieve by entry_id and user_id
- GetExpiredTTSCache(): find expired entries for cleanup
- DeleteTTSCache(): remove cache entry
- Uses UPSERT to handle re-generation"
```

### Task 3.2: Implement Cache Management

**Files:**
- Create: `internal/tts/cache.go`
- Create: `internal/tts/cache_test.go`

- [ ] **Step 1: Write test for cache logic**

Create `internal/tts/cache_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

type mockStorage struct {
	cache     *model.TTSCache
	cacheErr  error
	createErr error
}

func (m *mockStorage) GetTTSCache(entryID, userID int64) (*model.TTSCache, error) {
	if m.cacheErr != nil {
		return nil, m.cacheErr
	}
	return m.cache, nil
}

func (m *mockStorage) CreateTTSCache(cache *model.TTSCache) error {
	m.cache = cache
	return m.createErr
}

func TestGetOrGenerateAudio_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a cached file
	cachedPath := filepath.Join(tmpDir, "tts_audio", "123_456_789.mp3")
	os.MkdirAll(filepath.Dir(cachedPath), 0755)
	os.WriteFile(cachedPath, []byte("cached audio"), 0644)

	store := &mockStorage{
		cache: &model.TTSCache{
			ID:        1,
			EntryID:   123,
			UserID:    456,
			FilePath:  "tts_audio/123_456_789.mp3",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	entry := &model.Entry{
		ID:      123,
		Title:   "Test",
		Content: "Test content",
		Feed:    &model.Feed{Language: "en"},
	}

	client := NewClient("", "", "")

	result, err := GetOrGenerateAudio(store, client, entry, 456, 24*time.Hour, tmpDir, "en")
	if err != nil {
		t.Fatalf("GetOrGenerateAudio failed: %v", err)
	}

	if !strings.Contains(result.FilePath, "123_456_789") {
		t.Errorf("Expected cached file path, got %s", result.FilePath)
	}
}

func TestGetOrGenerateAudio_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock TTS server
	audioData := []byte("generated audio data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tts" {
			// TTS generation endpoint
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"audio_url": "` + server.URL + `/audio.mp3", "expires_at": "2026-03-17T10:00:00Z"}`))
		} else {
			// Audio download endpoint
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write(audioData)
		}
	}))
	defer server.Close()

	store := &mockStorage{
		cacheErr: os.ErrNotExist, // Cache miss
	}

	entry := &model.Entry{
		ID:      123,
		Title:   "Test Title",
		Content: "Test content",
		Feed:    &model.Feed{Language: "en"},
	}

	client := NewClient(server.URL+"/tts", "test-key", "alloy")

	result, err := GetOrGenerateAudio(store, client, entry, 456, 24*time.Hour, tmpDir, "en")
	if err != nil {
		t.Fatalf("GetOrGenerateAudio failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, result.FilePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Error("Expected audio file to be created")
	}

	// Verify file content
	content, _ := os.ReadFile(fullPath)
	if string(content) != string(audioData) {
		t.Errorf("Expected %s, got %s", audioData, content)
	}

	// Verify cache was stored
	if store.cache == nil {
		t.Error("Expected cache to be stored")
	}
}

func TestGetOrGenerateAudio_ContentTooLarge(t *testing.T) {
	tmpDir := t.TempDir()

	store := &mockStorage{
		cacheErr: os.ErrNotExist,
	}

	// Create entry with content > 50KB
	largeContent := strings.Repeat("a", 51000)
	entry := &model.Entry{
		ID:      123,
		Title:   "Test",
		Content: largeContent,
		Feed:    &model.Feed{Language: "en"},
	}

	client := NewClient("", "", "")

	_, err := GetOrGenerateAudio(store, client, entry, 456, 24*time.Hour, tmpDir, "en")
	if err == nil {
		t.Error("Expected error for content too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tts -v -run TestGetOrGenerateAudio`
Expected: FAIL with "undefined: GetOrGenerateAudio"

- [ ] **Step 3: Implement cache logic (part 1 - interfaces and locks)**

Create `internal/tts/cache.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"miniflux.app/v2/internal/model"
)

// Storage interface for cache operations.
type Storage interface {
	GetTTSCache(entryID, userID int64) (*model.TTSCache, error)
	CreateTTSCache(cache *model.TTSCache) error
}

var (
	generationLocks = make(map[string]*sync.Mutex)
	locksMapMutex   sync.Mutex
)

// getLock gets or creates a mutex for a specific entry/user combination.
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

- [ ] **Step 4: Implement GetOrGenerateAudio function**

Add to `internal/tts/cache.go`:

```go
// GetOrGenerateAudio retrieves cached audio or generates new audio.
func GetOrGenerateAudio(
	store Storage,
	client *Client,
	entry *model.Entry,
	userID int64,
	cacheDuration time.Duration,
	storagePath string,
	defaultLanguage string,
) (*AudioResult, error) {
	// Acquire lock for this entry/user pair
	lock := getLock(entry.ID, userID)
	lock.Lock()
	defer lock.Unlock()

	// Check cache
	cached, err := store.GetTTSCache(entry.ID, userID)
	if err == nil && cached.ExpiresAt.After(time.Now()) {
		// Verify file exists
		fullPath := filepath.Join(storagePath, cached.FilePath)
		if _, err := os.Stat(fullPath); err == nil {
			return &AudioResult{
				FilePath:  cached.FilePath,
				ExpiresAt: cached.ExpiresAt,
			}, nil
		}
	}

	// Cache miss - generate new audio
	// Validate content length
	text := entry.Title + "\n\n" + entry.Content
	if len(text) > maxContentLength {
		return nil, fmt.Errorf("entry content too large for TTS (%d > %d characters)", len(text), maxContentLength)
	}

	// Detect language
	language := DetectLanguage(entry, defaultLanguage)

	// Call TTS service
	serviceResult, err := client.Generate(text, language)
	if err != nil {
		return nil, fmt.Errorf("TTS generation failed: %w", err)
	}

	// Download audio file
	audioData, err := client.DownloadAudio(serviceResult.AudioURL)
	if err != nil {
		return nil, fmt.Errorf("audio download failed: %w", err)
	}

	// Save locally
	filename := fmt.Sprintf("%d_%d_%d.mp3", entry.ID, userID, time.Now().Unix())
	relPath := filepath.Join("tts_audio", filename)
	fullPath := filepath.Join(storagePath, relPath)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, audioData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write audio file: %w", err)
	}

	// Store in cache DB
	expiresAt := time.Now().Add(cacheDuration)
	if err := store.CreateTTSCache(&model.TTSCache{
		EntryID:   entry.ID,
		UserID:    userID,
		FilePath:  relPath,
		ExpiresAt: expiresAt,
	}); err != nil {
		// Log but don't fail - file is saved
		fmt.Printf("Warning: failed to cache TTS metadata: %v\n", err)
	}

	return &AudioResult{
		FilePath:  relPath,
		ExpiresAt: expiresAt,
	}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tts -v -run TestGetOrGenerateAudio`
Expected: PASS (all cache tests)

- [ ] **Step 6: Commit**

```bash
git add internal/tts/cache.go internal/tts/cache_test.go
git commit -m "feat(tts): add cache management with file storage

Implement cache logic:
- GetOrGenerateAudio(): main orchestrator function
- Cache lookup with file existence verification
- On cache miss: generate, download, save locally
- Concurrent request handling with per-entry locks
- File storage in configurable directory
- Database cache metadata with expiry"
```

---

## Chunk 4: API Endpoints & Audio Serving

### Task 4.1: Add TTS Generation API Endpoint

**Files:**
- Create: `internal/api/entry_tts.go`
- Create: `internal/api/entry_tts_test.go`

- [ ] **Step 1: Write test for TTS generation endpoint**

Create `internal/api/entry_tts_test.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

func TestGenerateTTSAudio_ConfigDisabled(t *testing.T) {
	// Save original config
	origEnabled := config.Opts.TTSEnabled()
	defer func() {
		// Restore would require modifying config - skip for unit test
	}()

	store := &storage.Storage{} // Mock storage
	pool := nil
	router := setupTestRouter(store, pool)

	req := httptest.NewRequest("POST", "/v1/entries/123/tts", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestGenerateTTSAudio_EntryNotFound(t *testing.T) {
	// Note: Full integration test would require test database
	// This is a placeholder for the test structure
	t.Skip("Requires test database setup")
}

func TestGenerateTTSAudio_Success(t *testing.T) {
	// Note: Full integration test would require:
	// - Test database with entry
	// - Mock TTS service
	// - Temporary file storage
	t.Skip("Requires full integration test setup")
}
```

- [ ] **Step 2: Run test to verify structure**

Run: `go test ./internal/api -v -run TestGenerateTTSAudio`
Expected: Tests run (may skip, but no compile errors)

- [ ] **Step 3: Implement TTS generation endpoint**

Create `internal/api/entry_tts.go`:

```go
// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"time"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response/json"
	"miniflux.app/v2/internal/tts"
)

// getTTSAudio generates or retrieves cached TTS audio for an entry.
func (h *handler) getTTSAudio(w http.ResponseWriter, r *http.Request) {
	// Check if TTS is enabled
	if !config.Opts.TTSEnabled() {
		json.Forbidden(w, r)
		return
	}

	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")

	// Fetch entry with feed details
	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryID(entryID)
	builder.WithoutStatus(model.EntryStatusRemoved)

	entry, err := builder.GetEntry()
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	if entry == nil {
		json.NotFound(w, r)
		return
	}

	// Check rate limit
	rateLimitPerHour := config.Opts.TTSRateLimitPerHour()
	if !tts.AllowRequest(userID, rateLimitPerHour) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		json.TooManyRequests(w, r)
		return
	}

	// Initialize TTS client
	client := tts.NewClient(
		config.Opts.TTSEndpointURL(),
		config.Opts.TTSAPIKey(),
		config.Opts.TTSVoice(),
	)

	// Get or generate audio
	cacheDuration := time.Duration(config.Opts.TTSCacheDuration()) * time.Hour
	storagePath := config.Opts.TTSStoragePath()
	defaultLanguage := config.Opts.TTSDefaultLanguage()

	result, err := tts.GetOrGenerateAudio(
		h.store,
		client,
		entry,
		userID,
		cacheDuration,
		storagePath,
		defaultLanguage,
	)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	// Extract filename from result FilePath
	filename := filepath.Base(result.FilePath)

	// Return audio file URL (will be served via separate endpoint)
	json.OK(w, r, map[string]any{
		"audio_url": "/tts/audio/" + filename,
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
	})
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build -o /dev/null ./internal/api`
Expected: No errors (may have unused imports to fix)

- [ ] **Step 5: Fix imports if needed**

Add missing imports to `internal/api/entry_tts.go`:
```go
import (
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response/json"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/tts"
)
```

- [ ] **Step 6: Commit**

```bash
git add internal/api/entry_tts.go internal/api/entry_tts_test.go
git commit -m "feat(api): add TTS generation endpoint

Add POST /v1/entries/{entryID}/tts endpoint:
- Check TTS enabled flag
- Verify user has access to entry
- Apply per-user rate limiting
- Generate or retrieve cached audio
- Return audio URL and expiry time"
```

### Task 4.2: Add Audio File Serving Endpoint

**Files:**
- Modify: `internal/api/entry_tts.go`

- [ ] **Step 1: Add test for audio serving**

Add to `internal/api/entry_tts_test.go`:

```go
func TestServeTTSAudio_NotFound(t *testing.T) {
	t.Skip("Requires test database setup")
}

func TestServeTTSAudio_Success(t *testing.T) {
	t.Skip("Requires test database setup")
}
```

- [ ] **Step 2: Implement audio file serving**

Add to `internal/api/entry_tts.go`:

```go
import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/http/response/json"
	"miniflux.app/v2/internal/model"
)

// serveTTSAudioFile serves the cached TTS audio file.
func (h *handler) serveTTSAudioFile(w http.ResponseWriter, r *http.Request) {
	// Check if TTS is enabled
	if !config.Opts.TTSEnabled() {
		json.Forbidden(w, r)
		return
	}

	userID := request.UserID(r)
	filename := request.RouteStringParam(r, "filename")

	// Parse entry_id and user_id from filename format: {entry_id}_{user_id}_{timestamp}.mp3
	// Validate filename to prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		json.BadRequest(w, r, errors.New("invalid filename"))
		return
	}

	// Parse entry and user IDs from filename
	parts := strings.Split(strings.TrimSuffix(filename, ".mp3"), "_")
	if len(parts) != 3 {
		json.BadRequest(w, r, errors.New("invalid filename format"))
		return
	}

	entryID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		json.BadRequest(w, r, errors.New("invalid entry ID"))
		return
	}

	fileUserID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		json.BadRequest(w, r, errors.New("invalid user ID"))
		return
	}

	// Verify requesting user matches file owner
	if userID != fileUserID {
		json.Forbidden(w, r)
		return
	}

	// Verify user still has access to entry
	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryID(entryID)
	builder.WithoutStatus(model.EntryStatusRemoved)

	entry, err := builder.GetEntry()
	if err != nil || entry == nil {
		json.Forbidden(w, r)
		return
	}

	// Build full file path
	storagePath := config.Opts.TTSStoragePath()
	relPath := filepath.Join("tts_audio", filename)
	fullPath := filepath.Join(storagePath, relPath)

	// Check file exists
	_, err = os.Stat(fullPath)
	if err != nil {
		json.NotFound(w, r)
		return
	}

	// Open file
	file, err := os.Open(fullPath)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}
	defer file.Close()

	// Serve file
	response.New(w, r).WithCaching(filename, 24*time.Hour, func(b *response.Builder) {
		b.WithHeader("Content-Type", "audio/mpeg")
		b.WithHeader("Content-Disposition", `inline; filename="`+filename+`"`)
		b.WithBody(file)
		b.WithoutCompression()
		b.Write()
	})
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build -o /dev/null ./internal/api`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/api/entry_tts.go internal/api/entry_tts_test.go
git commit -m "feat(api): add TTS audio file serving

Add GET /v1/entries/{entryID}/tts/audio endpoint:
- Verify cache entry exists and not expired
- Serve audio file from local storage
- Set Content-Type: audio/mpeg
- Enable browser caching (24h)
- Return 404 if file missing or expired"
```

### Task 4.3: Register TTS Routes

**Files:**
- Modify: `internal/api/api.go`

- [ ] **Step 1: Add TTS routes to API router**

Edit `internal/api/api.go`, add after line ~73 (after entry routes):

```go
sr.HandleFunc("/entries/{entryID}/tts", handler.getTTSAudio).Methods(http.MethodGet)
sr.HandleFunc("/tts/audio/{filename}", handler.serveTTSAudioFile).Methods(http.MethodGet)
```

- [ ] **Step 2: Verify compilation**

Run: `go build -o /dev/null ./internal/api`
Expected: No errors

- [ ] **Step 3: Test route registration**

Run: `go run . -info 2>&1 | grep -i api`
Expected: No errors (routes registered)

- [ ] **Step 4: Commit**

```bash
git add internal/api/api.go
git commit -m "feat(api): register TTS endpoints

Register TTS API routes:
- POST /v1/entries/{entryID}/tts - generate audio
- GET /v1/entries/{entryID}/tts/audio - serve audio file"
```

---

## Chunk 5: UI Implementation

### Task 5.1: Add TTS Button to Entry List View

**Files:**
- Modify: `internal/template/templates/common/item_meta.html`

- [ ] **Step 1: Add TTS button to item_meta template**

Edit `internal/template/templates/common/item_meta.html`, add after the star button (after line 38):

```html
{{ if .hasTTS }}
<li class="item-meta-icons-tts">
    <button
        aria-describedby="entry-title-{{ .entry.ID }}"
        title="{{ t "entry.tts.title" }}"
        data-tts-button="true"
        data-entry-id="{{ .entry.ID }}"
        data-label-loading="{{ t "entry.tts.loading" }}"
        data-label-ready="{{ t "entry.tts.ready" }}"
        data-label-error="{{ t "entry.tts.error" }}"
        >{{ icon "speaker" }}<span class="icon-label">{{ t "entry.tts.label" }}</span></button>
</li>
{{ end -}}
```

- [ ] **Step 2: Verify template syntax**

Run: `go build -o /dev/null ./internal/template`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/template/templates/common/item_meta.html
git commit -m "feat(ui): add TTS button to entry list view

Add TTS button in item_meta template:
- Shown when hasTTS is true
- Icon-only button for compact display
- data-tts-button for JavaScript handling
- data-entry-id for API calls"
```

### Task 5.2: Add TTS Button to Entry Detail View

**Files:**
- Modify: `internal/template/templates/views/entry.html`

- [ ] **Step 1: Add TTS button to entry detail header**

Edit `internal/template/templates/views/entry.html`, add after the star button (~line 78):

```html
{{ if .hasTTS }}
<li>
    <button
        class="page-button"
        data-tts-button="true"
        data-entry-id="{{ .entry.ID }}"
        title="{{ t "entry.tts.title" }}"
        data-label-loading="{{ t "entry.tts.loading" }}"
        data-label-ready="{{ t "entry.tts.ready" }}"
        data-label-error="{{ t "entry.tts.error" }}"
        >{{ icon "speaker" }}<span class="icon-label">{{ t "entry.tts.label" }}</span></button>
</li>
{{ end }}
```

- [ ] **Step 2: Add inline audio player**

Add after the entry actions (after the closing `</ul>` around line 110):

```html
{{ if .hasTTS }}
<div class="tts-player" id="tts-player-{{ .entry.ID }}" style="display: none; margin-top: 1rem;">
    <audio
        controls
        preload="none"
        data-entry-id="{{ .entry.ID }}"
        style="width: 100%; max-width: 600px;"
    >
        <p>{{ t "entry.tts.unsupported" }}</p>
    </audio>
</div>
{{ end }}
```

- [ ] **Step 3: Verify template syntax**

Run: `go build -o /dev/null ./internal/template`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/template/templates/views/entry.html
git commit -m "feat(ui): add TTS button and player to entry detail view

Add to entry detail view:
- TTS button with text label in header actions
- Inline HTML5 audio player (hidden by default)
- Player shown when audio is ready"
```

### Task 5.3: Add hasTTS Flag to View Context

**Files:**
- Modify: `internal/ui/entry.go` (or similar entry view handlers)

- [ ] **Step 1: Find entry view handlers**

Run: `find internal/ui -name "*entry*.go" -type f`
Expected: List of entry-related UI files

- [ ] **Step 2: Add hasTTS to view context in all entry handlers**

Search for view.Set calls in entry handlers and add:

```go
view.Set("hasTTS", config.Opts.TTSEnabled())
```

Example locations to modify:
- `showUnreadEntry()` in `internal/ui/unread_entries.go`
- `showReadEntry()` in `internal/ui/read_entries.go`
- `showStarredEntry()` in `internal/ui/starred_entries.go`
- `showCategoryEntry()` in `internal/ui/category_entries.go`
- `showFeedEntry()` in `internal/ui/feed_entries.go`

- [ ] **Step 3: Verify compilation**

Run: `go build -o /dev/null ./internal/ui`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/ui/*entry*.go
git commit -m "feat(ui): add hasTTS flag to entry view contexts

Set hasTTS template variable based on config:
- Enables TTS button visibility in templates
- Added to all entry view handlers"
```

### Task 5.4: Add TTS JavaScript Handlers

**Files:**
- Modify: `internal/ui/static/js/app.js`

- [ ] **Step 1: Add TTS button click handler**

Add to `internal/ui/static/js/app.js` (end of file):

```javascript
// TTS button handler
document.addEventListener('DOMContentLoaded', function() {
    const ttsButtons = document.querySelectorAll('[data-tts-button]');

    ttsButtons.forEach(button => {
        button.addEventListener('click', async function(e) {
            e.preventDefault();

            const entryId = this.dataset.entryId;
            const labelLoading = this.dataset.labelLoading;
            const labelReady = this.dataset.labelReady;
            const labelError = this.dataset.labelError;

            // Check if already loaded (cached)
            const player = document.getElementById(`tts-player-${entryId}`);
            const audio = player ? player.querySelector('audio') : null;

            if (audio && audio.src) {
                // Already loaded - toggle player
                player.style.display = player.style.display === 'none' ? 'block' : 'none';
                if (player.style.display === 'block') {
                    audio.play();
                }
                return;
            }

            // Update button state to loading
            const originalHTML = this.innerHTML;
            this.disabled = true;
            this.classList.add('tts-loading');
            const iconLabel = this.querySelector('.icon-label');
            if (iconLabel) {
                iconLabel.textContent = labelLoading;
            }

            try {
                // Call API to get/generate audio
                const response = await fetch(`/v1/entries/${entryId}/tts`, {
                    method: 'GET',
                    credentials: 'same-origin'
                });

                if (response.status === 429) {
                    throw new Error('Rate limit exceeded - please try again later');
                }

                if (!response.ok) {
                    throw new Error(`TTS generation failed: ${response.statusText}`);
                }

                const data = await response.json();

                // Load audio into player
                if (audio) {
                    audio.src = data.audio_url;
                    player.style.display = 'block';
                    audio.play();
                }

                // Update button state to ready
                this.classList.remove('tts-loading');
                this.classList.add('tts-ready');
                if (iconLabel) {
                    iconLabel.textContent = labelReady;
                }

            } catch (error) {
                console.error('TTS error:', error);

                // Update button state to error
                this.classList.remove('tts-loading');
                this.classList.add('tts-error');
                if (iconLabel) {
                    iconLabel.textContent = labelError;
                }

                // Show toast notification
                if (window.showToast) {
                    window.showToast(error.message || 'TTS generation failed');
                }

            } finally {
                this.disabled = false;
            }
        });
    });
});
```

- [ ] **Step 2: Verify JavaScript syntax**

Run: `node -c internal/ui/static/js/app.js` (if node available)
Or: Visual inspection for syntax errors

- [ ] **Step 3: Commit**

```bash
git add internal/ui/static/js/app.js
git commit -m "feat(ui): add TTS JavaScript handlers

Add TTS button click handler:
- Call POST /v1/entries/{id}/tts API
- Update button states (loading, ready, error)
- Load audio into HTML5 player
- Show/hide player on click
- Handle rate limiting and errors"
```

### Task 5.5: Add TTS Button Styles

**Files:**
- Modify: `internal/ui/static/css/app.css`

- [ ] **Step 1: Add TTS button styles**

Add to `internal/ui/static/css/app.css`:

```css
/* TTS Button States */
[data-tts-button] {
    transition: all 0.2s ease;
}

[data-tts-button].tts-loading {
    border-color: #ffa726;
    background-color: #fff3e0;
    cursor: wait;
}

[data-tts-button].tts-ready {
    border-color: #66bb6a;
    background-color: #e8f5e9;
}

[data-tts-button].tts-error {
    border-color: #ef5350;
    background-color: #ffebee;
}

/* TTS Player */
.tts-player {
    padding: 1rem 0;
}

.tts-player audio {
    border-radius: 4px;
}
```

- [ ] **Step 2: Verify CSS syntax**

Visual inspection for syntax errors

- [ ] **Step 3: Commit**

```bash
git add internal/ui/static/css/app.css
git commit -m "feat(ui): add TTS button styles

Add visual states for TTS button:
- Loading: orange border, light orange background
- Ready: green border, light green background
- Error: red border, light red background
- Smooth transitions between states"
```

### Task 5.6: Add TTS Translation Keys

**Files:**
- Modify: `internal/locale/translations/en_US.toml` (and other language files)

- [ ] **Step 1: Add English translation keys**

Edit `internal/locale/translations/en_US.toml`, add in the `[entry]` section:

```toml
[entry.tts]
title = "Read aloud"
label = "Read Aloud"
loading = "Generating audio..."
ready = "Play audio"
error = "TTS failed - click to retry"
unsupported = "Your browser does not support audio playback."
```

- [ ] **Step 2: Verify TOML syntax**

Run: `go build -o /dev/null ./internal/locale`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/locale/translations/en_US.toml
git commit -m "feat(locale): add TTS translation keys

Add English translations for TTS feature:
- Button labels for all states
- Accessibility titles
- Error messages"
```

---

## Chunk 6: Scheduler Integration & Icon

### Task 6.1: Add Speaker Icon to SVG Sprite

**Files:**
- Modify: `internal/ui/static/bin/sprite.svg`

- [ ] **Step 1: Add speaker icon to sprite**

Edit `internal/ui/static/bin/sprite.svg`, add before the closing `</svg>` tag:

```xml
<symbol id="icon-speaker" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
    <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
    <path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path>
    <path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path>
</symbol>
```

- [ ] **Step 2: Verify SVG syntax**

Visual inspection for SVG syntax errors

- [ ] **Step 3: Test icon displays**

Run app and check if `{{ icon "speaker" }}` renders correctly in templates

- [ ] **Step 4: Commit**

```bash
git add internal/ui/static/bin/sprite.svg
git commit -m "feat(ui): add speaker icon to SVG sprite

Add speaker icon for TTS button:
- SVG path for speaker with sound waves
- Matches existing icon style (24x24, stroke-based)"
```

### Task 6.2: Add TTS Cache Cleanup to Scheduler

**Files:**
- Modify: `internal/cli/cleanup_tasks.go`
- Modify: `internal/storage/tts_cache.go`

- [ ] **Step 1: Add cleanup method to storage**

Add to `internal/storage/tts_cache.go`:

```go
// CleanupExpiredTTSCache deletes expired TTS cache entries and returns file paths.
func (s *Storage) CleanupExpiredTTSCache() ([]string, error) {
	query := `
		DELETE FROM tts_audio_cache
		WHERE expires_at < NOW()
		RETURNING file_path
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("unable to delete expired TTS cache: %w", err)
	}
	defer rows.Close()

	var filePaths []string
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, fmt.Errorf("unable to scan TTS cache file path: %w", err)
		}
		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}
```

- [ ] **Step 2: Add TTS cleanup to cleanup tasks**

Edit `internal/cli/cleanup_tasks.go`, add after the session cleanup (after line 22):

```go
// TTS cache cleanup
if config.Opts.TTSEnabled() {
	filePaths, err := store.CleanupExpiredTTSCache()
	if err != nil {
		slog.Error("Unable to cleanup expired TTS cache", slog.Any("error", err))
	} else if len(filePaths) > 0 {
		// Delete physical files
		storagePath := config.Opts.TTSStoragePath()
		deletedCount := 0
		for _, relPath := range filePaths {
			fullPath := filepath.Join(storagePath, relPath)
			if err := os.Remove(fullPath); err != nil {
				slog.Warn("Unable to delete TTS audio file",
					slog.String("file_path", fullPath),
					slog.Any("error", err),
				)
			} else {
				deletedCount++
			}
		}
		slog.Info("TTS cache cleanup completed",
			slog.Int("database_entries_removed", len(filePaths)),
			slog.Int("files_deleted", deletedCount),
		)
	}
}
```

- [ ] **Step 3: Add required imports**

Add to imports in `internal/cli/cleanup_tasks.go`:

```go
import (
	"os"
	"path/filepath"
)
```

- [ ] **Step 4: Verify compilation**

Run: `go build -o /dev/null ./internal/cli`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/storage/tts_cache.go internal/cli/cleanup_tasks.go
git commit -m "feat(scheduler): add TTS cache cleanup job

Add scheduled cleanup for expired TTS audio:
- CleanupExpiredTTSCache() removes DB entries
- Cleanup task deletes physical audio files
- Runs with other cleanup tasks (default: hourly)
- Logs success and errors for monitoring"
```

### Task 6.3: Final Integration Test

**Files:**
- N/A (Manual testing)

- [ ] **Step 1: Build the application**

Run: `go build -o miniflux-test .`
Expected: Successful build

- [ ] **Step 2: Run migration**

Run: `./miniflux-test -migrate`
Expected: Migration runs successfully, tts_audio_cache table created

- [ ] **Step 3: Start application with TTS enabled**

Run with environment variables:
```bash
TTS_ENABLED=true \
TTS_ENDPOINT_URL=http://localhost:8000/tts \
TTS_API_KEY=test-key \
TTS_VOICE=alloy \
TTS_STORAGE_PATH=/tmp/miniflux-tts \
./miniflux-test
```

Expected: Application starts without errors

- [ ] **Step 4: Verify TTS button appears in UI**

1. Open browser to Miniflux
2. Navigate to unread entries
3. Check that speaker icon appears next to entries

Expected: TTS button visible (if TTS_ENABLED=true)

- [ ] **Step 5: Test TTS generation (with mock service)**

1. Click TTS button
2. Check browser console for API call to `/v1/entries/{id}/tts`

Expected:
- Button shows loading state
- API call made to POST /v1/entries/{id}/tts
- If mock service available: audio player appears

- [ ] **Step 6: Verify cleanup job**

Check logs after cleanup frequency passes (default: 1 hour)

Expected: Log message "TTS cache cleanup completed" with counts

- [ ] **Step 7: Clean up test build**

Run: `rm miniflux-test`

### Task 6.4: Documentation Update

**Files:**
- Create: `docs/configuration.md` (or update existing)

- [ ] **Step 1: Document TTS configuration options**

Add to configuration documentation:

```markdown
## Text-to-Speech (TTS) Configuration

Miniflux supports text-to-speech audio generation for feed entries via external TTS services.

### Environment Variables

- `TTS_ENABLED`: Enable/disable TTS feature (default: `false`)
- `TTS_ENDPOINT_URL`: URL of your TTS wrapper service endpoint
- `TTS_API_KEY`: API key for authentication with TTS service (or use `TTS_API_KEY_FILE` for secret file)
- `TTS_API_KEY_FILE`: Path to file containing API key (alternative to `TTS_API_KEY`)
- `TTS_VOICE`: Voice identifier to use (e.g., `alloy`, `echo`, `nova`)
- `TTS_STORAGE_PATH`: Local directory for caching audio files (default: `./tts_audio`)
- `TTS_CACHE_DURATION`: Cache duration in hours (default: `24`)
- `TTS_DEFAULT_LANGUAGE`: Fallback language code (default: `en`)
- `TTS_RATE_LIMIT_PER_HOUR`: Max TTS requests per user per hour (default: `10`)

### TTS Wrapper Service

Miniflux requires a wrapper service that implements the following HTTP contract:

**Request:**
```json
POST {TTS_ENDPOINT_URL}
Authorization: Bearer {TTS_API_KEY}
Content-Type: application/json

{
  "text": "Article title and content...",
  "language": "en",
  "voice": "alloy"
}
```

**Response:**
```json
{
  "audio_url": "https://your-service.com/audio/xyz.mp3",
  "expires_at": "2026-03-17T10:00:00Z"
}
```

See spec document for reference wrapper implementations.

### Example Configuration

```bash
export TTS_ENABLED=true
export TTS_ENDPOINT_URL=https://tts-wrapper.example.com/generate
export TTS_API_KEY_FILE=/run/secrets/tts_api_key
export TTS_VOICE=alloy
export TTS_STORAGE_PATH=/var/lib/miniflux/tts_audio
export TTS_CACHE_DURATION=48
export TTS_RATE_LIMIT_PER_HOUR=5
```
```

- [ ] **Step 2: Commit documentation**

```bash
git add docs/configuration.md
git commit -m "docs: add TTS configuration documentation

Document TTS feature configuration:
- All environment variables explained
- TTS wrapper service HTTP contract
- Example configuration
- Reference to spec for wrapper implementations"
```

---

## Implementation Complete

All chunks of the TTS feature implementation plan are now complete. The plan covers:

**Chunk 1:** Foundation (Config, Models, Migrations) - 3 tasks
**Chunk 2:** TTS Core (Language, Rate Limit, Client) - 3 tasks
**Chunk 3:** Cache & Storage - 2 tasks
**Chunk 4:** API Endpoints - 3 tasks
**Chunk 5:** UI Implementation - 6 tasks
**Chunk 6:** Scheduler & Icon - 4 tasks

**Total:** 21 tasks with ~150+ individual steps

The plan follows TDD principles with:
- Write test → Run test (fail) → Implement → Run test (pass) → Commit
- Frequent small commits (one per task)
- Complete code provided in plan
- Verification steps for each task
- Integration testing at the end

**Next step:** Execute this plan using `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`.

