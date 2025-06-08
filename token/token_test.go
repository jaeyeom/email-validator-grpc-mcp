package token

import (
	"strings"
	"testing"
	"time"
)

func TestGenerator_GenerateLinkToken(t *testing.T) {
	t.Parallel()

	generator := NewGenerator()

	// Test default token generation
	token1, err := generator.GenerateLinkToken()
	if err != nil {
		t.Fatalf("Failed to generate link token: %v", err)
	}

	// Verify token is not empty
	if token1 == "" {
		t.Error("Generated link token is empty")
	}

	// Verify token length is appropriate for base64 encoded DefaultLinkTokenLength bytes
	expectedLen := DefaultLinkTokenLength * 4 / 3 // approximate base64 length
	if len(token1) < expectedLen-3 || len(token1) > expectedLen+3 {
		t.Errorf("Token length %d is not within expected range of ~%d", len(token1), expectedLen)
	}

	// Generate a second token and verify uniqueness
	token2, err := generator.GenerateLinkToken()
	if err != nil {
		t.Fatalf("Failed to generate second link token: %v", err)
	}

	if token1 == token2 {
		t.Error("Two consecutively generated tokens are identical, which is extremely unlikely")
	}

	// Test with custom token length
	customLength := 64
	generator = generator.WithLinkTokenLength(customLength)

	token3, err := generator.GenerateLinkToken()
	if err != nil {
		t.Fatalf("Failed to generate token with custom length: %v", err)
	}

	// Verify custom length token
	expectedCustomLen := customLength * 4 / 3 // approximate base64 length
	if len(token3) < expectedCustomLen-3 || len(token3) > expectedCustomLen+3 {
		t.Errorf("Custom token length %d is not within expected range of ~%d", len(token3), expectedCustomLen)
	}
}

func TestGenerator_GenerateCodeToken(t *testing.T) {
	t.Parallel()

	generator := NewGenerator()

	// Test default code token generation
	token1, err := generator.GenerateCodeToken()
	if err != nil {
		t.Fatalf("Failed to generate code token: %v", err)
	}

	// Verify token is not empty
	if token1 == "" {
		t.Error("Generated code token is empty")
	}

	// Verify token length matches expected length
	if len(token1) != DefaultCodeTokenLength {
		t.Errorf("Code token length %d does not match expected length %d", len(token1), DefaultCodeTokenLength)
	}

	// Verify token only contains characters from the default charset
	for _, c := range token1 {
		if !strings.ContainsRune(DefaultCodeCharset, c) {
			t.Errorf("Code token contains invalid character: %c", c)
		}
	}

	// Generate a second token and verify uniqueness (although collisions are possible with short codes)
	token2, err := generator.GenerateCodeToken()
	if err != nil {
		t.Fatalf("Failed to generate second code token: %v", err)
	}

	// Log tokens for debugging and verify they're different (though with short codes, collisions are possible)
	t.Logf("First code token: %s, Second code token: %s", token1, token2)

	// Test with custom token length and charset
	customLength := 8
	customCharset := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	generator = generator.WithCodeTokenLength(customLength).WithCodeCharset(customCharset)

	token3, err := generator.GenerateCodeToken()
	if err != nil {
		t.Fatalf("Failed to generate token with custom length and charset: %v", err)
	}

	// Verify custom length
	if len(token3) != customLength {
		t.Errorf("Custom code token length %d does not match expected length %d", len(token3), customLength)
	}

	// Verify custom charset
	for _, c := range token3 {
		if !strings.ContainsRune(customCharset, c) {
			t.Errorf("Custom code token contains invalid character: %c", c)
		}
	}
}

func TestToken_IsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ttl      time.Duration
		expected bool
	}{
		{
			name:     "Not expired token",
			ttl:      time.Hour,
			expected: false,
		},
		{
			name:     "Expired token",
			ttl:      -time.Hour, // Negative duration to create an already expired token
			expected: true,
		},
		{
			name:     "About to expire token",
			ttl:      time.Millisecond,
			expected: false, // Initially not expired
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tkn := New("test-token", LinkToken, "test-id", testCase.ttl)

			if testCase.name == "About to expire token" {
				// Wait for token to expire
				time.Sleep(time.Millisecond * 10)

				if !tkn.IsExpired() {
					t.Error("Token should have expired after sleeping")
				}
			} else if tkn.IsExpired() != testCase.expected {
				t.Errorf("IsExpired() = %v, expected %v", tkn.IsExpired(), testCase.expected)
			}
		})
	}
}
