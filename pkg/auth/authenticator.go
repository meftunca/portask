package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret       string        `yaml:"jwt_secret" json:"jwt_secret"`
	JWTExpiration   time.Duration `yaml:"jwt_expiration" json:"jwt_expiration"`
	APIKeyLength    int           `yaml:"api_key_length" json:"api_key_length"`
	EnableRateLimit bool          `yaml:"enable_rate_limit" json:"enable_rate_limit"`
	RateLimitRPS    int           `yaml:"rate_limit_rps" json:"rate_limit_rps"`
	EnableAuditLog  bool          `yaml:"enable_audit_log" json:"enable_audit_log"`
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		JWTSecret:       "portask-default-secret-change-in-production",
		JWTExpiration:   24 * time.Hour,
		APIKeyLength:    32,
		EnableRateLimit: true,
		RateLimitRPS:    1000,
		EnableAuditLog:  true,
	}
}

// User represents a system user
type User struct {
	ID        string            `json:"id"`
	Username  string            `json:"username"`
	Email     string            `json:"email"`
	Roles     []string          `json:"roles"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// APIKey represents an API key for service authentication
type APIKey struct {
	ID          string            `json:"id"`
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	UserID      string            `json:"user_id"`
	Permissions []string          `json:"permissions"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Active      bool              `json:"active"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// AuthContext holds authentication context information
type AuthContext struct {
	User        *User
	APIKey      *APIKey
	Permissions []string
	AuthMethod  string // "jwt", "api_key", "anonymous"
	RequestID   string
	ClientIP    string
	UserAgent   string
}

// Permission constants
const (
	PermissionMessageRead   = "message:read"
	PermissionMessageWrite  = "message:write"
	PermissionMessageDelete = "message:delete"
	PermissionTopicCreate   = "topic:create"
	PermissionTopicDelete   = "topic:delete"
	PermissionTopicList     = "topic:list"
	PermissionAdminRead     = "admin:read"
	PermissionAdminWrite    = "admin:write"
	PermissionMetricsRead   = "metrics:read"
	PermissionSystemManage  = "system:manage"
)

// Role constants
const (
	RoleAdmin    = "admin"
	RoleProducer = "producer"
	RoleConsumer = "consumer"
	RoleReadOnly = "readonly"
	RoleService  = "service"
)

// AuthenticatorInterface defines authentication methods
type AuthenticatorInterface interface {
	// JWT methods
	GenerateJWT(user *User) (string, error)
	ValidateJWT(tokenString string) (*JWTClaims, error)

	// API Key methods
	GenerateAPIKey(userID, name, description string, permissions []string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(keyString string) (*APIKey, error)
	RevokeAPIKey(keyID string) error

	// User management
	CreateUser(username, email string, roles []string) (*User, error)
	GetUser(userID string) (*User, error)
	UpdateUser(userID string, updates map[string]interface{}) (*User, error)
	DeleteUser(userID string) error

	// Permission checking
	HasPermission(authCtx *AuthContext, permission string) bool
	HasRole(authCtx *AuthContext, role string) bool
}

// Authenticator implements authentication functionality
type Authenticator struct {
	config  *AuthConfig
	users   map[string]*User   // In-memory store - use database in production
	apiKeys map[string]*APIKey // In-memory store - use database in production

	// Audit logging
	auditLogger AuditLogger
}

// AuditEvent represents an audit log event
type AuditEvent struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"event_type"`
	UserID    string                 `json:"user_id,omitempty"`
	APIKeyID  string                 `json:"api_key_id,omitempty"`
	Resource  string                 `json:"resource"`
	Action    string                 `json:"action"`
	Result    string                 `json:"result"` // "success", "failure", "denied"
	ClientIP  string                 `json:"client_ip"`
	UserAgent string                 `json:"user_agent"`
	RequestID string                 `json:"request_id"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// AuditLogger interface for audit logging
type AuditLogger interface {
	LogEvent(event *AuditEvent) error
	GetEvents(filters map[string]interface{}, limit int) ([]*AuditEvent, error)
}

// NewAuthenticator creates a new authenticator instance
func NewAuthenticator(config *AuthConfig, auditLogger AuditLogger) *Authenticator {
	if config == nil {
		config = DefaultAuthConfig()
	}

	auth := &Authenticator{
		config:      config,
		users:       make(map[string]*User),
		apiKeys:     make(map[string]*APIKey),
		auditLogger: auditLogger,
	}

	// Create default admin user
	adminUser, _ := auth.CreateUser("admin", "admin@portask.local", []string{RoleAdmin})

	// Create default API key for admin
	auth.GenerateAPIKey(adminUser.ID, "default-admin-key", "Default admin API key",
		[]string{PermissionSystemManage}, nil)

	return auth
}

// GenerateJWT creates a new JWT token for a user
func (a *Authenticator) GenerateJWT(user *User) (string, error) {
	permissions := a.getRolePermissions(user.Roles)

	claims := &JWTClaims{
		UserID:      user.ID,
		Username:    user.Username,
		Roles:       user.Roles,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.config.JWTExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "portask",
			Subject:   user.ID,
			ID:        generateRandomString(16),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JWTSecret))
}

// ValidateJWT validates a JWT token and returns claims
func (a *Authenticator) ValidateJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// GenerateAPIKey creates a new API key
func (a *Authenticator) GenerateAPIKey(userID, name, description string, permissions []string, expiresAt *time.Time) (*APIKey, error) {
	keyString := generateRandomString(a.config.APIKeyLength)

	apiKey := &APIKey{
		ID:          generateRandomString(16),
		Key:         keyString,
		Name:        name,
		Description: description,
		UserID:      userID,
		Permissions: permissions,
		Active:      true,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
	}

	a.apiKeys[keyString] = apiKey

	// Log audit event
	if a.auditLogger != nil && a.config.EnableAuditLog {
		a.auditLogger.LogEvent(&AuditEvent{
			ID:        generateRandomString(16),
			Timestamp: time.Now(),
			EventType: "api_key_created",
			UserID:    userID,
			APIKeyID:  apiKey.ID,
			Resource:  "api_key",
			Action:    "create",
			Result:    "success",
			Details: map[string]interface{}{
				"key_name":    name,
				"permissions": permissions,
			},
		})
	}

	return apiKey, nil
}

// ValidateAPIKey validates an API key and returns key information
func (a *Authenticator) ValidateAPIKey(keyString string) (*APIKey, error) {
	apiKey, exists := a.apiKeys[keyString]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	if !apiKey.Active {
		return nil, fmt.Errorf("API key is disabled")
	}

	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last used timestamp
	now := time.Now()
	apiKey.LastUsedAt = &now

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (a *Authenticator) RevokeAPIKey(keyID string) error {
	for keyString, apiKey := range a.apiKeys {
		if apiKey.ID == keyID {
			apiKey.Active = false

			// Log audit event
			if a.auditLogger != nil && a.config.EnableAuditLog {
				a.auditLogger.LogEvent(&AuditEvent{
					ID:        generateRandomString(16),
					Timestamp: time.Now(),
					EventType: "api_key_revoked",
					APIKeyID:  keyID,
					Resource:  "api_key",
					Action:    "revoke",
					Result:    "success",
				})
			}

			delete(a.apiKeys, keyString)
			return nil
		}
	}

	return fmt.Errorf("API key not found")
}

// CreateUser creates a new user
func (a *Authenticator) CreateUser(username, email string, roles []string) (*User, error) {
	userID := generateRandomString(16)

	user := &User{
		ID:        userID,
		Username:  username,
		Email:     email,
		Roles:     roles,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	a.users[userID] = user

	// Log audit event
	if a.auditLogger != nil && a.config.EnableAuditLog {
		a.auditLogger.LogEvent(&AuditEvent{
			ID:        generateRandomString(16),
			Timestamp: time.Now(),
			EventType: "user_created",
			UserID:    userID,
			Resource:  "user",
			Action:    "create",
			Result:    "success",
			Details: map[string]interface{}{
				"username": username,
				"email":    email,
				"roles":    roles,
			},
		})
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (a *Authenticator) GetUser(userID string) (*User, error) {
	user, exists := a.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// UpdateUser updates user information
func (a *Authenticator) UpdateUser(userID string, updates map[string]interface{}) (*User, error) {
	user, exists := a.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	// Apply updates
	if username, ok := updates["username"].(string); ok {
		user.Username = username
	}
	if email, ok := updates["email"].(string); ok {
		user.Email = email
	}
	if roles, ok := updates["roles"].([]string); ok {
		user.Roles = roles
	}
	if active, ok := updates["active"].(bool); ok {
		user.Active = active
	}

	user.UpdatedAt = time.Now()

	return user, nil
}

// DeleteUser deletes a user
func (a *Authenticator) DeleteUser(userID string) error {
	if _, exists := a.users[userID]; !exists {
		return fmt.Errorf("user not found")
	}

	delete(a.users, userID)

	// Revoke all API keys for this user
	for keyString, apiKey := range a.apiKeys {
		if apiKey.UserID == userID {
			delete(a.apiKeys, keyString)
		}
	}

	return nil
}

// HasPermission checks if the auth context has a specific permission
func (a *Authenticator) HasPermission(authCtx *AuthContext, permission string) bool {
	for _, perm := range authCtx.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// HasRole checks if the auth context has a specific role
func (a *Authenticator) HasRole(authCtx *AuthContext, role string) bool {
	if authCtx.User != nil {
		for _, userRole := range authCtx.User.Roles {
			if userRole == role {
				return true
			}
		}
	}
	return false
}

// getRolePermissions returns permissions for given roles
func (a *Authenticator) getRolePermissions(roles []string) []string {
	permissionSet := make(map[string]bool)

	for _, role := range roles {
		switch role {
		case RoleAdmin:
			// Admin has all permissions
			adminPerms := []string{
				PermissionMessageRead, PermissionMessageWrite, PermissionMessageDelete,
				PermissionTopicCreate, PermissionTopicDelete, PermissionTopicList,
				PermissionAdminRead, PermissionAdminWrite,
				PermissionMetricsRead, PermissionSystemManage,
			}
			for _, perm := range adminPerms {
				permissionSet[perm] = true
			}
		case RoleProducer:
			permissionSet[PermissionMessageWrite] = true
			permissionSet[PermissionTopicList] = true
		case RoleConsumer:
			permissionSet[PermissionMessageRead] = true
			permissionSet[PermissionTopicList] = true
		case RoleReadOnly:
			permissionSet[PermissionMessageRead] = true
			permissionSet[PermissionTopicList] = true
			permissionSet[PermissionMetricsRead] = true
		case RoleService:
			permissionSet[PermissionMessageRead] = true
			permissionSet[PermissionMessageWrite] = true
			permissionSet[PermissionTopicList] = true
		}
	}

	permissions := make([]string, 0, len(permissionSet))
	for perm := range permissionSet {
		permissions = append(permissions, perm)
	}

	return permissions
}

// Middleware function for HTTP authentication
func (a *Authenticator) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authCtx, err := a.authenticateRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Add auth context to request context
		ctx := context.WithValue(r.Context(), "auth", authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// authenticateRequest extracts and validates authentication from HTTP request
func (a *Authenticator) authenticateRequest(r *http.Request) (*AuthContext, error) {
	// Check for JWT token in Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := a.ValidateJWT(tokenString)
			if err != nil {
				return nil, err
			}

			user, err := a.GetUser(claims.UserID)
			if err != nil {
				return nil, err
			}

			return &AuthContext{
				User:        user,
				Permissions: claims.Permissions,
				AuthMethod:  "jwt",
				ClientIP:    getClientIP(r),
				UserAgent:   r.UserAgent(),
			}, nil
		}
	}

	// Check for API key in X-API-Key header
	apiKeyHeader := r.Header.Get("X-API-Key")
	if apiKeyHeader != "" {
		apiKey, err := a.ValidateAPIKey(apiKeyHeader)
		if err != nil {
			return nil, err
		}

		user, _ := a.GetUser(apiKey.UserID)

		return &AuthContext{
			APIKey:      apiKey,
			User:        user,
			Permissions: apiKey.Permissions,
			AuthMethod:  "api_key",
			ClientIP:    getClientIP(r),
			UserAgent:   r.UserAgent(),
		}, nil
	}

	return nil, fmt.Errorf("no authentication provided")
}

// Helper functions

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// getClientIP extracts client IP from HTTP request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Use RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}

// secureCompare performs constant-time string comparison
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
