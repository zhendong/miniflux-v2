// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"
	"time"
)

func TestProviderResult_HasAudioData(t *testing.T) {
	result := &ProviderResult{
		AudioData: []byte("test audio"),
	}

	if len(result.AudioData) == 0 {
		t.Error("Expected AudioData to be populated")
	}
}

func TestProviderResult_HasAudioURL(t *testing.T) {
	expiresAt := time.Now().Add(1 * time.Hour)
	result := &ProviderResult{
		AudioURL:  "https://example.com/audio.mp3",
		ExpiresAt: expiresAt,
	}

	if result.AudioURL == "" {
		t.Error("Expected AudioURL to be populated")
	}

	if result.ExpiresAt.IsZero() {
		t.Error("Expected ExpiresAt to be set")
	}
}
