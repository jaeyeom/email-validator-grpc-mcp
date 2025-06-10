package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jaeyeom/email-validator-grpc-mcp/token"
	"github.com/redis/go-redis/v9"
)

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	// Start a mini Redis server for testing
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	// Create a Redis client connected to the mini Redis server
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

func TestStorage_Store(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   *token.Token
		wantErr bool
	}{
		{
			name: "store link token",
			token: &token.Token{
				Value:        "test-link-token",
				Type:         token.TypeLink,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "validation-123",
			},
			wantErr: false,
		},
		{
			name: "store code token",
			token: &token.Token{
				Value:        "1234",
				Type:         token.TypeCode,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "validation-456",
			},
			wantErr: false,
		},
		{
			name: "store token with empty value",
			token: &token.Token{
				Value:        "",
				Type:         token.TypeLink,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "validation-789",
			},
			wantErr: true,
		},
		{
			name: "store token with empty validation ID",
			token: &token.Token{
				Value:        "test-token",
				Type:         token.TypeLink,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mr, client := setupMiniRedis(t)
			defer mr.Close()

			storage := New(client, WithRedisClient(client))
			err := storage.Store(context.Background(), tt.token)

			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.Store() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStorage_Retrieve(t *testing.T) {
	t.Parallel()

	// Create a token with known values
	validToken := &token.Token{
		Value:        "test-token",
		Type:         token.TypeLink,
		CreatedAt:    time.Now(),
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-123",
	}

	expiredToken := &token.Token{
		Value:        "expired-token",
		Type:         token.TypeLink,
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		ValidUntil:   time.Now().Add(-time.Hour), // Expired 1 hour ago
		ValidationID: "validation-456",
	}

	tests := []struct {
		name           string
		setupFunc      func(*Storage, context.Context)
		tokenValue     string
		tokenType      token.Type
		want           *token.Token
		wantErr        bool
		wantErrExpired bool
	}{
		{
			name: "retrieve existing token",
			setupFunc: func(s *Storage, ctx context.Context) {
				_ = s.Store(ctx, validToken)
			},
			tokenValue: validToken.Value,
			tokenType:  validToken.Type,
			want:       validToken,
			wantErr:    false,
		},
		{
			name:       "retrieve non-existent token",
			setupFunc:  func(s *Storage, ctx context.Context) {},
			tokenValue: "non-existent-token",
			tokenType:  token.TypeLink,
			want:       nil,
			wantErr:    true,
		},
		{
			name: "retrieve expired token",
			setupFunc: func(s *Storage, ctx context.Context) {
				_ = s.Store(ctx, expiredToken)
				// Force expiration in miniredis
				// Note: This will be handled by the test setup
			},
			tokenValue:     expiredToken.Value,
			tokenType:      expiredToken.Type,
			want:           nil,
			wantErr:        true,
			wantErrExpired: true,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mr, client := setupMiniRedis(t)
			defer mr.Close()

			ctx := context.Background()
			storage := New(client, WithRedisClient(client))
			tt.setupFunc(storage, ctx)

			// For expired token test, manually set expiration in miniredis
			if tt.wantErrExpired {
				mr.FastForward(2 * time.Hour) // Fast forward time to make token expire
			}

			got, err := storage.Retrieve(ctx, tt.tokenValue, tt.tokenType)

			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.Retrieve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErrExpired && err != nil {
				// In Redis, expired tokens are automatically removed by the TTL mechanism
				// So we might get ErrTokenNotFound instead of TokenExpiredError
				if !token.IsTokenExpiredError(err) && err != token.ErrTokenNotFound {
					t.Errorf("Storage.Retrieve() expected TokenExpiredError or ErrTokenNotFound, got %T: %v", err, err)
				}
				return
			}

			if tt.want != nil && got != nil {
				// Compare relevant fields
				if got.Value != tt.want.Value ||
					got.Type != tt.want.Type ||
					got.ValidationID != tt.want.ValidationID {
					t.Errorf("Storage.Retrieve() = %v, want %v", got, tt.want)
				}
			} else if (got == nil) != (tt.want == nil) {
				t.Errorf("Storage.Retrieve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorage_Delete(t *testing.T) {
	t.Parallel()

	// Create tokens with known values
	token1 := &token.Token{
		Value:        "test-token-1",
		Type:         token.TypeLink,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-123",
	}

	tests := []struct {
		name       string
		setupFunc  func(*Storage, context.Context)
		tokenValue string
		tokenType  token.Type
		wantErr    bool
	}{
		{
			name: "delete existing token",
			setupFunc: func(s *Storage, ctx context.Context) {
				_ = s.Store(ctx, token1)
			},
			tokenValue: token1.Value,
			tokenType:  token1.Type,
			wantErr:    false,
		},
		{
			name:       "delete non-existent token",
			setupFunc:  func(s *Storage, ctx context.Context) {},
			tokenValue: "non-existent-token",
			tokenType:  token.TypeLink,
			wantErr:    false, // Delete is idempotent
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mr, client := setupMiniRedis(t)
			defer mr.Close()

			ctx := context.Background()
			storage := New(client, WithRedisClient(client))
			tt.setupFunc(storage, ctx)

			err := storage.Delete(ctx, tt.tokenValue, tt.tokenType)
			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify token is deleted
			_, err = storage.Retrieve(ctx, tt.tokenValue, tt.tokenType)
			if err == nil {
				t.Errorf("Storage.Delete() token still exists after deletion")
			}
		})
	}
}

func TestStorage_DeleteByValidationID(t *testing.T) {
	t.Parallel()

	// Create tokens with known values
	token1 := &token.Token{
		Value:        "test-token-1",
		Type:         token.TypeLink,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-123",
	}

	token2 := &token.Token{
		Value:        "test-token-2",
		Type:         token.TypeCode,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-123", // Same validation ID as token1
	}

	token3 := &token.Token{
		Value:        "test-token-3",
		Type:         token.TypeLink,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-456", // Different validation ID
	}

	tests := []struct {
		name         string
		setupFunc    func(*Storage, context.Context)
		validationID string
		wantErr      bool
	}{
		{
			name: "delete tokens by existing validation ID",
			setupFunc: func(s *Storage, ctx context.Context) {
				_ = s.Store(ctx, token1)
				_ = s.Store(ctx, token2)
				_ = s.Store(ctx, token3)
			},
			validationID: "validation-123",
			wantErr:      false,
		},
		{
			name: "delete tokens by non-existent validation ID",
			setupFunc: func(s *Storage, ctx context.Context) {
				_ = s.Store(ctx, token3)
			},
			validationID: "non-existent-validation-id",
			wantErr:      false, // DeleteByValidationID is idempotent
		},
		{
			name:         "delete tokens by empty validation ID",
			setupFunc:    func(s *Storage, ctx context.Context) {},
			validationID: "",
			wantErr:      true, // Empty validation ID should return an error
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mr, client := setupMiniRedis(t)
			defer mr.Close()

			ctx := context.Background()
			storage := New(client, WithRedisClient(client))
			tt.setupFunc(storage, ctx)

			err := storage.DeleteByValidationID(ctx, tt.validationID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.DeleteByValidationID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validationID != "" {
				// Verify tokens with this validation ID are deleted
				if tt.validationID == token1.ValidationID {
					_, err1 := storage.Retrieve(ctx, token1.Value, token1.Type)
					_, err2 := storage.Retrieve(ctx, token2.Value, token2.Type)
					if err1 == nil || err2 == nil {
						t.Errorf("Storage.DeleteByValidationID() tokens still exist after deletion")
					}

					// Verify token with different validation ID still exists
					_, err3 := storage.Retrieve(context.Background(), token3.Value, token3.Type)
					if err3 != nil && !token.IsTokenExpiredError(err3) {
						t.Errorf("Storage.DeleteByValidationID() deleted token with different validation ID")
					}
				}
			}
		})
	}
}
