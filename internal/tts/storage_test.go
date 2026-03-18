// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Mock implementation for testing
type mockAudioStorage struct {
	saveCalled   bool
	getURLCalled bool
	deleteCalled bool
}

func (m *mockAudioStorage) Save(data []byte, path string) error {
	m.saveCalled = true
	return nil
}

func (m *mockAudioStorage) GetURL(path string, expiresAt time.Time) (string, error) {
	m.getURLCalled = true
	return "http://example.com/" + path, nil
}

func (m *mockAudioStorage) Delete(path string) error {
	m.deleteCalled = true
	return nil
}

func TestAudioStorageInterface(t *testing.T) {
	var _ AudioStorage = &mockAudioStorage{}

	mock := &mockAudioStorage{}

	err := mock.Save([]byte("test"), "test.mp3")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if !mock.saveCalled {
		t.Error("Save was not called")
	}

	url, err := mock.GetURL("test.mp3", time.Now())
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}
	if url == "" {
		t.Error("GetURL returned empty string")
	}
	if !mock.getURLCalled {
		t.Error("GetURL was not called")
	}

	err = mock.Delete("test.mp3")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !mock.deleteCalled {
		t.Error("Delete was not called")
	}
}

func TestNewAudioStorage_UnsupportedBackend(t *testing.T) {
	config := &StorageConfig{
		Backend: "invalid",
	}

	_, err := NewAudioStorage(config)
	if err == nil {
		t.Fatal("Expected error for unsupported backend")
	}

	expected := "unsupported storage backend: invalid"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestNewAudioStorage_Local(t *testing.T) {
	config := &StorageConfig{
		Backend:  "local",
		BasePath: "/tmp/miniflux-test",
	}

	storage, err := NewAudioStorage(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if storage == nil {
		t.Fatal("Expected storage to be created")
	}

	// Verify it's a LocalStorage type
	_, ok := storage.(*LocalStorage)
	if !ok {
		t.Error("Expected LocalStorage instance")
	}
}

func TestNewAudioStorage_R2(t *testing.T) {
	config := &StorageConfig{
		Backend:           "r2",
		R2Endpoint:        "https://test.r2.cloudflarestorage.com",
		R2AccessKeyID:     "test-key",
		R2SecretAccessKey: "test-secret",
		R2Bucket:          "test-bucket",
	}

	storage, err := NewAudioStorage(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if storage == nil {
		t.Fatal("Expected storage to be created")
	}

	// Verify it's an R2Storage type
	_, ok := storage.(*R2Storage)
	if !ok {
		t.Error("Expected R2Storage instance")
	}
}

func TestLocalStorage_SaveGetURLDelete(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	config := &StorageConfig{
		Backend:  "local",
		BasePath: tempDir,
	}

	storage, err := NewAudioStorage(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Test Save
	testData := []byte("test audio data")
	testPath := "tts_audio/test_123_456.mp3"

	err = storage.Save(testData, testPath)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tempDir, testPath)
	savedData, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if string(savedData) != string(testData) {
		t.Errorf("Saved data mismatch: got %q, want %q", savedData, testData)
	}

	// Test GetURL
	expiresAt := time.Now().Add(1 * time.Hour)
	url, err := storage.GetURL(testPath, expiresAt)
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}

	if url != testPath {
		t.Errorf("GetURL returned %q, want %q", url, testPath)
	}

	// Test Delete
	err = storage.Delete(testPath)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Error("File was not deleted")
	}
}

func TestLocalStorage_GetURL_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	storage := newLocalStorage(&StorageConfig{
		Backend:  "local",
		BasePath: tempDir,
	})

	_, err := storage.GetURL("nonexistent/file.mp3", time.Now())
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestLocalStorage_Delete_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()

	storage := newLocalStorage(&StorageConfig{
		Backend:  "local",
		BasePath: tempDir,
	})

	// Deleting non-existent file should not error
	err := storage.Delete("nonexistent/file.mp3")
	if err != nil {
		t.Errorf("Delete of non-existent file should not error: %v", err)
	}
}

func TestNewR2Storage_MissingEndpoint(t *testing.T) {
	config := &StorageConfig{
		Backend:           "r2",
		R2AccessKeyID:     "test-key",
		R2SecretAccessKey: "test-secret",
		R2Bucket:          "test-bucket",
	}

	_, err := newR2Storage(config)
	if err == nil {
		t.Fatal("Expected error for missing endpoint")
	}

	if err.Error() != "R2 endpoint is required" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNewR2Storage_MissingAccessKey(t *testing.T) {
	config := &StorageConfig{
		Backend:           "r2",
		R2Endpoint:        "https://test.r2.cloudflarestorage.com",
		R2SecretAccessKey: "test-secret",
		R2Bucket:          "test-bucket",
	}

	_, err := newR2Storage(config)
	if err == nil {
		t.Fatal("Expected error for missing access key")
	}

	if err.Error() != "R2 access key ID is required" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNewR2Storage_MissingBucket(t *testing.T) {
	config := &StorageConfig{
		Backend:           "r2",
		R2Endpoint:        "https://test.r2.cloudflarestorage.com",
		R2AccessKeyID:     "test-key",
		R2SecretAccessKey: "test-secret",
	}

	_, err := newR2Storage(config)
	if err == nil {
		t.Fatal("Expected error for missing bucket")
	}

	if err.Error() != "R2 bucket is required" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestR2Storage_GetURL_PastExpiration(t *testing.T) {
	config := &StorageConfig{
		Backend:           "r2",
		R2Endpoint:        "https://test.r2.cloudflarestorage.com",
		R2AccessKeyID:     "test-key",
		R2SecretAccessKey: "test-secret",
		R2Bucket:          "test-bucket",
	}

	storage, err := newR2Storage(config)
	if err != nil {
		t.Fatalf("Failed to create R2Storage: %v", err)
	}

	// Try to generate URL with past expiration
	pastTime := time.Now().Add(-1 * time.Hour)
	_, err = storage.GetURL("test.mp3", pastTime)
	if err == nil {
		t.Fatal("Expected error for past expiration time")
	}
}

func TestR2Storage_GetURL_Format(t *testing.T) {
	config := &StorageConfig{
		Backend:           "r2",
		R2Endpoint:        "https://test.r2.cloudflarestorage.com",
		R2AccessKeyID:     "test-key",
		R2SecretAccessKey: "test-secret",
		R2Bucket:          "test-bucket",
		R2PublicURL:       "https://cdn.example.com",
	}

	storage, err := newR2Storage(config)
	if err != nil {
		t.Fatalf("Failed to create R2Storage: %v", err)
	}

	// Generate presigned URL
	expiresAt := time.Now().Add(1 * time.Hour)
	url, err := storage.GetURL("tts_audio/test.mp3", expiresAt)
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}

	// Verify URL is not empty and looks like a presigned URL
	if url == "" {
		t.Error("GetURL returned empty string")
	}

	// Presigned URLs should contain signature parameters
	if !contains(url, "X-Amz") {
		t.Error("GetURL should return presigned URL with X-Amz parameters")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
