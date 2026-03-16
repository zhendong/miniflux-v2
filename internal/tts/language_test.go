// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"

	"miniflux.app/v2/internal/model"
)

func TestDetectLanguage_ReturnsDefault(t *testing.T) {
	entry := &model.Entry{
		Feed: &model.Feed{},
	}

	result := DetectLanguage(entry, "fr")
	expected := "fr"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDetectLanguage_FallbackToEnglish(t *testing.T) {
	entry := &model.Entry{
		Feed: &model.Feed{},
	}

	result := DetectLanguage(entry, "")
	expected := "en"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
