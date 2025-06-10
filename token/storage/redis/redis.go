// Package redis provides a Redis-backed implementation of token storage.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jaeyeom/email-validator-grpc-mcp/token"
	"github.com/redis/go-redis/v9"
)

// Storage provides a Redis-backed implementation for token storage.
type Storage struct {
	client *redis.Client
	logger *slog.Logger
}

// Option is a functional option for configuring Storage.
type Option func(*Storage)

// WithLogger sets a custom logger for Storage.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Storage) {
		s.logger = logger
	}
}

// WithRedisClient sets a custom Redis client for Storage.
func WithRedisClient(client *redis.Client) Option {
	return func(s *Storage) {
		s.client = client
	}
}

// New creates a new Redis-backed token storage.
func New(client *redis.Client, opts ...Option) *Storage {
	s := &Storage{
		client: client,
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Store saves a token to Redis.
// The token is stored with a composite key and will expire according to its ValidUntil field.
func (s *Storage) Store(ctx context.Context, t *token.Token) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if err := token.Validate(t); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	// Serialize token to JSON
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Calculate TTL based on token expiration
	now := time.Now()
	if t.ValidUntil.Before(now) {
		return token.ErrInvalidToken
	}
	ttl := t.ValidUntil.Sub(now)

	// Store token in Redis with expiration
	key := fmt.Sprintf("token:%s:%d", t.Value, t.Type)
	err = s.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to store token in Redis: %w", err)
	}

	// Store validation ID index
	validationKey := fmt.Sprintf("validation:%s", t.ValidationID)
	err = s.client.SAdd(ctx, validationKey, key).Err()
	if err != nil {
		return fmt.Errorf("failed to store validation ID index: %w", err)
	}

	// Set expiration on validation ID index
	err = s.client.ExpireAt(ctx, validationKey, t.ValidUntil).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration on validation ID index: %w", err)
	}

	s.logger.Debug("token stored in Redis",
		"token_type", t.Type,
		"validation_id", t.ValidationID)

	return nil
}

// Retrieve gets a token from Redis by its value and type.
// Returns token.ErrTokenNotFound if the token does not exist.
// Returns token.TokenExpiredError if the token exists but has expired.
func (s *Storage) Retrieve(ctx context.Context, tokenValue string, tokenType token.Type) (*token.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	// Construct the key
	key := fmt.Sprintf("token:%s:%d", tokenValue, tokenType)

	// Get token data from Redis
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			s.logger.Debug("token not found in Redis",
				"token_value", tokenValue,
				"token_type", tokenType)
			return nil, token.ErrTokenNotFound
		}
		s.logger.Error("failed to retrieve token from Redis", "error", err)
		return nil, fmt.Errorf("failed to retrieve token from Redis: %w", err)
	}

	// Deserialize token
	var t token.Token
	if err := json.Unmarshal(data, &t); err != nil {
		s.logger.Error("failed to unmarshal token", "error", err)
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Check if token has expired
	if t.IsExpired() {
		// Delete expired token
		s.client.Del(ctx, key)
		s.logger.Debug("expired token retrieved and deleted",
			"token_type", t.Type,
			"validation_id", t.ValidationID)
		return nil, &token.TokenExpiredError{
			TokenValue: tokenValue,
			TokenType:  tokenType,
			ExpiredAt:  t.ValidUntil,
		}
	}

	s.logger.Debug("token retrieved from Redis",
		"token_type", t.Type,
		"validation_id", t.ValidationID)
	return &t, nil
}

// Delete removes a token from Redis.
// This operation is idempotent and will not return an error if the token does not exist.
func (s *Storage) Delete(ctx context.Context, tokenValue string, tokenType token.Type) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	// Construct the key
	key := fmt.Sprintf("token:%s:%d", tokenValue, tokenType)

	// Get the token to find its validation ID
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Token doesn't exist, nothing to delete
			s.logger.Debug("token not found for deletion",
				"token_value", tokenValue,
				"token_type", tokenType)
			return nil
		}
		s.logger.Error("failed to retrieve token for deletion", "error", err)
		return fmt.Errorf("failed to retrieve token for deletion: %w", err)
	}

	// Deserialize token to get validation ID
	var t token.Token
	if err := json.Unmarshal(data, &t); err != nil {
		s.logger.Error("failed to unmarshal token for deletion", "error", err)
		return fmt.Errorf("failed to unmarshal token for deletion: %w", err)
	}

	// Remove token from validation ID index
	validationKey := fmt.Sprintf("validation:%s", t.ValidationID)
	err = s.client.SRem(ctx, validationKey, key).Err()
	if err != nil && err != redis.Nil {
		s.logger.Error("failed to remove token from validation index", "error", err)
		return fmt.Errorf("failed to remove token from validation index: %w", err)
	}

	// Delete the token
	err = s.client.Del(ctx, key).Err()
	if err != nil && err != redis.Nil {
		s.logger.Error("failed to delete token", "error", err)
		return fmt.Errorf("failed to delete token: %w", err)
	}

	s.logger.Debug("token deleted from Redis",
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

	// Get all token keys for this validation ID
	validationKey := fmt.Sprintf("validation:%s", validationID)
	keys, err := s.client.SMembers(ctx, validationKey).Result()
	if err != nil {
		if err == redis.Nil {
			// No tokens for this validation ID
			s.logger.Debug("no tokens found for validation ID", "validation_id", validationID)
			return nil
		}
		s.logger.Error("failed to get token keys for validation ID", "error", err, "validation_id", validationID)
		return fmt.Errorf("failed to get token keys for validation ID: %w", err)
	}

	if len(keys) == 0 {
		// No tokens for this validation ID
		s.logger.Debug("no tokens found for validation ID", "validation_id", validationID)
		return nil
	}

	// Delete all tokens
	pipe := s.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	// Delete the validation ID index
	pipe.Del(ctx, validationKey)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("failed to delete tokens by validation ID", "error", err, "validation_id", validationID)
		return fmt.Errorf("failed to delete tokens by validation ID: %w", err)
	}

	s.logger.Debug("tokens deleted by validation ID",
		"validation_id", validationID,
		"count", len(keys))

	return nil
}
