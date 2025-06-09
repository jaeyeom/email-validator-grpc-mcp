// Package token provides functionality for secure token generation, storage,
// retrieval, and expiration handling for the email validation service.
package token

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage provides a Redis-backed implementation for token storage.
type RedisStorage struct {
	client *redis.Client
	logger *slog.Logger
}

// RedisOption is a functional option for configuring RedisStorage.
type RedisOption func(*RedisStorage)

// WithLogger sets a custom logger for RedisStorage.
func WithLogger(logger *slog.Logger) RedisOption {
	return func(s *RedisStorage) {
		s.logger = logger
	}
}

// NewRedisStorage creates a new Redis-backed token storage.
func NewRedisStorage(client *redis.Client, opts ...RedisOption) *RedisStorage {
	s := &RedisStorage{
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
// Store saves a token to Redis.
// The token is stored with a composite key and will expire according to its ValidUntil field.
func (s *RedisStorage) Store(ctx context.Context, token *Token) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if err := validateToken(token); err != nil {
		return err
	}

	// Create a composite key for the token
	key := fmt.Sprintf("token:%s:%d", token.Value, token.Type)

	// Serialize the token to JSON
	tokenData, err := json.Marshal(token)
	if err != nil {
		s.logger.Error("failed to marshal token", "error", err)
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Calculate TTL based on token expiration
	ttl := time.Until(token.ValidUntil)
	if ttl <= 0 {
		return ErrInvalidToken
	}

	// Store the token in Redis with expiration
	if err := s.client.Set(ctx, key, tokenData, ttl).Err(); err != nil {
		s.logger.Error("failed to store token in Redis", "error", err)
		return fmt.Errorf("failed to store token in Redis: %w", err)
	}

	// Store a reference in the validation ID index
	indexKey := fmt.Sprintf("validation:%s", token.ValidationID)
	if err := s.client.SAdd(ctx, indexKey, key).Err(); err != nil {
		s.logger.Error("failed to update validation index", "error", err)
		return fmt.Errorf("failed to update validation index: %w", err)
	}

	// Set the same expiration on the index
	if err := s.client.Expire(ctx, indexKey, ttl).Err(); err != nil {
		s.logger.Error("failed to set expiration on validation index", "error", err)
		return fmt.Errorf("failed to set expiration on validation index: %w", err)
	}

	s.logger.Debug("token stored in Redis",
		"token_type", token.Type,
		"validation_id", token.ValidationID,
		"expires_in", ttl.String())

	return nil
}

// Retrieve gets a token from Redis by its value and type.
// Returns TokenExpiredError if the token exists but has expired.
func (s *RedisStorage) Retrieve(ctx context.Context, tokenValue string, tokenType Type) (*Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	// Create the composite key for the token
	key := fmt.Sprintf("token:%s:%d", tokenValue, tokenType)

	// Get the token from Redis
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			s.logger.Debug("token not found", "value", tokenValue, "type", tokenType)
			return nil, ErrTokenNotFound
		}
		s.logger.Error("failed to retrieve token from Redis", "error", err)
		return nil, fmt.Errorf("failed to retrieve token from Redis: %w", err)
	}

	// Deserialize the token
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		s.logger.Error("failed to unmarshal token", "error", err)
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Check if the token has expired
	if time.Now().After(token.ValidUntil) {
		// Delete the expired token
		s.logger.Debug("token expired", "value", tokenValue, "type", tokenType)
		_ = s.Delete(ctx, tokenValue, tokenType)
		return nil, &TokenExpiredError{
			TokenValue: tokenValue,
			TokenType:  tokenType,
			ExpiredAt:  token.ValidUntil,
		}
	}

	s.logger.Debug("token retrieved", "type", token.Type, "validation_id", token.ValidationID)
	return &token, nil
}

// Delete removes a token from Redis.
// This operation is idempotent and will not return an error if the token does not exist.
func (s *RedisStorage) Delete(ctx context.Context, tokenValue string, tokenType Type) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	// Create the composite key for the token
	key := fmt.Sprintf("token:%s:%d", tokenValue, tokenType)

	// Get the token to find its validation ID
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Token doesn't exist, nothing to delete
			return nil
		}
		s.logger.Error("failed to retrieve token for deletion", "error", err)
		return fmt.Errorf("failed to retrieve token for deletion: %w", err)
	}

	// Deserialize the token to get the validation ID
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		s.logger.Error("failed to unmarshal token for deletion", "error", err)
		return fmt.Errorf("failed to unmarshal token for deletion: %w", err)
	}

	// Remove the token from the validation ID index
	indexKey := fmt.Sprintf("validation:%s", token.ValidationID)
	if err := s.client.SRem(ctx, indexKey, key).Err(); err != nil {
		s.logger.Error("failed to update validation index on deletion", "error", err)
		return fmt.Errorf("failed to update validation index on deletion: %w", err)
	}

	// Delete the token
	if err := s.client.Del(ctx, key).Err(); err != nil {
		s.logger.Error("failed to delete token", "error", err)
		return fmt.Errorf("failed to delete token: %w", err)
	}

	s.logger.Debug("token deleted", "value", tokenValue, "type", tokenType)
	return nil
}

// DeleteByValidationID removes all tokens associated with a validation ID.
// This operation is idempotent and will not return an error if no tokens exist for the validation ID.
func (s *RedisStorage) DeleteByValidationID(ctx context.Context, validationID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	if validationID == "" {
		return ErrEmptyValidationID
	}

	// Get all token keys for this validation ID
	indexKey := fmt.Sprintf("validation:%s", validationID)
	keys, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		s.logger.Error("failed to retrieve tokens for validation ID", "error", err)
		return fmt.Errorf("failed to retrieve tokens for validation ID: %w", err)
	}

	if len(keys) == 0 {
		// No tokens found, nothing to delete
		return nil
	}

	// Delete all tokens in a pipeline
	pipe := s.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	// Delete the index itself
	pipe.Del(ctx, indexKey)

	// Execute the pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Error("failed to delete tokens by validation ID", "error", err)
		return fmt.Errorf("failed to delete tokens by validation ID: %w", err)
	}

	s.logger.Debug("tokens deleted by validation ID",
		"validation_id", validationID,
		"count", len(keys))

	return nil
}
