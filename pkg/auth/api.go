package auth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// AuthAPI provides HTTP endpoints for authentication management
type AuthAPI struct {
	auth        *Authenticator
	router      *mux.Router
	rateLimiter *RateLimiter
}

// NewAuthAPI creates a new authentication API
func NewAuthAPI(auth *Authenticator) *AuthAPI {
	api := &AuthAPI{
		auth:        auth,
		router:      mux.NewRouter(),
		rateLimiter: NewRateLimiter(auth.config.RateLimitRPS),
	}

	api.setupRoutes()
	return api
}

// setupRoutes configures API routes
func (api *AuthAPI) setupRoutes() {
	// Public routes (no authentication required)
	api.router.HandleFunc("/auth/login", api.loginHandler).Methods("POST")
	api.router.HandleFunc("/auth/health", api.healthHandler).Methods("GET")

	// Protected routes (authentication required)
	protected := api.router.PathPrefix("/auth").Subrouter()
	protected.Use(func(next http.Handler) http.Handler {
		return api.auth.AuthMiddleware(next.ServeHTTP)
	})
	protected.Use(api.rateLimitMiddleware)

	// User management
	protected.HandleFunc("/users", api.createUserHandler).Methods("POST")
	protected.HandleFunc("/users", api.listUsersHandler).Methods("GET")
	protected.HandleFunc("/users/{id}", api.getUserHandler).Methods("GET")
	protected.HandleFunc("/users/{id}", api.updateUserHandler).Methods("PUT")
	protected.HandleFunc("/users/{id}", api.deleteUserHandler).Methods("DELETE")

	// API Key management
	protected.HandleFunc("/api-keys", api.createAPIKeyHandler).Methods("POST")
	protected.HandleFunc("/api-keys", api.listAPIKeysHandler).Methods("GET")
	protected.HandleFunc("/api-keys/{id}", api.revokeAPIKeyHandler).Methods("DELETE")

	// Token management
	protected.HandleFunc("/tokens/refresh", api.refreshTokenHandler).Methods("POST")
	protected.HandleFunc("/tokens/validate", api.validateTokenHandler).Methods("POST")

	// Audit logs
	protected.HandleFunc("/audit", api.getAuditLogsHandler).Methods("GET")

	// Permissions
	protected.HandleFunc("/permissions/check", api.checkPermissionHandler).Methods("POST")
}

// GetRouter returns the HTTP router
func (api *AuthAPI) GetRouter() *mux.Router {
	return api.router
}

// Login request/response structures
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      *User     `json:"user"`
}

type CreateUserRequest struct {
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Password string   `json:"password,omitempty"`
}

type CreateAPIKeyRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type CreateAPIKeyResponse struct {
	APIKey *APIKey `json:"api_key"`
	Key    string  `json:"key"` // Only returned once during creation
}

type CheckPermissionRequest struct {
	Permission string `json:"permission"`
}

type CheckPermissionResponse struct {
	HasPermission bool     `json:"has_permission"`
	UserRoles     []string `json:"user_roles,omitempty"`
	UserPerms     []string `json:"user_permissions,omitempty"`
}

// HTTP Handlers

func (api *AuthAPI) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := parseJSONRequest(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	// For demo purposes, create/find user by username
	// In production, implement proper password validation
	var user *User
	var err error

	// Try to find existing user
	for _, u := range api.auth.users {
		if u.Username == req.Username || u.Email == req.Email {
			user = u
			break
		}
	}

	// If user doesn't exist, create new one (demo mode)
	if user == nil {
		email := req.Email
		if email == "" {
			email = req.Username + "@portask.local"
		}
		user, err = api.auth.CreateUser(req.Username, email, []string{RoleConsumer})
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to create user")
			return
		}
	}

	// Generate JWT token
	token, err := api.auth.GenerateJWT(user)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(api.auth.config.JWTExpiration),
		User:      user,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func (api *AuthAPI) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"services": map[string]string{
			"authentication": "up",
			"audit_logging":  "up",
		},
	}
	writeJSONResponse(w, http.StatusOK, health)
}

func (api *AuthAPI) createUserHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	if !api.auth.HasPermission(authCtx, PermissionAdminWrite) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req CreateUserRequest
	if err := parseJSONRequest(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	user, err := api.auth.CreateUser(req.Username, req.Email, req.Roles)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	writeJSONResponse(w, http.StatusCreated, user)
}

func (api *AuthAPI) listUsersHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	if !api.auth.HasPermission(authCtx, PermissionAdminRead) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	users := make([]*User, 0, len(api.auth.users))
	for _, user := range api.auth.users {
		users = append(users, user)
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

func (api *AuthAPI) getUserHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	vars := mux.Vars(r)
	userID := vars["id"]

	// Users can view their own profile, admins can view any profile
	if authCtx.User.ID != userID && !api.auth.HasPermission(authCtx, PermissionAdminRead) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	user, err := api.auth.GetUser(userID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSONResponse(w, http.StatusOK, user)
}

func (api *AuthAPI) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	vars := mux.Vars(r)
	userID := vars["id"]

	// Users can update their own profile, admins can update any profile
	if authCtx.User.ID != userID && !api.auth.HasPermission(authCtx, PermissionAdminWrite) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var updates map[string]interface{}
	if err := parseJSONRequest(r, &updates); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	user, err := api.auth.UpdateUser(userID, updates)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSONResponse(w, http.StatusOK, user)
}

func (api *AuthAPI) deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	if !api.auth.HasPermission(authCtx, PermissionAdminWrite) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]

	err := api.auth.DeleteUser(userID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

func (api *AuthAPI) createAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)

	var req CreateAPIKeyRequest
	if err := parseJSONRequest(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Verify user has permissions they're trying to grant
	for _, perm := range req.Permissions {
		if !api.auth.HasPermission(authCtx, perm) {
			writeErrorResponse(w, http.StatusForbidden, fmt.Sprintf("Cannot grant permission: %s", perm))
			return
		}
	}

	apiKey, err := api.auth.GenerateAPIKey(authCtx.User.ID, req.Name, req.Description, req.Permissions, req.ExpiresAt)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}

	response := CreateAPIKeyResponse{
		APIKey: apiKey,
		Key:    apiKey.Key, // Return the key only during creation
	}

	// Remove the key from the response object for security
	response.APIKey.Key = "***"

	writeJSONResponse(w, http.StatusCreated, response)
}

func (api *AuthAPI) listAPIKeysHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)

	var apiKeys []*APIKey
	for _, key := range api.auth.apiKeys {
		// Users can only see their own API keys, admins can see all
		if key.UserID == authCtx.User.ID || api.auth.HasPermission(authCtx, PermissionAdminRead) {
			// Don't include the actual key value
			safeCopy := *key
			safeCopy.Key = "***"
			apiKeys = append(apiKeys, &safeCopy)
		}
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"api_keys": apiKeys,
		"total":    len(apiKeys),
	})
}

func (api *AuthAPI) revokeAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	vars := mux.Vars(r)
	keyID := vars["id"]

	// Find the API key to check ownership
	var targetKey *APIKey
	for _, key := range api.auth.apiKeys {
		if key.ID == keyID {
			targetKey = key
			break
		}
	}

	if targetKey == nil {
		writeErrorResponse(w, http.StatusNotFound, "API key not found")
		return
	}

	// Users can revoke their own keys, admins can revoke any key
	if targetKey.UserID != authCtx.User.ID && !api.auth.HasPermission(authCtx, PermissionAdminWrite) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	err := api.auth.RevokeAPIKey(keyID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "API key not found")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "API key revoked successfully"})
}

func (api *AuthAPI) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)

	// Generate new token
	token, err := api.auth.GenerateJWT(authCtx.User)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	response := LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(api.auth.config.JWTExpiration),
		User:      authCtx.User,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func (api *AuthAPI) validateTokenHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)

	response := map[string]interface{}{
		"valid":       true,
		"user":        authCtx.User,
		"permissions": authCtx.Permissions,
		"auth_method": authCtx.AuthMethod,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func (api *AuthAPI) getAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)
	if !api.auth.HasPermission(authCtx, PermissionAdminRead) {
		writeErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	// Parse query parameters for filtering
	filters := make(map[string]interface{})
	query := r.URL.Query()

	if userID := query.Get("user_id"); userID != "" {
		filters["user_id"] = userID
	}
	if eventType := query.Get("event_type"); eventType != "" {
		filters["event_type"] = eventType
	}
	if resource := query.Get("resource"); resource != "" {
		filters["resource"] = resource
	}
	if action := query.Get("action"); action != "" {
		filters["action"] = action
	}

	limit := 100 // Default limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := parseIntParam(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	events, err := api.auth.auditLogger.GetEvents(filters, limit)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve audit logs")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  len(events),
		"limit":  limit,
	})
}

func (api *AuthAPI) checkPermissionHandler(w http.ResponseWriter, r *http.Request) {
	authCtx := getAuthContext(r)

	var req CheckPermissionRequest
	if err := parseJSONRequest(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	hasPermission := api.auth.HasPermission(authCtx, req.Permission)

	response := CheckPermissionResponse{
		HasPermission: hasPermission,
		UserRoles:     authCtx.User.Roles,
		UserPerms:     authCtx.Permissions,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// Middleware

func (api *AuthAPI) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if api.auth.config.EnableRateLimit {
			clientIP := getClientIP(r)
			if !api.rateLimiter.Allow(clientIP) {
				writeErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// TLS Configuration

// GetTLSConfig returns TLS configuration for secure connections
func GetTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

// Helper functions

func getAuthContext(r *http.Request) *AuthContext {
	if authCtx, ok := r.Context().Value("auth").(*AuthContext); ok {
		return authCtx
	}
	return nil
}

func parseJSONRequest(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(target)
}

func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeErrorResponse(w http.ResponseWriter, status int, message string) {
	error := map[string]interface{}{
		"error":     message,
		"status":    status,
		"timestamp": time.Now(),
	}
	writeJSONResponse(w, status, error)
}

func parseIntParam(s string) (int, error) {
	// Simple integer parsing - in production use strconv.Atoi
	var result int
	for _, char := range s {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		result = result*10 + int(char-'0')
	}
	return result, nil
}
