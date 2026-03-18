// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
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
