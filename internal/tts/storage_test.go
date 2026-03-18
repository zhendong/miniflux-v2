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
