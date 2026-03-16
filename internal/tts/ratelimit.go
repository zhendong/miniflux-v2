// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"sync"
	"time"
)

// RateLimiter implements in-memory per-user rate limiting.
type RateLimiter struct {
	mu         sync.RWMutex
	limits     map[int64]*userLimit
	maxPerHour int
}

type userLimit struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a new rate limiter with specified max requests per hour.
func NewRateLimiter(maxPerHour int) *RateLimiter {
	rl := &RateLimiter{
		limits:     make(map[int64]*userLimit),
		maxPerHour: maxPerHour,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed for the given user.
func (rl *RateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	limit, exists := rl.limits[userID]

	// New window
	if !exists || now.Sub(limit.windowStart) > time.Hour {
		rl.limits[userID] = &userLimit{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Check limit
	if limit.count >= rl.maxPerHour {
		return false
	}

	// Increment and allow
	limit.count++
	return true
}

// Remaining returns the number of requests remaining for the user.
func (rl *RateLimiter) Remaining(userID int64) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limit, exists := rl.limits[userID]

	// No requests yet or window expired
	if !exists || time.Since(limit.windowStart) > time.Hour {
		return rl.maxPerHour
	}

	remaining := rl.maxPerHour - limit.count
	if remaining < 0 {
		return 0
	}

	return remaining
}

// cleanup removes stale entries periodically.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for userID, limit := range rl.limits {
			// Remove entries older than 2 hours
			if now.Sub(limit.windowStart) > 2*time.Hour {
				delete(rl.limits, userID)
			}
		}

		rl.mu.Unlock()
	}
}

// Global rate limiter instance
var globalRateLimiter *RateLimiter
var rateLimiterOnce sync.Once

// AllowRequest checks if a user can make a TTS request (singleton pattern).
func AllowRequest(userID int64, limitPerHour int) bool {
	rateLimiterOnce.Do(func() {
		globalRateLimiter = NewRateLimiter(limitPerHour)
	})
	return globalRateLimiter.Allow(userID)
}
