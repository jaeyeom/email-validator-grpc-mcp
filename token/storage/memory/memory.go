// Package memory provides an in-memory implementation of token storage.
package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jaeyeom/email-validator-grpc-mcp/token"
)

// Storage provides an in-memory implementation for token storage.
type Storage struct {
	tokens       sync.Map // map[tokenKey]*token.Token
	validationID sync.Map // map[string][]tokenKey
	mu           sync.RWMutex
	logger       *slog.Logger
}

// tokenKey is a composite key for token lookup.
type tokenKey struct {
	value string
	typ   token.Type
}

// Option is a functional option for configuring Storage.
type Option func(*Storage)

// WithLogger sets a custom logger for Storage.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Storage) {
		s.logger = logger
	}
}

// New creates a new in-memory token storage.
func New(opts ...Option) *Storage {
	s := &Storage{
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Store saves a token to the in-memory storage.
// Returns an error if the token is invalid.
func (s *Storage) Store(ctx context.Context, t *token.Token) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if err := token.Validate(t); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	key := tokenKey{value: t.Value, typ: t.Type}

	s.tokens.Store(key, t)

	// Update the validation ID index
	s.mu.Lock()
	defer s.mu.Unlock()

	var keys []tokenKey
	if val, ok := s.validationID.Load(t.ValidationID); ok {
		keys, _ = val.([]tokenKey)
	}

	keys = append(keys, key)
	s.validationID.Store(t.ValidationID, keys)

	s.logger.Debug("token stored in memory",
		"token_type", t.Type,
		"validation_id", t.ValidationID)

	return nil
}

// Retrieve gets a token from the in-memory storage.
// Returns token.ErrTokenNotFound if the token does not exist.
// Returns token.TokenExpiredError if the token exists but has expired.
func (s *Storage) Retrieve(ctx context.Context, tokenValue string, tokenType token.Type) (*token.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	key := tokenKey{value: tokenValue, typ: tokenType}

	val, ok := s.tokens.Load(key)
	if !ok {
		return nil, token.ErrTokenNotFound
	}

	t, ok := val.(*token.Token)
	if !ok {
		return nil, token.ErrInvalidTokenType
	}

	// Check if the token has expired
	if t.IsExpired() {
		// Delete the expired token
		s.tokens.Delete(key)
		s.logger.Debug("expired token retrieved and deleted",
			"token_type", t.Type,
			"validation_id", t.ValidationID)
		return nil, &token.TokenExpiredError{
			TokenValue: tokenValue,
			ExpiredAt:  t.ValidUntil,
		}
	}

	s.logger.Debug("token retrieved from memory",
		"token_type", t.Type,
		"validation_id", t.ValidationID)

	return t, nil
}

// Delete removes a token from the in-memory storage.
// This operation is idempotent and will not return an error if the token does not exist.
func (s *Storage) Delete(ctx context.Context, tokenValue string, tokenType token.Type) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	key := tokenKey{value: tokenValue, typ: tokenType}

	// Get the token to find its validation ID
	val, ok := s.tokens.Load(key)
	if !ok {
		// Token doesn't exist, nothing to delete
		return nil
	}

	t, ok := val.(*token.Token)
	if !ok {
		return token.ErrInvalidTokenType
	}

	// Delete the token
	s.tokens.Delete(key)

	// Update the validation ID index
	s.mu.Lock()
	defer s.mu.Unlock()

	validationID := t.ValidationID
	val, ok = s.validationID.Load(validationID)
	if !ok {
		return nil
	}

	keys, ok := val.([]tokenKey)
	if !ok {
		return token.ErrInvalidTokenKeyType
	}

	// Remove the key from the slice
	var newKeys []tokenKey
	for _, k := range keys {
		if k.value != key.value || k.typ != key.typ {
			newKeys = append(newKeys, k)
		}
	}

	if len(newKeys) > 0 {
		s.validationID.Store(validationID, newKeys)
	} else {
		s.validationID.Delete(validationID)
	}

	s.logger.Debug("token deleted from memory",
		"token_type", t.Type,
		"validation_id", t.ValidationID)

	return nil
}

// DeleteByValidationID removes all tokens associated with a validation ID.
// This operation is idempotent and will not return an error if no tokens exist for the validation ID.
func (s *Storage) DeleteByValidationID(ctx context.Context, validationID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if validationID == "" {
		return token.ErrEmptyValidationID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.validationID.Load(validationID)
	if !ok {
		// No tokens for this validation ID
		return nil
	}

	keys, ok := val.([]tokenKey)
	if !ok {
		return token.ErrInvalidTokenKeyType
	}

	// Delete all tokens for this validation ID
	for _, key := range keys {
		s.tokens.Delete(key)
	}

	// Remove the validation ID entry
	s.validationID.Delete(validationID)

	s.logger.Debug("tokens deleted by validation ID",
		"validation_id", validationID,
		"count", len(keys))

	return nil
}
