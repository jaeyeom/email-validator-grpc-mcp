package token

import (
	"testing"
	"time"
)

func TestMemoryStorage_Store(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   *Token
		wantErr bool
	}{
		{
			name: "store link token",
			token: &Token{
				Value:        "test-link-token",
				Type:         TypeLink,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "validation-123",
			},
			wantErr: false,
		},
		{
			name: "store code token",
			token: &Token{
				Value:        "1234",
				Type:         TypeCode,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "validation-456",
			},
			wantErr: false,
		},
		{
			name: "store token with empty value",
			token: &Token{
				Value:        "",
				Type:         TypeLink,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "validation-789",
			},
			wantErr: true,
		},
		{
			name: "store token with empty validation ID",
			token: &Token{
				Value:        "test-token",
				Type:         TypeLink,
				ValidUntil:   time.Now().Add(time.Hour),
				ValidationID: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		// No need to copy tt in Go 1.22+
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage := NewMemoryStorage()
			err := storage.Store(t.Context(), tt.token)

			if (err != nil) != tt.wantErr {
				t.Errorf("MemoryStorage.Store() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryStorage_Retrieve(t *testing.T) {
	t.Parallel()

	// Create a token with known values
	validToken := &Token{
		Value:        "test-token",
		Type:         TypeLink,
		CreatedAt:    time.Now(),
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-123",
	}

	expiredToken := &Token{
		Value:        "expired-token",
		Type:         TypeLink,
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		ValidUntil:   time.Now().Add(-time.Hour), // Expired 1 hour ago
		ValidationID: "validation-456",
	}

	tests := []struct {
		name           string
		setupFunc      func(*MemoryStorage)
		tokenValue     string
		tokenType      Type
		want           *Token
		wantErr        bool
		wantErrExpired bool
	}{
		{
			name: "retrieve existing token",
			setupFunc: func(s *MemoryStorage) {
				_ = s.Store(t.Context(), validToken)
			},
			tokenValue:     "test-token",
			tokenType:      TypeLink,
			want:           validToken,
			wantErr:        false,
			wantErrExpired: false,
		},
		{
			name: "retrieve non-existent token",
			setupFunc: func(s *MemoryStorage) {
				// No setup needed
			},
			tokenValue:     "non-existent",
			tokenType:      TypeLink,
			want:           nil,
			wantErr:        true,
			wantErrExpired: false,
		},
		{
			name: "retrieve expired token",
			setupFunc: func(s *MemoryStorage) {
				_ = s.Store(t.Context(), expiredToken)
			},
			tokenValue:     "expired-token",
			tokenType:      TypeLink,
			want:           nil,
			wantErr:        true,
			wantErrExpired: true,
		},
	}

	for _, tt := range tests {
		// No need to copy tt in Go 1.22+
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage := NewMemoryStorage()
			if tt.setupFunc != nil {
				tt.setupFunc(storage)
			}

			got, err := storage.Retrieve(t.Context(), tt.tokenValue, tt.tokenType)

			if (err != nil) != tt.wantErr {
				t.Errorf("MemoryStorage.Retrieve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErrExpired && err != nil {
				if !IsTokenExpiredError(err) {
					t.Errorf("Expected TokenExpiredError, got %T: %v", err, err)
				}
			}

			if tt.want != nil {
				if got == nil {
					t.Fatal("Expected token, got nil")
				}
				if got.Value != tt.want.Value {
					t.Errorf("Token value = %v, want %v", got.Value, tt.want.Value)
				}
				if got.Type != tt.want.Type {
					t.Errorf("Token type = %v, want %v", got.Type, tt.want.Type)
				}
				if got.ValidationID != tt.want.ValidationID {
					t.Errorf("Token validationID = %v, want %v", got.ValidationID, tt.want.ValidationID)
				}
			}
		})
	}
}

func TestMemoryStorage_Delete(t *testing.T) {
	t.Parallel()

	// Create a token with known values
	token := &Token{
		Value:        "test-token",
		Type:         TypeLink,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "validation-123",
	}

	tests := []struct {
		name       string
		setupFunc  func(*MemoryStorage)
		tokenValue string
		tokenType  Type
		wantErr    bool
	}{
		{
			name: "delete existing token",
			setupFunc: func(s *MemoryStorage) {
				_ = s.Store(t.Context(), token)
			},
			tokenValue: "test-token",
			tokenType:  TypeLink,
			wantErr:    false,
		},
		{
			name: "delete non-existent token",
			setupFunc: func(s *MemoryStorage) {
				// No setup needed
			},
			tokenValue: "non-existent",
			tokenType:  TypeLink,
			wantErr:    false, // Deleting non-existent token should not error
		},
	}

	for _, tt := range tests {
		// No need to copy tt in Go 1.22+
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage := NewMemoryStorage()
			if tt.setupFunc != nil {
				tt.setupFunc(storage)
			}

			err := storage.Delete(t.Context(), tt.tokenValue, tt.tokenType)

			if (err != nil) != tt.wantErr {
				t.Errorf("MemoryStorage.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify token is deleted by trying to retrieve it
			if tt.setupFunc != nil {
				_, err := storage.Retrieve(t.Context(), tt.tokenValue, tt.tokenType)
				if err == nil {
					t.Errorf("Token was not deleted")
				}
			}
		})
	}
}

func TestMemoryStorage_DeleteByValidationID(t *testing.T) {
	t.Parallel()

	// Create tokens with the same validation ID
	validationID := "validation-123"
	token1 := &Token{
		Value:        "token1",
		Type:         TypeLink,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: validationID,
	}
	token2 := &Token{
		Value:        "token2",
		Type:         TypeCode,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: validationID,
	}
	token3 := &Token{
		Value:        "token3",
		Type:         TypeLink,
		ValidUntil:   time.Now().Add(time.Hour),
		ValidationID: "different-validation-id",
	}

	tests := []struct {
		name         string
		setupFunc    func(*MemoryStorage)
		validationID string
		wantErr      bool
	}{
		{
			name: "delete tokens by validation ID",
			setupFunc: func(s *MemoryStorage) {
				_ = s.Store(t.Context(), token1)
				_ = s.Store(t.Context(), token2)
				_ = s.Store(t.Context(), token3)
			},
			validationID: validationID,
			wantErr:      false,
		},
		{
			name: "delete with non-existent validation ID",
			setupFunc: func(s *MemoryStorage) {
				_ = s.Store(t.Context(), token3)
			},
			validationID: "non-existent",
			wantErr:      false, // Deleting with non-existent ID should not error
		},
	}

	for _, tt := range tests {
		// No need to copy tt in Go 1.22+
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage := NewMemoryStorage()
			if tt.setupFunc != nil {
				tt.setupFunc(storage)
			}

			err := storage.DeleteByValidationID(t.Context(), tt.validationID)

			if (err != nil) != tt.wantErr {
				t.Errorf("MemoryStorage.DeleteByValidationID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify tokens with the validation ID are deleted
			if tt.setupFunc != nil && tt.validationID == validationID {
				_, err1 := storage.Retrieve(t.Context(), token1.Value, token1.Type)
				_, err2 := storage.Retrieve(t.Context(), token2.Value, token2.Type)
				_, err3 := storage.Retrieve(t.Context(), token3.Value, token3.Type)

				if err1 == nil || err2 == nil {
					t.Errorf("Tokens with validation ID %s were not deleted", validationID)
				}

				if err3 != nil {
					t.Errorf("Token with different validation ID was incorrectly deleted")
				}
			}
		})
	}
}
