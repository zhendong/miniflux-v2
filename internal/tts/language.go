// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"miniflux.app/v2/internal/model"
)

// DetectLanguage returns the language for TTS generation.
// Currently uses the configured default language.
func DetectLanguage(entry *model.Entry, defaultLanguage string) string {
	// Use configured default language
	if defaultLanguage != "" {
		return defaultLanguage
	}

	// Ultimate fallback to English
	return "en"
}
