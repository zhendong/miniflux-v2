// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bufio"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAliyun_ParseSSEStream_SizeLimit(t *testing.T) {
	// Create a large chunk that exceeds 50MB when accumulated
	largeData := make([]byte, 51<<20) // 51MB
	for i := range largeData {
		largeData[i] = 'A'
	}
	chunk := base64.StdEncoding.EncodeToString(largeData)

	sseData := "data: {\"output\":{\"audio\":{\"data\":\"" + chunk + "\"}}}\n\n"

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	_, err := provider.parseSSEStream(reader)
	if err == nil {
		t.Fatal("Expected error for size limit exceeded")
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Errorf("Expected size limit error, got: %v", err)
	}
}

func TestAliyun_ParseSSEStream_EmptyStream(t *testing.T) {
	// Empty SSE stream
	sseData := ""

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	_, err := provider.parseSSEStream(reader)
	if err == nil {
		t.Fatal("Expected error for empty stream")
	}
	if !strings.Contains(err.Error(), "no audio data") {
		t.Errorf("Expected 'no audio data' error, got: %v", err)
	}
}

func TestAliyun_ParseSSEStream_NoDataEvents(t *testing.T) {
	// SSE stream with non-data lines only
	sseData := "event: start\n\n:comment\n\nid: 123\n\n"

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	_, err := provider.parseSSEStream(reader)
	if err == nil {
		t.Fatal("Expected error for stream with no data events")
	}
}

func TestAliyun_ParseSSEStream_MixedContent(t *testing.T) {
	// SSE stream with mixed content (comments, events, data)
	chunk1 := base64.StdEncoding.EncodeToString([]byte("audio 1"))
	chunk2 := base64.StdEncoding.EncodeToString([]byte("audio 2"))

	sseData := ":comment line\n\n" +
		"event: start\n\n" +
		"data: {\"output\":{\"audio\":{\"data\":\"" + chunk1 + "\"}}}\n\n" +
		":another comment\n\n" +
		"id: 123\n\n" +
		"data: {\"output\":{\"audio\":{\"data\":\"" + chunk2 + "\"}}}\n\n"

	reader := bufio.NewReader(strings.NewReader(sseData))
	provider := newAliyunProvider(&ProviderConfig{})

	audioData, err := provider.parseSSEStream(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "audio 1audio 2"
	if string(audioData) != expected {
		t.Errorf("Expected %q, got %q", expected, string(audioData))
	}
}

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

func TestAliyun_HandleHTTPError(t *testing.T) {
	tests := []struct {
		statusCode int
		wantMsg    string
	}{
		{400, "invalid Aliyun request parameters"},
		{401, "Aliyun authentication failed"},
		{403, "Aliyun authentication failed"},
		{429, "Aliyun rate limit exceeded"},
		{500, "Aliyun service unavailable"},
		{502, "Aliyun service unavailable"},
		{503, "Aliyun service unavailable"},
		{404, "Aliyun request failed: HTTP 404"},
	}

	provider := newAliyunProvider(&ProviderConfig{})

	for _, tt := range tests {
		err := provider.handleHTTPError(tt.statusCode)
		if err == nil {
			t.Errorf("handleHTTPError(%d) expected error, got nil", tt.statusCode)
		}
		if !strings.Contains(err.Error(), tt.wantMsg) {
			t.Errorf("handleHTTPError(%d) = %q, want substring %q", tt.statusCode, err.Error(), tt.wantMsg)
		}
	}
}

func TestAliyun_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
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
	_, err := provider.Generate("Test", "en")

	if err == nil {
		t.Fatal("Expected error for HTTP 401")
	}

	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected authentication error, got: %v", err)
	}
}

func TestAliyun_Generate_ContentTooLarge(t *testing.T) {
	config := &ProviderConfig{
		Endpoint:   "http://example.com",
		APIKey:     "test-api-key",
		Model:      "qwen3-tts-flash",
		Voice:      "Cherry",
		HTTPClient: &http.Client{},
	}

	// Create text larger than maxContentLength (50000)
	largeText := strings.Repeat("a", 50001)

	provider := newAliyunProvider(config)
	_, err := provider.Generate(largeText, "en")

	if err == nil {
		t.Fatal("Expected error for content too large")
	}

	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

func TestAliyun_Generate_NonStreaming_EmptyURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return response with empty URL
		w.Write([]byte(`{"output":{"audio":{"url":""}}}`))
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
	_, err := provider.Generate("Test", "en")

	if err == nil {
		t.Fatal("Expected error for empty URL response")
	}

	if !strings.Contains(err.Error(), "empty audio URL") {
		t.Errorf("Expected 'empty audio URL' error, got: %v", err)
	}
}
