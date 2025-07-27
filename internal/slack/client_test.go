package slack

import (
	"errors"
	"testing"
	"time"
)

func TestIsRateLimitError(t *testing.T) {
	client := &Client{}
	
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "message_limit_exceeded error",
			err:      errors.New("message_limit_exceeded"),
			expected: true,
		},
		{
			name:     "rate_limited error",
			err:      errors.New("rate_limited"),
			expected: true,
		},
		{
			name:     "too_many_requests error",
			err:      errors.New("too_many_requests"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "case insensitive",
			err:      errors.New("MESSAGE_LIMIT_EXCEEDED"),
			expected: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.isRateLimitError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for error: %v", tc.expected, result, tc.err)
			}
		})
	}
}

func TestHandleRateLimit(t *testing.T) {
	client := &Client{}
	
	// Test message_limit_exceeded gets longer backoff
	err := errors.New("message_limit_exceeded")
	client.handleRateLimit(err)
	
	if client.rateLimitBackoff != 5*time.Minute {
		t.Errorf("Expected 5 minute backoff for message_limit_exceeded, got %v", client.rateLimitBackoff)
	}
	
	// Test other rate limit errors get shorter backoff
	client.rateLimitBackoff = 0
	err = errors.New("rate_limited")
	client.handleRateLimit(err)
	
	if client.rateLimitBackoff != 1*time.Minute {
		t.Errorf("Expected 1 minute backoff for rate_limited, got %v", client.rateLimitBackoff)
	}
}

func TestIsRateLimited(t *testing.T) {
	client := &Client{}
	
	// Initially not rate limited
	if client.IsRateLimited() {
		t.Error("Expected client to not be rate limited initially")
	}
	
	// Set backoff
	client.rateLimitBackoff = 1 * time.Minute
	if !client.IsRateLimited() {
		t.Error("Expected client to be rate limited after setting backoff")
	}
	
	// Clear backoff
	client.rateLimitBackoff = 0
	if client.IsRateLimited() {
		t.Error("Expected client to not be rate limited after clearing backoff")
	}
}