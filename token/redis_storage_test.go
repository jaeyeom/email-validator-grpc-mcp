package token

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisStorage_Store(t *testing.T) {
	// Skip this test if we don't have a Redis server available
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}

	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Use default DB
	})

	// Ping Redis to check if it's available
	ctx := t.Context()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server not available, skipping test")
	}

	// Create a new RedisStorage
	storage := NewRedisStorage(rdb)

	// Create a valid token for testing
	validToken := &Token{
		Value:        "test-token",
		Type:         TypeLink,
		ValidationID: "test-validation-id",
		ValidUntil:   time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}

	// Test storing a valid token
	err := storage.Store(ctx, validToken)
	if err != nil {
		t.Errorf("Store() error = %v, want nil", err)
	}

	// Clean up
	t.Cleanup(func() {
		_ = rdb.FlushDB(ctx).Err()
		_ = rdb.Close()
	})
}

func TestRedisStorage_Retrieve(t *testing.T) {
	// Skip this test if we don't have a Redis server available
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}

	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Use default DB
	})

	// Ping Redis to check if it's available
	ctx := t.Context()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server not available, skipping test")
	}

	// Create a new RedisStorage
	storage := NewRedisStorage(rdb)

	// Create a valid token for testing
	validToken := &Token{
		Value:        "test-token",
		Type:         TypeLink,
		ValidationID: "test-validation-id",
		ValidUntil:   time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}

	// Store the token first
	err := storage.Store(ctx, validToken)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Test retrieving a valid token
	retrievedToken, err := storage.Retrieve(ctx, validToken.Value, validToken.Type)
	if err != nil {
		t.Errorf("Retrieve() error = %v, want nil", err)
	}
	if retrievedToken == nil {
		t.Errorf("Retrieve() = nil, want token")
	} else {
		if retrievedToken.Value != validToken.Value {
			t.Errorf("Retrieve().Value = %v, want %v", retrievedToken.Value, validToken.Value)
		}
		if retrievedToken.Type != validToken.Type {
			t.Errorf("Retrieve().Type = %v, want %v", retrievedToken.Type, validToken.Type)
		}
		if retrievedToken.ValidationID != validToken.ValidationID {
			t.Errorf("Retrieve().ValidationID = %v, want %v", retrievedToken.ValidationID, validToken.ValidationID)
		}
	}

	// Clean up
	t.Cleanup(func() {
		_ = rdb.FlushDB(ctx).Err()
		_ = rdb.Close()
	})
}

func TestRedisStorage_Delete(t *testing.T) {
	// Skip this test if we don't have a Redis server available
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}

	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Use default DB
	})

	// Ping Redis to check if it's available
	ctx := t.Context()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server not available, skipping test")
	}

	// Create a new RedisStorage
	storage := NewRedisStorage(rdb)

	// Create a valid token for testing
	validToken := &Token{
		Value:        "test-token",
		Type:         TypeLink,
		ValidationID: "test-validation-id",
		ValidUntil:   time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}

	// Store the token first
	err := storage.Store(ctx, validToken)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Test deleting the token
	err = storage.Delete(ctx, validToken.Value, validToken.Type)
	if err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}

	// Verify the token is deleted
	_, err = storage.Retrieve(ctx, validToken.Value, validToken.Type)
	if err == nil {
		t.Errorf("Retrieve() after Delete() error = nil, want error")
	}

	// Clean up
	t.Cleanup(func() {
		_ = rdb.FlushDB(ctx).Err()
		_ = rdb.Close()
	})
}

func TestRedisStorage_DeleteByValidationID(t *testing.T) {
	// Skip this test if we don't have a Redis server available
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}

	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Use default DB
	})

	// Ping Redis to check if it's available
	ctx := t.Context()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server not available, skipping test")
	}

	// Create a new RedisStorage
	storage := NewRedisStorage(rdb)

	// Create multiple tokens with the same validation ID
	validationID := "test-validation-id"
	tokens := []*Token{
		{
			Value:        "test-token-1",
			Type:         TypeLink,
			ValidationID: validationID,
			ValidUntil:   time.Now().Add(time.Hour),
			CreatedAt:    time.Now(),
		},
		{
			Value:        "test-token-2",
			Type:         TypeCode,
			ValidationID: validationID,
			ValidUntil:   time.Now().Add(time.Hour),
			CreatedAt:    time.Now(),
		},
	}

	// Store all tokens
	for i, token := range tokens {
		err := storage.Store(ctx, token)
		if err != nil {
			t.Fatalf("Failed to store token %d: %v", i, err)
		}
	}

	// Test deleting all tokens by validation ID
	err := storage.DeleteByValidationID(ctx, validationID)
	if err != nil {
		t.Errorf("DeleteByValidationID() error = %v, want nil", err)
	}

	// Verify all tokens are deleted
	for i, token := range tokens {
		_, err := storage.Retrieve(ctx, token.Value, token.Type)
		if err == nil {
			t.Errorf("Retrieve() after DeleteByValidationID() for token %d error = nil, want error", i)
		}
	}

	// Clean up
	t.Cleanup(func() {
		_ = rdb.FlushDB(ctx).Err()
		_ = rdb.Close()
	})
}

func TestRedisStorage_ExpiredToken(t *testing.T) {
	// Skip this test if we don't have a Redis server available
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}

	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Use default DB
	})

	// Ping Redis to check if it's available
	ctx := t.Context()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server not available, skipping test")
	}

	// Create a new RedisStorage
	storage := NewRedisStorage(rdb)

	// Create a token that is already expired
	expiredToken := &Token{
		Value:        "expired-token",
		Type:         TypeLink,
		ValidationID: "test-validation-id",
		ValidUntil:   time.Now().Add(time.Hour), // Set future expiry for storage
		CreatedAt:    time.Now(),
	}

	// Store the token
	err := storage.Store(ctx, expiredToken)
	if err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}

	// Manually update the token to be expired in Redis
	key := "token:" + expiredToken.Value + ":" + "0" // TypeLink is 0
	// Get the token data
	data, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		t.Fatalf("Failed to get token data: %v", err)
	}

	// Deserialize, modify expiration, and serialize back
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		t.Fatalf("Failed to unmarshal token: %v", err)
	}
	token.ValidUntil = time.Now().Add(-time.Hour) // Set to expired

	updatedData, err := json.Marshal(&token)
	if err != nil {
		t.Fatalf("Failed to marshal updated token: %v", err)
	}

	// Store the updated token back
	if err := rdb.Set(ctx, key, updatedData, time.Hour).Err(); err != nil {
		t.Fatalf("Failed to update token: %v", err)
	}

	// Try to retrieve the expired token
	_, err = storage.Retrieve(ctx, expiredToken.Value, expiredToken.Type)
	if !IsTokenExpiredError(err) {
		t.Errorf("Retrieve() expired token error = %v, want TokenExpiredError", err)
	}

	// Clean up
	t.Cleanup(func() {
		_ = rdb.FlushDB(ctx).Err()
		_ = rdb.Close()
	})
}

func TestRedisStorage_CancelledContext(t *testing.T) {
	// Skip this test if we don't have a Redis server available
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}

	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Use default DB
	})

	// Ping Redis to check if it's available
	ctx := t.Context()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server not available, skipping test")
	}

	// Create a new RedisStorage
	storage := NewRedisStorage(rdb)

	// Create a cancelled context
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	// Create a valid token for testing
	validToken := &Token{
		Value:        "test-token",
		Type:         TypeLink,
		ValidationID: "test-validation-id",
		ValidUntil:   time.Now().Add(time.Hour),
		CreatedAt:    time.Now(),
	}

	// Test operations with cancelled context
	testCases := []struct {
		name string
		op   func() error
	}{
		{
			name: "Store with cancelled context",
			op: func() error {
				return storage.Store(cancelledCtx, validToken)
			},
		},
		{
			name: "Retrieve with cancelled context",
			op: func() error {
				_, err := storage.Retrieve(cancelledCtx, validToken.Value, validToken.Type)
				return err
			},
		},
		{
			name: "Delete with cancelled context",
			op: func() error {
				return storage.Delete(cancelledCtx, validToken.Value, validToken.Type)
			},
		},
		{
			name: "DeleteByValidationID with cancelled context",
			op: func() error {
				return storage.DeleteByValidationID(cancelledCtx, validToken.ValidationID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.op()
			if err == nil {
				t.Errorf("%s error = nil, want context.Canceled", tc.name)
			}
		})
	}

	// Clean up
	t.Cleanup(func() {
		_ = rdb.FlushDB(ctx).Err()
		_ = rdb.Close()
	})
}
