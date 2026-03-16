// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow_FirstRequest(t *testing.T) {
	rl := NewRateLimiter(10)
	userID := int64(123)

	if !rl.Allow(userID) {
		t.Error("First request should be allowed")
	}
}

func TestRateLimiter_Allow_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(3)
	userID := int64(123)

	// Make 3 requests (limit)
	for i := 0; i < 3; i++ {
		if !rl.Allow(userID) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_Deny_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(2)
	userID := int64(123)

	// Make 2 requests (limit)
	rl.Allow(userID)
	rl.Allow(userID)

	// 3rd request should be denied
	if rl.Allow(userID) {
		t.Error("Request exceeding limit should be denied")
	}
}

func TestRateLimiter_Remaining(t *testing.T) {
	rl := NewRateLimiter(5)
	userID := int64(123)

	// No requests yet
	if remaining := rl.Remaining(userID); remaining != 5 {
		t.Errorf("Expected 5 remaining, got %d", remaining)
	}

	// Make 2 requests
	rl.Allow(userID)
	rl.Allow(userID)

	if remaining := rl.Remaining(userID); remaining != 3 {
		t.Errorf("Expected 3 remaining, got %d", remaining)
	}
}

func TestRateLimiter_DifferentUsers(t *testing.T) {
	rl := NewRateLimiter(1)
	user1 := int64(123)
	user2 := int64(456)

	// User 1 uses their limit
	if !rl.Allow(user1) {
		t.Error("User 1 first request should be allowed")
	}
	if rl.Allow(user1) {
		t.Error("User 1 second request should be denied")
	}

	// User 2 should still have their limit
	if !rl.Allow(user2) {
		t.Error("User 2 first request should be allowed")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	rl := NewRateLimiter(1)
	userID := int64(123)

	// Use the limit
	if !rl.Allow(userID) {
		t.Error("First request should be allowed")
	}

	// Manually expire the window by setting windowStart to past
	rl.mu.Lock()
	if limit, exists := rl.limits[userID]; exists {
		limit.windowStart = time.Now().Add(-2 * time.Hour)
	}
	rl.mu.Unlock()

	// Should allow again after window reset
	if !rl.Allow(userID) {
		t.Error("Request after window reset should be allowed")
	}
}
