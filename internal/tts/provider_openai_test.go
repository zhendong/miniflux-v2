// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"encoding/json"
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
	body := provider.buildRequestBody("Hello world", "en")

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
	body := provider.buildRequestBody("Test", "en")

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Instructions should only be included for gpt-4o-mini-tts model
	if parsed["model"] == "tts-1" {
		if _, exists := parsed["instructions"]; exists {
			t.Error("instructions field should not be present for non-gpt-4o-mini-tts models")
		}
	}
}
