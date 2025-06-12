// Package token provides functionality for secure token generation, storage,
// retrieval, and expiration handling for the email validation service.
package token

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Manager provides a high-level interface for token operations.
type Manager struct {
	generator *Generator
	storage   Storage
	logger    *slog.Logger

	// Default TTL values
	linkTokenTTL time.Duration
	codeTokenTTL time.Duration
}

// ManagerOption is a functional option for configuring Manager.
type ManagerOption func(*Manager)

// WithStorage sets a custom storage backend for the Manager.
func WithStorage(storage Storage) ManagerOption {
	return func(m *Manager) {
		m.storage = storage
	}
}

// WithManagerLogger sets a custom logger for the Manager.
func WithManagerLogger(logger *slog.Logger) ManagerOption {
	return func(m *Manager) {
		m.logger = logger
	}
}

// WithLinkTokenTTL sets the default TTL for link tokens.
func WithLinkTokenTTL(ttl time.Duration) ManagerOption {
	return func(m *Manager) {
		m.linkTokenTTL = ttl
	}
}

// WithCodeTokenTTL sets the default TTL for code tokens.
func WithCodeTokenTTL(ttl time.Duration) ManagerOption {
	return func(m *Manager) {
		m.codeTokenTTL = ttl
	}
}

// WithGenerator sets a custom token generator for the Manager.
func WithGenerator(generator *Generator) ManagerOption {
	return func(m *Manager) {
		m.generator = generator
	}
}

// NewManager creates a new token manager with the given storage backend.
func NewManager(storage Storage, opts ...ManagerOption) *Manager {
	m := &Manager{
		generator:    NewGenerator(),
		storage:      storage,
		logger:       slog.Default(),
		linkTokenTTL: 24 * time.Hour,   // Default 24 hours for link tokens
		codeTokenTTL: 10 * time.Minute, // Default 10 minutes for code tokens
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// CreateLinkToken generates and stores a new link token for email validation.
func (m *Manager) CreateLinkToken(ctx context.Context, validationID string) (*Token, error) {
	return m.createToken(ctx, TypeLink, validationID, m.linkTokenTTL)
}

// CreateCodeToken generates and stores a new code token for email validation.
func (m *Manager) CreateCodeToken(ctx context.Context, validationID string) (*Token, error) {
	return m.createToken(ctx, TypeCode, validationID, m.codeTokenTTL)
}

// CreateTokenWithTTL generates and stores a new token with a custom TTL.
func (m *Manager) CreateTokenWithTTL(ctx context.Context, tokenType Type, validationID string, ttl time.Duration) (*Token, error) {
	return m.createToken(ctx, tokenType, validationID, ttl)
}

// createToken is the internal method that generates and stores tokens.
func (m *Manager) createToken(ctx context.Context, tokenType Type, validationID string, ttl time.Duration) (*Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	if validationID == "" {
		return nil, ErrEmptyValidationID
	}

	if ttl <= 0 {
		return nil, fmt.Errorf("invalid TTL: must be positive duration")
	}

	// Generate the token value
	var tokenValue string
	var err error

	switch tokenType {
	case TypeLink:
		tokenValue, err = m.generator.GenerateLinkToken()
	case TypeCode:
		tokenValue, err = m.generator.GenerateCodeToken()
	default:
		return nil, fmt.Errorf("unsupported token type: %d", tokenType)
	}

	if err != nil {
		m.logger.Error("failed to generate token",
			"error", err,
			"token_type", tokenType,
			"validation_id", validationID)
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create the token struct
	token := New(tokenValue, tokenType, validationID, ttl)

	// Store the token
	if err := m.storage.Store(ctx, token); err != nil {
		m.logger.Error("failed to store token",
			"error", err,
			"token_type", tokenType,
			"validation_id", validationID)
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	m.logger.Info("token created successfully",
		"token_type", tokenType,
		"validation_id", validationID,
		"expires_at", token.ValidUntil)

	return token, nil
}

// VerifyToken retrieves and validates a token, checking its existence, type, and expiration.
func (m *Manager) VerifyToken(ctx context.Context, tokenValue string, tokenType Type) (*Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	if tokenValue == "" {
		return nil, ErrEmptyTokenValue
	}

	// Retrieve the token from storage
	token, err := m.storage.Retrieve(ctx, tokenValue, tokenType)
	if err != nil {
		// Log verification attempt for security auditing
		m.logger.Warn("token verification failed",
			"token_value", tokenValue,
			"token_type", tokenType,
			"error", err)
		return nil, fmt.Errorf("failed to retrieve token from storage: %w", err)
	}

	// Additional verification checks
	if token.Type != tokenType {
		m.logger.Warn("token type mismatch during verification",
			"expected_type", tokenType,
			"actual_type", token.Type,
			"validation_id", token.ValidationID)
		return nil, fmt.Errorf("token type mismatch: expected %d, got %d", tokenType, token.Type)
	}

	// The storage backend already handles expiration checking,
	// but we double-check here for additional security
	if token.IsExpired() {
		m.logger.Warn("expired token detected during verification",
			"token_type", tokenType,
			"validation_id", token.ValidationID,
			"expired_at", token.ValidUntil)
		return nil, &TokenExpiredError{
			TokenValue: tokenValue,
			TokenType:  tokenType,
			ExpiredAt:  token.ValidUntil,
		}
	}

	m.logger.Info("token verified successfully",
		"token_type", tokenType,
		"validation_id", token.ValidationID)

	return token, nil
}

// InvalidateToken removes a token from storage, effectively invalidating it.
func (m *Manager) InvalidateToken(ctx context.Context, tokenValue string, tokenType Type) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if tokenValue == "" {
		return ErrEmptyTokenValue
	}

	err := m.storage.Delete(ctx, tokenValue, tokenType)
	if err != nil {
		m.logger.Error("failed to invalidate token",
			"error", err,
			"token_value", tokenValue,
			"token_type", tokenType)
		return fmt.Errorf("failed to invalidate token: %w", err)
	}

	m.logger.Info("token invalidated successfully",
		"token_type", tokenType,
		"token_value", tokenValue)

	return nil
}

// InvalidateValidation removes all tokens associated with a validation ID.
func (m *Manager) InvalidateValidation(ctx context.Context, validationID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if validationID == "" {
		return ErrEmptyValidationID
	}

	err := m.storage.DeleteByValidationID(ctx, validationID)
	if err != nil {
		m.logger.Error("failed to invalidate validation tokens",
			"error", err,
			"validation_id", validationID)
		return fmt.Errorf("failed to invalidate validation tokens: %w", err)
	}

	m.logger.Info("validation tokens invalidated successfully",
		"validation_id", validationID)

	return nil
}

// GetTokenInfo retrieves token information without performing full verification.
// This is useful for debugging and administrative purposes.
func (m *Manager) GetTokenInfo(ctx context.Context, tokenValue string, tokenType Type) (*Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	if tokenValue == "" {
		return nil, ErrEmptyTokenValue
	}

	token, err := m.storage.Retrieve(ctx, tokenValue, tokenType)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token info from storage: %w", err)
	}

	m.logger.Debug("token info retrieved",
		"token_type", tokenType,
		"validation_id", token.ValidationID,
		"expired", token.IsExpired())

	return token, nil
}
