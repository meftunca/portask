package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ValidateCredentials validates username and password
// In production, this should query a database with hashed passwords
func (a *Authenticator) ValidateCredentials(username, password string) (string, error) {
	// TODO: In production, replace this with proper database query and password hashing
	// This is a simple demonstration implementation
	
	// Example: Check if user exists
	for id, user := range a.users {
		if user.Username == username && user.Active {
			// TODO: Implement proper password hashing (bcrypt, argon2, etc.)
			// For now, we'll just return the user ID
			// In production: compare hashed password with stored hash
			return id, nil
		}
	}
	
	return "", fmt.Errorf("invalid credentials")
}

// ValidateToken validates a JWT token and returns claims
func (a *Authenticator) ValidateToken(tokenString string) (*JWTClaims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract and validate claims
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		// Check if token is expired
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// GenerateToken generates a new JWT token for a user
func (a *Authenticator) GenerateToken(userID string, roles []string) (string, time.Time, error) {
	expiresAt := time.Now().Add(a.config.JWTExpiration)
	
	claims := &JWTClaims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "portask",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// GenerateRefreshToken generates a refresh token
func (a *Authenticator) GenerateRefreshToken(userID string, roles []string) (string, time.Time, error) {
	expiresAt := time.Now().Add(a.config.JWTExpiration * 7) // 7x longer than access token
	
	claims := &JWTClaims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "portask",
			Subject:   "refresh",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateAPIKey validates an API key and returns (valid, userID)
func (a *Authenticator) ValidateAPIKeySimple(keyString string) (bool, string) {
	apiKey, err := a.ValidateAPIKey(keyString)
	if err != nil {
		return false, ""
	}
	return true, apiKey.UserID
}

// GenerateSecureKey generates a cryptographically secure random key
func GenerateSecureKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HashPassword hashes a password using bcrypt (placeholder for future implementation)
func HashPassword(password string) (string, error) {
	// TODO: Implement proper password hashing with bcrypt or argon2
	// For now, return a placeholder
	return password, fmt.Errorf("password hashing not implemented - use bcrypt in production")
}

// VerifyPassword verifies a password against its hash (placeholder for future implementation)
func VerifyPassword(password, hash string) error {
	// TODO: Implement proper password verification with bcrypt or argon2
	// For now, return a placeholder error
	return fmt.Errorf("password verification not implemented - use bcrypt in production")
}

// Config alias for backwards compatibility
type Config = AuthConfig

