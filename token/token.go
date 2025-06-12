// Package token provides functionality for secure token generation, storage,
// retrieval, and expiration handling for the email validation service.
package token

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// Type represents the type of token being generated.
type Type int

const (
	// TypeLink is used for validation via clickable links in emails.
	TypeLink Type = iota
	// TypeCode is used for validation via code entry.
	TypeCode
)

// DefaultLinkTokenLength is the default byte length for link tokens before encoding.
const DefaultLinkTokenLength = 32 // 256 bits of entropy

// DefaultCodeTokenLength is the default byte length for code tokens before encoding.
const DefaultCodeTokenLength = 4 // 32 bits of entropy

// DefaultCodeCharset defines the characters used in code tokens.
const DefaultCodeCharset = "0123456789"

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

// Generator provides secure token generation functionality.
type Generator struct {
	linkTokenLength int
	codeTokenLength int
	codeCharset     string
}

// NewGenerator creates a new Generator with secure defaults.
func NewGenerator() *Generator {
	return &Generator{
		linkTokenLength: DefaultLinkTokenLength,
		codeTokenLength: DefaultCodeTokenLength,
		codeCharset:     DefaultCodeCharset,
	}
}

// WithLinkTokenLength sets a custom link token length.
func (g *Generator) WithLinkTokenLength(length int) *Generator {
	g.linkTokenLength = length

	return g
}

// WithCodeTokenLength sets a custom code token length.
func (g *Generator) WithCodeTokenLength(length int) *Generator {
	g.codeTokenLength = length

	return g
}

// WithCodeCharset sets a custom character set for code tokens.
func (g *Generator) WithCodeCharset(charset string) *Generator {
	if charset != "" {
		g.codeCharset = charset
	}

	return g
}

// GenerateLinkToken creates a cryptographically secure random token for link
// validation. The token is URL-safe base64 encoded.
func (g *Generator) GenerateLinkToken() (string, error) {
	bytes := make([]byte, g.linkTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Use URL-safe base64 encoding without padding
	token := base64.RawURLEncoding.EncodeToString(bytes)

	return token, nil
}

// GenerateCodeToken creates a cryptographically secure random token for code
// validation. The token consists of digits from the configured charset.
func (g *Generator) GenerateCodeToken() (string, error) {
	bytes := make([]byte, g.codeTokenLength)
	charsetLength := len(g.codeCharset)

	// Generate random bytes with the crypto/rand package
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Map random bytes to characters in the charset
	code := make([]byte, g.codeTokenLength)
	for i, b := range bytes {
		// Use modulo to map the byte to a character in the charset
		code[i] = g.codeCharset[int(b)%charsetLength]
	}

	return string(code), nil
}

// Token represents a validation token with metadata.
type Token struct {
	Value        string    // The token value
	Type         Type      // The type of token (link or code)
	CreatedAt    time.Time // When the token was created
	ValidUntil   time.Time // When the token expires
	ValidationID string    // ID of the validation this token is associated with
}

// New creates a new Token with the given parameters.
func New(value string, tokenType Type, validationID string, ttl time.Duration) *Token {
	now := time.Now()

	return &Token{
		Value:        value,
		Type:         tokenType,
		CreatedAt:    now,
		ValidUntil:   now.Add(ttl),
		ValidationID: validationID,
	}
}

// IsExpired checks if the token has expired.
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ValidUntil)
}

// ValidateToken checks if a token is valid for storage.
func (t *Token) ValidateToken() error {
	if t == nil {
		return errors.New("token cannot be nil")
	}

	if t.Value == "" {
		return errors.New("token value cannot be empty")
	}

	if t.ValidationID == "" {
		return errors.New("validation ID cannot be empty")
	}

	if t.ValidUntil.IsZero() {
		return fmt.Errorf("invalid token: missing expiration time")
	}

	return nil
}

// TokenExpiredError represents an error when a token has expired.
type TokenExpiredError struct {
	TokenValue string
	TokenType  Type
	ExpiredAt  time.Time
}

// Error implements the error interface.
func (e *TokenExpiredError) Error() string {
	return fmt.Sprintf("token %s of type %d expired at %s", e.TokenValue, e.TokenType, e.ExpiredAt.Format(time.RFC3339))
}

// IsTokenExpiredError checks if an error is a TokenExpiredError.
func IsTokenExpiredError(err error) bool {
	var expiredErr *TokenExpiredError
	return errors.As(err, &expiredErr)
}

// Storage defines the interface for token storage backends.
type Storage interface {
	// Store saves a token to the storage backend.
	Store(ctx context.Context, token *Token) error

	// Retrieve gets a token from the storage backend by its value and type.
	Retrieve(ctx context.Context, tokenValue string, tokenType Type) (*Token, error)

	// Delete removes a token from the storage backend.
	Delete(ctx context.Context, tokenValue string, tokenType Type) error

	// DeleteByValidationID removes all tokens associated with a validation ID.
	DeleteByValidationID(ctx context.Context, validationID string) error
}

// Validate checks if a token is valid for storage.
// This function is exported for use by storage implementations.
func Validate(token *Token) error {
	if token == nil {
		return ErrTokenNil
	}

	if token.Value == "" {
		return ErrEmptyTokenValue
	}

	if token.ValidationID == "" {
		return ErrEmptyValidationID
	}

	if token.ValidUntil.IsZero() {
		return fmt.Errorf("%w: missing expiration time", ErrInvalidToken)
	}

	return nil
}
