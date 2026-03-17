// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	body, err := provider.buildRequestBody("Hello world", "en")
	if err != nil {
		t.Fatalf("Failed to build request body: %v", err)
	}

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
	body, err := provider.buildRequestBody("Test", "en")
	if err != nil {
		t.Fatalf("Failed to build request body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Instructions should not be present for non-gpt-4o-mini-tts models
	if _, exists := parsed["instructions"]; exists {
		t.Error("instructions field should not be present for non-gpt-4o-mini-tts models")
	}
}

func TestOpenAI_BuildRequestBody_GPT4oMiniNoInstructions(t *testing.T) {
	config := &ProviderConfig{
		Model:        "gpt-4o-mini-tts",
		Voice:        "alloy",
		Speed:        1.0,
		Format:       "mp3",
		Instructions: "", // Empty
	}

	provider := newOpenAIProvider(config)
	body, err := provider.buildRequestBody("Test", "en")
	if err != nil {
		t.Fatalf("Failed to build request body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Instructions should not be present even for gpt-4o-mini-tts if empty
	if _, exists := parsed["instructions"]; exists {
		t.Error("instructions field should not be present when empty")
	}
}

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
