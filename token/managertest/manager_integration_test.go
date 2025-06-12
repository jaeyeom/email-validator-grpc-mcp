package managertest

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jaeyeom/email-validator-grpc-mcp/token"
	"github.com/jaeyeom/email-validator-grpc-mcp/token/storage/memory"
)

func TestManager_CreateLinkToken(t *testing.T) {
	ctx := context.Background()
	storage := memory.New()
	manager := token.NewManager(storage)

	tests := []struct {
		name         string
		validationID string
		wantErr      bool
	}{
		{
			name:         "valid validation ID",
			validationID: "test-validation-123",
			wantErr:      false,
		},
		{
			name:         "empty validation ID",
			validationID: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := manager.CreateLinkToken(ctx, tt.validationID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateLinkToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tok == nil {
					t.Error("CreateLinkToken() returned nil token")
					return
				}
				if tok.Type != token.TypeLink {
					t.Errorf("CreateLinkToken() token type = %v, want %v", tok.Type, token.TypeLink)
				}
				if tok.ValidationID != tt.validationID {
					t.Errorf("CreateLinkToken() validation ID = %v, want %v", tok.ValidationID, tt.validationID)
				}
				if tok.Value == "" {
					t.Error("CreateLinkToken() token value is empty")
				}
				if tok.IsExpired() {
					t.Error("CreateLinkToken() token is already expired")
				}
			}
		})
	}
}

func TestManager_CreateCodeToken(t *testing.T) {
	ctx := context.Background()
	storage := memory.New()
	manager := token.NewManager(storage)

	tests := []struct {
		name         string
		validationID string
		wantErr      bool
	}{
		{
			name:         "valid validation ID",
			validationID: "test-validation-456",
			wantErr:      false,
		},
		{
			name:         "empty validation ID",
			validationID: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := manager.CreateCodeToken(ctx, tt.validationID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCodeToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tok == nil {
					t.Error("CreateCodeToken() returned nil token")
					return
				}
				if tok.Type != token.TypeCode {
					t.Errorf("CreateCodeToken() token type = %v, want %v", tok.Type, token.TypeCode)
				}
				if tok.ValidationID != tt.validationID {
					t.Errorf("CreateCodeToken() validation ID = %v, want %v", tok.ValidationID, tt.validationID)
				}
				if tok.Value == "" {
					t.Error("CreateCodeToken() token value is empty")
				}
				if tok.IsExpired() {
					t.Error("CreateCodeToken() token is already expired")
				}
			}
		})
	}
}

func TestManager_VerifyToken(t *testing.T) {
	ctx := context.Background()
	storage := memory.New()
	manager := token.NewManager(storage)

	// Create a valid token first
	validToken, err := manager.CreateLinkToken(ctx, "test-validation-verify")
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	tests := []struct {
		name       string
		tokenValue string
		tokenType  token.Type
		wantErr    bool
		wantToken  bool
	}{
		{
			name:       "valid token",
			tokenValue: validToken.Value,
			tokenType:  token.TypeLink,
			wantErr:    false,
			wantToken:  true,
		},
		{
			name:       "non-existent token",
			tokenValue: "non-existent-token",
			tokenType:  token.TypeLink,
			wantErr:    true,
			wantToken:  false,
		},
		{
			name:       "empty token value",
			tokenValue: "",
			tokenType:  token.TypeLink,
			wantErr:    true,
			wantToken:  false,
		},
		{
			name:       "wrong token type",
			tokenValue: validToken.Value,
			tokenType:  token.TypeCode,
			wantErr:    true,
			wantToken:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := manager.VerifyToken(ctx, tt.tokenValue, tt.tokenType)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantToken && tok == nil {
				t.Error("VerifyToken() expected token but got nil")
			}
			if !tt.wantToken && tok != nil {
				t.Error("VerifyToken() expected nil token but got one")
			}
		})
	}
}

func TestManager_VerifyToken_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	storage := memory.New()
	manager := token.NewManager(storage)

	// Create a token with very short TTL
	expiredToken, err := manager.CreateTokenWithTTL(ctx, token.TypeLink, "test-validation-expired", time.Nanosecond)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = manager.VerifyToken(ctx, expiredToken.Value, token.TypeLink)
	if err == nil {
		t.Error("VerifyToken() expected error for expired token")
	}

	if !token.IsTokenExpiredError(err) {
		t.Errorf("VerifyToken() expected TokenExpiredError, got %T", err)
	}
}

func TestManager_InvalidateValidation(t *testing.T) {
	ctx := context.Background()
	storage := memory.New()
	manager := token.NewManager(storage)

	validationID := "test-validation-multiple"

	// Create multiple tokens for the same validation
	linkToken, err := manager.CreateLinkToken(ctx, validationID)
	if err != nil {
		t.Fatalf("Failed to create link token: %v", err)
	}

	codeToken, err := manager.CreateCodeToken(ctx, validationID)
	if err != nil {
		t.Fatalf("Failed to create code token: %v", err)
	}

	// Invalidate the validation
	err = manager.InvalidateValidation(ctx, validationID)
	if err != nil {
		t.Errorf("InvalidateValidation() error = %v", err)
	}

	// Verify both tokens are no longer retrievable
	_, err = manager.VerifyToken(ctx, linkToken.Value, token.TypeLink)
	if err == nil {
		t.Error("VerifyToken() should fail for link token after validation invalidation")
	}

	_, err = manager.VerifyToken(ctx, codeToken.Value, token.TypeCode)
	if err == nil {
		t.Error("VerifyToken() should fail for code token after validation invalidation")
	}
}

func TestManager_WithOptions(t *testing.T) {
	storage := memory.New()
	logger := slog.Default()
	generator := token.NewGenerator().WithLinkTokenLength(64)

	manager := token.NewManager(storage,
		token.WithManagerLogger(logger),
		token.WithLinkTokenTTL(2*time.Hour),
		token.WithCodeTokenTTL(30*time.Minute),
		token.WithGenerator(generator),
	)

	// Test creating tokens with custom options
	ctx := context.Background()

	// Test link token with custom TTL
	linkToken, err := manager.CreateLinkToken(ctx, "test-custom-options")
	if err != nil {
		t.Fatalf("CreateLinkToken() failed: %v", err)
	}

	// Verify the token was created with expected expiration
	expectedExpiry := time.Now().Add(2 * time.Hour)
	if linkToken.ValidUntil.Before(expectedExpiry.Add(-time.Minute)) ||
		linkToken.ValidUntil.After(expectedExpiry.Add(time.Minute)) {
		t.Errorf("Link token expiry not within expected range, got %v", linkToken.ValidUntil)
	}

	// Test code token with custom TTL
	codeToken, err := manager.CreateCodeToken(ctx, "test-custom-options-code")
	if err != nil {
		t.Fatalf("CreateCodeToken() failed: %v", err)
	}

	expectedCodeExpiry := time.Now().Add(30 * time.Minute)
	if codeToken.ValidUntil.Before(expectedCodeExpiry.Add(-time.Minute)) ||
		codeToken.ValidUntil.After(expectedCodeExpiry.Add(time.Minute)) {
		t.Errorf("Code token expiry not within expected range, got %v", codeToken.ValidUntil)
	}
}

// BenchmarkManager_CreateAndVerifyToken benchmarks the complete token lifecycle.
func BenchmarkManager_CreateAndVerifyToken(b *testing.B) {
	ctx := context.Background()
	storage := memory.New()
	manager := token.NewManager(storage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create token
		tok, err := manager.CreateLinkToken(ctx, "benchmark-validation")
		if err != nil {
			b.Fatalf("CreateLinkToken failed: %v", err)
		}

		// Verify token
		_, err = manager.VerifyToken(ctx, tok.Value, token.TypeLink)
		if err != nil {
			b.Fatalf("VerifyToken failed: %v", err)
		}

		// Clean up
		err = manager.InvalidateToken(ctx, tok.Value, token.TypeLink)
		if err != nil {
			b.Fatalf("InvalidateToken failed: %v", err)
		}
	}
}
