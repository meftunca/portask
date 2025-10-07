package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthenticator(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	assert.NotNil(t, auth)
	assert.NotNil(t, auth.users)
	assert.NotNil(t, auth.apiKeys)
	assert.Equal(t, config, auth.config)
}

func TestAuthenticator_GenerateToken(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	t.Run("successful token generation", func(t *testing.T) {
		token, expiresAt, err := auth.GenerateToken("test-user", []string{"user", "admin"})
		
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.True(t, expiresAt.After(time.Now()))
		assert.True(t, expiresAt.Before(time.Now().Add(25*time.Hour))) // Within expiration
	})
	
	t.Run("token with empty roles", func(t *testing.T) {
		token, _, err := auth.GenerateToken("test-user", []string{})
		
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})
}

func TestAuthenticator_ValidateToken(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	t.Run("valid token", func(t *testing.T) {
		token, _, err := auth.GenerateToken("test-user", []string{"user"})
		require.NoError(t, err)
		
		claims, err := auth.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "test-user", claims.UserID)
		assert.Contains(t, claims.Roles, "user")
	})
	
	t.Run("invalid token", func(t *testing.T) {
		claims, err := auth.ValidateToken("invalid-token")
		
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
	
	t.Run("expired token", func(t *testing.T) {
		// Create auth with very short expiration
		shortConfig := DefaultAuthConfig()
		shortConfig.JWTExpiration = 1 * time.Nanosecond
		shortAuth := NewAuthenticator(shortConfig, nil)
		
		token, _, err := shortAuth.GenerateToken("test-user", []string{"user"})
		require.NoError(t, err)
		
		time.Sleep(2 * time.Millisecond)
		
		claims, err := shortAuth.ValidateToken(token)
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

func TestAuthenticator_GenerateAPIKey(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	t.Run("successful API key generation", func(t *testing.T) {
		apiKey, err := auth.GenerateAPIKey(
			"test-user",
			"test-key",
			"Test API Key",
			[]string{"read", "write"},
			nil,
		)
		
		require.NoError(t, err)
		assert.NotEmpty(t, apiKey.ID)
		assert.NotEmpty(t, apiKey.Key)
		assert.Equal(t, "test-user", apiKey.UserID)
		assert.Equal(t, "test-key", apiKey.Name)
		assert.True(t, apiKey.Active)
		assert.Nil(t, apiKey.ExpiresAt)
	})
	
	t.Run("API key with expiration", func(t *testing.T) {
		expiresAt := time.Now().Add(24 * time.Hour)
		apiKey, err := auth.GenerateAPIKey(
			"test-user",
			"expiring-key",
			"Expiring Key",
			nil,
			&expiresAt,
		)
		
		require.NoError(t, err)
		assert.NotNil(t, apiKey.ExpiresAt)
		assert.True(t, apiKey.ExpiresAt.After(time.Now()))
	})
}

func TestAuthenticator_ValidateAPIKey(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	t.Run("valid API key", func(t *testing.T) {
		apiKey, err := auth.GenerateAPIKey("test-user", "test-key", "Test", nil, nil)
		require.NoError(t, err)
		
		validated, err := auth.ValidateAPIKey(apiKey.Key)
		require.NoError(t, err)
		assert.Equal(t, apiKey.ID, validated.ID)
		assert.NotNil(t, validated.LastUsedAt)
	})
	
	t.Run("invalid API key", func(t *testing.T) {
		validated, err := auth.ValidateAPIKey("invalid-key")
		
		assert.Error(t, err)
		assert.Nil(t, validated)
	})
	
	t.Run("expired API key", func(t *testing.T) {
		expiredTime := time.Now().Add(-1 * time.Hour)
		apiKey, err := auth.GenerateAPIKey("test-user", "expired-key", "Expired", nil, &expiredTime)
		require.NoError(t, err)
		
		validated, err := auth.ValidateAPIKey(apiKey.Key)
		assert.Error(t, err)
		assert.Nil(t, validated)
	})
	
	t.Run("disabled API key", func(t *testing.T) {
		apiKey, err := auth.GenerateAPIKey("test-user", "disabled-key", "Disabled", nil, nil)
		require.NoError(t, err)
		
		// Disable the key
		err = auth.RevokeAPIKey(apiKey.ID)
		require.NoError(t, err)
		
		validated, err := auth.ValidateAPIKey(apiKey.Key)
		assert.Error(t, err)
		assert.Nil(t, validated)
	})
}

func TestAuthenticator_ValidateAPIKeySimple(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	apiKey, err := auth.GenerateAPIKey("test-user", "simple-key", "Simple", nil, nil)
	require.NoError(t, err)
	
	t.Run("valid key", func(t *testing.T) {
		valid, userID := auth.ValidateAPIKeySimple(apiKey.Key)
		assert.True(t, valid)
		assert.Equal(t, "test-user", userID)
	})
	
	t.Run("invalid key", func(t *testing.T) {
		valid, userID := auth.ValidateAPIKeySimple("invalid")
		assert.False(t, valid)
		assert.Empty(t, userID)
	})
}

func TestAuthenticator_CreateUser(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	t.Run("successful user creation", func(t *testing.T) {
		user, err := auth.CreateUser("testuser", "test@example.com", []string{"user"})
		
		require.NoError(t, err)
		assert.NotEmpty(t, user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Contains(t, user.Roles, "user")
		assert.True(t, user.Active)
	})
	
	t.Run("user with multiple roles", func(t *testing.T) {
		user, err := auth.CreateUser("adminuser", "admin@example.com", []string{"user", "admin"})
		
		require.NoError(t, err)
		assert.Len(t, user.Roles, 2)
		assert.Contains(t, user.Roles, "user")
		assert.Contains(t, user.Roles, "admin")
	})
}

func TestAuthenticator_GetUser(t *testing.T) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	t.Run("existing user", func(t *testing.T) {
		created, err := auth.CreateUser("getuser", "get@example.com", []string{"user"})
		require.NoError(t, err)
		
		user, err := auth.GetUser(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, user.ID)
		assert.Equal(t, "getuser", user.Username)
	})
	
	t.Run("non-existent user", func(t *testing.T) {
		user, err := auth.GetUser("non-existent-id")
		
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestDefaultAuthConfig(t *testing.T) {
	config := DefaultAuthConfig()
	
	assert.NotNil(t, config)
	assert.NotEmpty(t, config.JWTSecret)
	assert.Equal(t, 24*time.Hour, config.JWTExpiration)
	assert.Equal(t, 32, config.APIKeyLength)
	assert.True(t, config.EnableRateLimit)
	assert.Equal(t, 1000, config.RateLimitRPS)
	assert.True(t, config.EnableAuditLog)
}

func TestGenerateSecureKey(t *testing.T) {
	t.Run("generate 16 byte key", func(t *testing.T) {
		key, err := GenerateSecureKey(16)
		require.NoError(t, err)
		assert.Len(t, key, 32) // Hex encoding doubles the length
	})
	
	t.Run("generate 32 byte key", func(t *testing.T) {
		key, err := GenerateSecureKey(32)
		require.NoError(t, err)
		assert.Len(t, key, 64)
	})
	
	t.Run("keys are unique", func(t *testing.T) {
		key1, err1 := GenerateSecureKey(16)
		key2, err2 := GenerateSecureKey(16)
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, key1, key2)
	})
}

func BenchmarkAuthenticator_GenerateToken(b *testing.B) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.GenerateToken("bench-user", []string{"user"})
	}
}

func BenchmarkAuthenticator_ValidateToken(b *testing.B) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	token, _, _ := auth.GenerateToken("bench-user", []string{"user"})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.ValidateToken(token)
	}
}

func BenchmarkAuthenticator_ValidateAPIKey(b *testing.B) {
	config := DefaultAuthConfig()
	auth := NewAuthenticator(config, nil)
	
	apiKey, _ := auth.GenerateAPIKey("bench-user", "bench-key", "Benchmark", nil, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.ValidateAPIKey(apiKey.Key)
	}
}

