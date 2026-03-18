# Cloudflare R2 Storage Backend for TTS Audio

**Date:** 2026-03-18
**Status:** Design Review
**Author:** Claude Code

## Overview

Add Cloudflare R2 object storage as a configurable backend for TTS audio files. Users can choose between local filesystem storage (current behavior) and Cloudflare R2 via environment variables. This enables scalable, distributed storage for TTS audio without managing local disk space.

## Goals

- Support Cloudflare R2 as a storage backend for generated TTS audio files
- Maintain backward compatibility with existing local filesystem storage
- Use presigned URLs for R2 to serve audio directly from R2 to users
- Keep storage backend configurable via environment variables
- No database schema changes required

## Non-Goals

- Automatic migration of existing audio files between backends
- Fallback between storage backends at runtime
- Support for other object storage providers (S3, GCS, Azure) in this iteration
- Mixed storage (all files use same backend)

## Design Decisions

### Decision 1: Storage Backend Selection

**Choice:** Configurable via environment variable with global scope.

Users set `TTS_STORAGE_BACKEND` to either `local` or `r2`. All audio files use the same backend - no mixing. This keeps the implementation simple and migration story clear.

**Alternatives Considered:**
- Per-file backend tracking in database - rejected for added complexity
- Automatic fallback - rejected to avoid unexpected behavior

### Decision 2: URL Generation Strategy

**Choice:** Generate presigned URLs for R2 that expire at cache expiration time.

When using R2, the application generates time-limited presigned URLs that allow direct download from R2. URLs expire at the same time as the cache entry, maintaining consistency.

**Alternatives Considered:**
- Short-lived URLs (1 hour) - rejected due to extra R2 API calls per request
- Proxy through Miniflux - rejected due to higher server load
- Public bucket - rejected due to lack of access control

### Decision 3: Database Schema

**Choice:** No schema changes, use existing `file_path` column for object keys.

The `file_path` column stores relative paths for both backends:
- Local: `tts_audio/123_456_1234567890.mp3` (relative to base directory)
- R2: `tts_audio/123_456_1234567890.mp3` (S3 object key within bucket)

This allows zero-downtime migration and clean backward compatibility.

### Decision 4: Architecture Pattern

**Choice:** Storage interface pattern with separate implementations.

Create an `AudioStorage` interface with `LocalStorage` and `R2Storage` implementations. This follows Go best practices, makes testing easier, and future-proofs for additional backends.

**Alternatives Considered:**
- Conditional logic in cache layer - rejected for maintainability concerns
- Provider pattern (mirror TTS providers) - rejected as too heavy for simple storage operations

## Architecture

### Core Abstraction

```go
type AudioStorage interface {
    Save(data []byte, path string) error
    GetURL(path string, expiresAt time.Time) (string, error)
    Delete(path string) error
}
```

### Implementations

**LocalStorage:**
- Saves files to filesystem using `os.WriteFile()`
- Returns local file paths
- Delete removes files from disk

**R2Storage:**
- Uses AWS S3 SDK v2 (R2 is S3-compatible)
- Uploads files via `PutObject`
- Generates presigned URLs via `PresignGetObject`
- Deletes via `DeleteObject`

### Factory Function

```go
func NewAudioStorage(config *StorageConfig) (AudioStorage, error)
```

Returns appropriate implementation based on `config.Backend`. Validates R2 credentials when R2 is selected.

## Configuration

### New Environment Variables

```
TTS_STORAGE_BACKEND
  Description: Storage backend for TTS audio files
  Default: "local"
  Values: "local", "r2"

TTS_STORAGE_PATH
  Description: Base storage path (filesystem dir for local, bucket name for R2)
  Default: "" (uses DATA_DIR for local, required for R2)

TTS_R2_ENDPOINT
  Description: Cloudflare R2 endpoint URL
  Default: ""
  Example: "https://<account-id>.r2.cloudflarestorage.com"
  Required: when TTS_STORAGE_BACKEND=r2

TTS_R2_ACCESS_KEY_ID
  Description: R2 access key ID
  Default: ""
  Required: when TTS_STORAGE_BACKEND=r2

TTS_R2_SECRET_ACCESS_KEY
  Description: R2 secret access key
  Default: ""
  Required: when TTS_STORAGE_BACKEND=r2

TTS_R2_BUCKET
  Description: R2 bucket name for audio storage
  Default: ""
  Required: when TTS_STORAGE_BACKEND=r2

TTS_R2_PUBLIC_URL
  Description: Public R2 URL for presigned URLs (optional)
  Default: "" (uses TTS_R2_ENDPOINT if not set)
  Example: "https://r2.example.com" (if using custom domain)
```

### Configuration Validation

When `TTS_STORAGE_BACKEND=r2`:
- Validate all required R2 settings are present
- Fail at startup with clear error if missing: "TTS R2 storage enabled but TTS_R2_BUCKET not configured"

### Backward Compatibility

Existing installations without `TTS_STORAGE_BACKEND` default to `local` and continue working unchanged.

## Implementation Structure

### New Files

**`internal/tts/storage.go`**
- `AudioStorage` interface definition
- `StorageConfig` struct
- `NewAudioStorage()` factory function

**`internal/tts/storage_local.go`**
- `LocalStorage` struct and implementation
- File creation, path handling, deletion

**`internal/tts/storage_r2.go`**
- `R2Storage` struct and implementation
- S3 client initialization
- Upload, presigned URL generation, deletion

**`internal/tts/storage_test.go`**
- Unit tests for both implementations
- Mock interface tests
- Integration tests (local with temp dir, R2 with mock)

### Modified Files

**`internal/config/options.go`**
- Add all TTS storage configuration options
- Add validation function for TTS_STORAGE_BACKEND
- Add accessor methods

**`internal/tts/cache.go`**
- Accept `AudioStorage` parameter in `GetOrGenerateAudio()`
- Replace `os.WriteFile()` with `storage.Save()`
- Replace path construction with `storage.GetURL()`
- Remove direct filesystem operations

**`internal/api/entry_tts.go`**
- Initialize `AudioStorage` from config at startup
- Pass storage instance to `GetOrGenerateAudio()`
- For local: serve file from disk (current behavior)
- For R2: return HTTP 302 redirect to presigned URL

## Data Flow

### TTS Audio Request (R2 Backend)

1. User requests: `GET /v1/entries/{entryID}/tts`
2. API handler checks cache database
3. **Cache Hit:**
   - Call `storage.GetURL(filePath, expiresAt)`
   - R2Storage generates presigned URL valid until `expiresAt`
   - Return HTTP 302 redirect to presigned URL
   - User downloads directly from R2
4. **Cache Miss:**
   - Acquire generation lock
   - TTS provider generates audio bytes
   - Call `storage.Save(audioData, "tts_audio/123_456_789.mp3")`
   - R2Storage uploads to R2
   - Save cache entry with object key in `file_path`
   - Generate presigned URL and redirect user
   - Release lock

### TTS Audio Request (Local Backend)

1. User requests: `GET /v1/entries/{entryID}/tts`
2. API handler checks cache database
3. **Cache Hit:**
   - Call `storage.GetURL(filePath, expiresAt)`
   - LocalStorage returns file path
   - Handler reads file and streams bytes to user
4. **Cache Miss:**
   - Same provider generation
   - LocalStorage writes to filesystem
   - Handler serves file from disk

### Cleanup Flow (Expired Cache)

1. Scheduled job calls `CleanupExpiredTTSCache()`
2. Database deletes expired entries, returns `file_path` list
3. For each path: call `storage.Delete(path)`
   - LocalStorage: removes file from disk
   - R2Storage: calls S3 DeleteObject
4. Continue cleanup even if individual deletes fail

## Error Handling

### Startup Validation

- Validate R2 credentials when `TTS_STORAGE_BACKEND=r2`
- Fail fast with clear error messages
- Optional: test R2 connectivity by listing bucket

### Runtime Errors

**R2 Upload Failure:**
- Log error with context (entry ID, user ID, error)
- Return error to user: "Failed to save audio to storage"
- Do NOT save cache entry if upload fails
- User retries on next request

**Presigned URL Generation Failure:**
- Log error and return HTTP 500
- Should be rare (credentials invalid)
- Cache entry exists but can't be served

**R2 Delete Failure (cleanup):**
- Log warning but continue cleanup
- Database entry removed even if R2 delete fails
- Orphaned objects cleaned up eventually

**Local Filesystem Errors:**
- Disk full: same as R2 upload failure
- Permission errors: log and return error
- Directory creation failure: fail request

### Error Messages

- User-facing: "Audio generation failed, please try again"
- Logs: detailed error with operation, entry ID, user ID, underlying error

### Graceful Degradation

- No automatic fallback between backends
- Storage unavailable = TTS requests fail with clear error
- Existing cache entries can be served if storage temporarily down

## Testing Strategy

### Unit Tests

- Mock `AudioStorage` interface in cache tests
- Test `LocalStorage` with temp directories
- Test `R2Storage` with mock S3 client
- Test factory function with various configs

### Integration Tests

- Local storage: write/read/delete with real filesystem
- R2 storage: test with R2-compatible mock (MinIO)
- Test presigned URL generation and expiration

### Manual Testing

- Deploy with R2 backend
- Verify presigned URLs work
- Verify URLs expire at correct time
- Test cleanup job removes R2 objects
- Test migration scenario (local to R2)

## Migration Path

### Local to R2 Migration

1. Set up R2 bucket and credentials
2. Update environment variables:
   ```
   TTS_STORAGE_BACKEND=r2
   TTS_R2_ENDPOINT=https://...
   TTS_R2_ACCESS_KEY_ID=...
   TTS_R2_SECRET_ACCESS_KEY=...
   TTS_R2_BUCKET=miniflux-tts
   ```
3. Restart Miniflux
4. New audio goes to R2
5. Old local files expire naturally based on cache duration
6. Optional: manually delete old local files after expiration

### R2 to Local Migration

1. Update `TTS_STORAGE_BACKEND=local`
2. Restart Miniflux
3. New audio goes to local filesystem
4. Old R2 objects expire based on cache duration
5. Optional: manually delete old R2 objects after expiration

No data loss occurs - expired cache entries are regenerated on demand.

## Security Considerations

- R2 credentials stored in environment variables (standard practice)
- Presigned URLs have time-limited access (expire with cache)
- No public bucket access - all access via presigned URLs
- Object keys use entry ID + user ID + timestamp (not guessable)
- Validate audio file sizes to prevent quota exhaustion

## Performance Considerations

- Presigned URL generation is fast (local crypto operation)
- R2 uploads are async from user perspective (background after TTS)
- Direct download from R2 reduces server bandwidth
- No proxy overhead for audio streaming
- R2 provides global CDN for fast downloads

## Dependencies

**New Dependency:**
- `github.com/aws/aws-sdk-go-v2` - Official AWS SDK for Go
- `github.com/aws/aws-sdk-go-v2/service/s3` - S3 client
- `github.com/aws/aws-sdk-go-v2/credentials` - Credential provider
- `github.com/aws/aws-sdk-go-v2/config` - AWS config

These are industry-standard, well-maintained libraries with broad adoption.

## Future Enhancements

- Support for other S3-compatible providers (AWS S3, DigitalOcean Spaces)
- Support for Google Cloud Storage
- Support for Azure Blob Storage
- Automatic migration tool between backends
- Bucket lifecycle policies for automatic cleanup
- CDN integration for R2 custom domains

## Open Questions

None - all design decisions approved.

## References

- Cloudflare R2 Documentation: https://developers.cloudflare.com/r2/
- AWS S3 SDK for Go: https://aws.github.io/aws-sdk-go-v2/docs/
- Existing TTS implementation: `internal/tts/`
