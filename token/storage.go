package token

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Common errors for token storage operations.
var (
	ErrTokenNotFound       = errors.New("token not found")
	ErrInvalidToken        = errors.New("invalid token")
	ErrInvalidTokenType    = errors.New("invalid token type in storage")
	ErrInvalidTokenKeyType = errors.New("invalid token key type in storage")
	ErrTokenNil            = errors.New("token cannot be nil")
	ErrEmptyTokenValue     = errors.New("token value cannot be empty")
	ErrEmptyValidationID   = errors.New("validation ID cannot be empty")
)

// MemoryStorage provides an in-memory implementation for token storage.
type MemoryStorage struct {
	tokens       sync.Map // map[tokenKey]*Token
	validationID sync.Map // map[string][]tokenKey
	mu           sync.RWMutex
}

// tokenKey is a composite key for token lookup.
type tokenKey struct {
	value string
	typ   Type
}

// NewMemoryStorage creates a new in-memory token storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

// Store saves a token to the in-memory storage.
// Returns an error if the token is invalid.
func (s *MemoryStorage) Store(ctx context.Context, token *Token) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}
	if err := validateToken(token); err != nil {
		return err
	}

	key := tokenKey{value: token.Value, typ: token.Type}

	s.tokens.Store(key, token)

	// Update the validation ID index
	s.mu.Lock()
	defer s.mu.Unlock()

	var keys []tokenKey
	if val, ok := s.validationID.Load(token.ValidationID); ok {
		keys, _ = val.([]tokenKey)
	}

	keys = append(keys, key)
	s.validationID.Store(token.ValidationID, keys)

	return nil
}

// Retrieve gets a token from the in-memory storage.
// Returns ErrTokenNotFound if the token does not exist.
// Returns TokenExpiredError if the token exists but has expired.
func (s *MemoryStorage) Retrieve(ctx context.Context, tokenValue string, tokenType Type) (*Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}
	key := tokenKey{value: tokenValue, typ: tokenType}

	val, ok := s.tokens.Load(key)
	if !ok {
		return nil, ErrTokenNotFound
	}

	token, ok := val.(*Token)
	if !ok {
		return nil, ErrInvalidTokenType
	}

	// Check if the token has expired
	if token.IsExpired() {
		// Delete expired token
		s.tokens.Delete(key)

		return nil, &TokenExpiredError{
			TokenValue: token.Value,
			TokenType:  token.Type,
			ExpiredAt:  token.ValidUntil,
		}
	}

	return token, nil
}

// Delete removes a token from the in-memory storage.
// This operation is idempotent and will not return an error if the token does not exist.
func (s *MemoryStorage) Delete(ctx context.Context, tokenValue string, tokenType Type) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}
	key := tokenKey{value: tokenValue, typ: tokenType}

	// Get the token to find its validation ID
	val, ok := s.tokens.Load(key)
	if !ok {
		return nil
	}
	token, ok := val.(*Token)
	if !ok {
		return ErrInvalidTokenType
	}

	// Remove from tokens map
	s.tokens.Delete(key)

	// Update validation ID index
	s.mu.Lock()
	defer s.mu.Unlock()

	if val, ok := s.validationID.Load(token.ValidationID); ok {
		keys, ok := val.([]tokenKey)
		if !ok {
			return ErrInvalidTokenKeyType
		}
		newKeys := make([]tokenKey, 0, len(keys)-1)

		for _, k := range keys {
			if k.value != tokenValue || k.typ != tokenType {
				newKeys = append(newKeys, k)
			}
		}

		if len(newKeys) > 0 {
			s.validationID.Store(token.ValidationID, newKeys)
		} else {
			s.validationID.Delete(token.ValidationID)
		}
	}

	return nil
}

// DeleteByValidationID removes all tokens associated with a validation ID.
// This operation is idempotent and will not return an error if no tokens exist for the validation ID.
func (s *MemoryStorage) DeleteByValidationID(ctx context.Context, validationID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.validationID.Load(validationID)
	if !ok {
		return nil
	}

	keys, ok := val.([]tokenKey)
	if !ok {
		return ErrInvalidTokenKeyType
	}
	for _, key := range keys {
		s.tokens.Delete(key)
	}

	s.validationID.Delete(validationID)
	return nil
}

// validateToken checks if a token is valid for storage.
func validateToken(token *Token) error {
	if token == nil {
		return ErrTokenNil
	}

	if token.Value == "" {
		return ErrEmptyTokenValue
	}

	if token.ValidationID == "" {
		return ErrEmptyValidationID
	}

	// Note: We don't check for expiration here to allow storing expired tokens for testing
	// The expiration check is performed during retrieval instead

	return nil
}
