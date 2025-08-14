# auth/ (Authentication, Audit, Encryption, Rate Limiter)

This package provides authentication, audit logging, encryption, and rate limiting for the Portask system. It supports secure user management, API key and JWT handling, audit trails, and flexible encryption strategies.

## Main Files
- `api.go`: Auth API endpoints and HTTP handlers
- `audit.go`: Audit logging (in-memory and file-based)
- `authenticator.go`: Authentication logic, user/session management
- `encryption.go`: Encryption, signing, and message security
- `ratelimiter.go`: Rate limiting implementation

## Key Types, Interfaces, and Methods

### Interfaces
- `AuthenticatorInterface` — Methods for authentication (JWT, API keys, users)
- `AuditLogger` — Interface for audit logging

### Structs & Types
- `AuthAPI` — API handler for authentication endpoints
- `LoginRequest`, `LoginResponse`, `CreateUserRequest`, `CreateAPIKeyRequest`, `CreateAPIKeyResponse`, `CheckPermissionRequest`, `CheckPermissionResponse` — API request/response types
- `AuthConfig` — Configuration for authentication
- `User`, `APIKey`, `JWTClaims`, `AuthContext` — User and session types
- `Authenticator` — Main authentication logic
- `AuditEvent` — Audit event structure
- `InMemoryAuditLogger`, `FileAuditLogger` — Audit logger implementations
- `MessageEncryption`, `EncryptionConfig`, `EncryptedMessage`, `MessageSigner`, `SignedMessage`, `EncryptedAndSignedMessage`, `MessageSecurityProvider` — Encryption and signing
- `RateLimiter` — Rate limiting logic

### Main Methods (selected)
- `NewAuthAPI(auth *Authenticator) *AuthAPI`, `setupRoutes`, `GetRouter`, `loginHandler`, `createUserHandler`, `listUsersHandler`, `getUserHandler`, `updateUserHandler`, `deleteUserHandler`, `createAPIKeyHandler`, `listAPIKeysHandler`, `revokeAPIKeyHandler`, `refreshTokenHandler`, `validateTokenHandler`, `getAuditLogsHandler`, `checkPermissionHandler`, `rateLimitMiddleware` (AuthAPI)
- `NewInMemoryAuditLogger()`, `LogEvent`, `GetEvents` (InMemoryAuditLogger)
- `NewFileAuditLogger(filePath string)`, `LogEvent`, `GetEvents` (FileAuditLogger)
- `NewAuthenticator(config *AuthConfig, auditLogger AuditLogger) *Authenticator`, `GenerateJWT`, `ValidateJWT`, `GenerateAPIKey`, `ValidateAPIKey`, `RevokeAPIKey`, `CreateUser`, `GetUser`, `UpdateUser`, `DeleteUser`, `HasPermission`, `HasRole`, `AuthMiddleware` (Authenticator)
- `NewMessageEncryption(config *EncryptionConfig)`, `Encrypt`, `Decrypt`, `EncryptString`, `DecryptString` (MessageEncryption)
- `NewMessageSigner(secret string)`, `Sign`, `Verify` (MessageSigner)
- `NewMessageSecurityProvider(encConfig, signingSecret)`, `SecureMessage`, `UnsecureMessage` (MessageSecurityProvider)
- `NewRateLimiter(requestsPerSecond int) *RateLimiter`, `Allow`, `Stop` (RateLimiter)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/auth"

auth := auth.NewAuthenticator(auth.DefaultAuthConfig(), nil)
jwt, err := auth.GenerateJWT(&auth.User{Username: "demo"})
```

## TODO / Missing Features
- [ ] Advanced audit trail (external storage, search)
- [ ] Token/session management improvements
- [ ] Extensible encryption algorithms
- [ ] Multi-factor authentication
- [ ] Rate limiter persistence

---

For details, see the code and comments in each file.
