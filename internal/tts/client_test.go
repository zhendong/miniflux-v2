// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"miniflux.app/v2/internal/config"
)

func TestClient_Generate_Success(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

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
	configureIntegrationAllowPrivateNetworksOption(t)

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
	configureIntegrationAllowPrivateNetworksOption(t)

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
	configureIntegrationAllowPrivateNetworksOption(t)

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
	configureIntegrationAllowPrivateNetworksOption(t)

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
	configureIntegrationAllowPrivateNetworksOption(t)

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

func configureIntegrationAllowPrivateNetworksOption(t *testing.T) {
	t.Helper()

	t.Setenv("INTEGRATION_ALLOW_PRIVATE_NETWORKS", "1")

	configParser := config.NewConfigParser()
	parsedOptions, err := configParser.ParseEnvironmentVariables()
	if err != nil {
		t.Fatalf("Unable to configure test options: %v", err)
	}

	previousOptions := config.Opts
	config.Opts = parsedOptions
	t.Cleanup(func() {
		config.Opts = previousOptions
	})
}
