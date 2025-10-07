# 🔐 Portask Security Guide

## Table of Contents

1. [Authentication](#authentication)
2. [Authorization](#authorization)
3. [Rate Limiting](#rate-limiting)
4. [TLS/SSL Configuration](#tlsssl-configuration)
5. [Security Headers](#security-headers)
6. [Best Practices](#best-practices)
7. [Security Checklist](#security-checklist)

---

## Authentication

Portask supports multiple authentication methods:

### JWT Authentication

```go
// Setup JWT authentication
authConfig := &auth.AuthConfig{
    JWTSecret:     os.Getenv("JWT_SECRET"),
    JWTExpiration: 24 * time.Hour,
}

authenticator := auth.NewAuthenticator(authConfig, nil)
authMiddleware := auth.NewAuthMiddleware(authenticator, authConfig)

// Protect routes
app.Use(authMiddleware.FiberAuth())
```

**Login Example:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}'
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-10-08T20:00:00Z",
  "user": {
    "id": "user-123",
    "username": "admin",
    "roles": ["admin"]
  }
}
```

### API Key Authentication

```go
// Use API key auth
apiKeyMiddleware := authMiddleware.APIKeyAuth()
app.Use(apiKeyMiddleware)
```

**Usage:**

```bash
# In header
curl -H "X-API-Key: your-api-key-here" http://localhost:8080/api/v1/messages

# In query string
curl "http://localhost:8080/api/v1/messages?api_key=your-api-key-here"
```

### Creating API Keys

```go
apiKey, err := authenticator.GenerateAPIKey(
    "user-id",
    "production-key",
    "Production API Key",
    []string{"read", "write"},
    nil, // never expires
)
```

---

## Authorization

### Role-Based Access Control (RBAC)

```go
// Require specific role
app.Get("/admin/users",
    authMiddleware.FiberAuth(),
    authMiddleware.RequireRole("admin"),
    handleUsers,
)

// Multiple roles
app.Get("/api/data",
    authMiddleware.FiberAuth(),
    authMiddleware.RequireRole("admin", "user"),
    handleData,
)
```

### Built-in Roles

- `admin` - Full system access
- `producer` - Can publish messages
- `consumer` - Can consume messages
- `readonly` - Read-only access
- `service` - Service-to-service communication

### Custom Permissions

```go
user, _ := authenticator.CreateUser("john", "john@example.com", []string{"user"})

// Check permissions
if authenticator.HasPermission(authCtx, "message:write") {
    // Allow action
}
```

---

## Rate Limiting

### Configuration

```go
// IP-based rate limiting
rateLimiter := auth.RateLimitByIP(100) // 100 req/sec
app.Use(rateLimiter.Middleware())

// User-based rate limiting
userLimiter := auth.RateLimitByUser(1000) // 1000 req/sec
app.Use(userLimiter.Middleware())

// Endpoint-specific limiting
apiLimiter := auth.RateLimitByEndpoint(50) // 50 req/sec
app.Use("/api/messages", apiLimiter.Middleware())
```

### Custom Configuration

```go
config := &auth.RateLimitConfig{
    RequestsPerSecond: 100,
    BurstSize:         200,
    Window:            time.Minute,
    Message:           "Rate limit exceeded",
    StatusCode:        429,
}
rateLimiter := auth.NewRateLimiterMiddleware(config)
```

### Response Headers

Rate limit info is included in response headers:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
Retry-After: 60
```

---

## TLS/SSL Configuration

### Basic TLS

```yaml
# config.yaml
network:
  tls:
    enabled: true
    cert_file: "/etc/portask/tls/server.crt"
    key_file: "/etc/portask/tls/server.key"
    min_version: "TLS13"
```

### Mutual TLS (mTLS)

```yaml
network:
  tls:
    enabled: true
    cert_file: "/etc/portask/tls/server.crt"
    key_file: "/etc/portask/tls/server.key"
    ca_file: "/etc/portask/tls/ca.crt"
    client_auth: "require"
    min_version: "TLS13"
```

### Generate Self-Signed Certificates (Testing Only)

```bash
# Server certificate
openssl req -x509 -newkey rsa:4096 \
  -keyout server.key -out server.crt \
  -days 365 -nodes \
  -subj "/CN=localhost"

# Client certificate
openssl req -x509 -newkey rsa:4096 \
  -keyout client.key -out client.crt \
  -days 365 -nodes \
  -subj "/CN=client"
```

### Production Certificates

Use Let's Encrypt or your organization's PKI:

```bash
# Let's Encrypt with certbot
certbot certonly --standalone -d yourdomain.com
```

---

## Security Headers

### Default Security Headers

```go
security := api.NewSecurityMiddleware(api.DefaultSecurityConfig())
app.Use(security.Middleware())
```

**Headers Added:**

- `Content-Security-Policy`: Prevents XSS attacks
- `X-Frame-Options`: Prevents clickjacking
- `X-Content-Type-Options`: Prevents MIME sniffing
- `Strict-Transport-Security`: Enforces HTTPS
- `X-XSS-Protection`: Legacy XSS protection
- `Referrer-Policy`: Controls referrer information
- `Permissions-Policy`: Controls browser features

### Production Configuration

```go
security := api.NewSecurityMiddleware(api.ProductionSecurityConfig())
app.Use(security.Middleware())
```

### Custom Configuration

```go
config := &api.SecurityConfig{
    ContentSecurityPolicy: "default-src 'self'",
    XFrameOptions: "SAMEORIGIN",
    HSTSMaxAge: 31536000,
    CustomHeaders: map[string]string{
        "X-Custom-Header": "value",
    },
}
security := api.NewSecurityMiddleware(config)
```

---

## Best Practices

### 1. Use Strong Secrets

```bash
# Generate secure JWT secret
openssl rand -base64 64

# Use in environment
export JWT_SECRET="your-generated-secret-here"
```

### 2. Implement Password Hashing

```go
// TODO: Implement with bcrypt or argon2
import "golang.org/x/crypto/bcrypt"

hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), 14)
```

### 3. Rotate API Keys

```go
// Set expiration on API keys
expiresAt := time.Now().Add(90 * 24 * time.Hour) // 90 days
apiKey, _ := authenticator.GenerateAPIKey(
    userID,
    "quarterly-key",
    "Quarterly API Key",
    permissions,
    &expiresAt,
)
```

### 4. Enable Audit Logging

```go
authConfig := &auth.AuthConfig{
    EnableAuditLog: true,
    // ... other config
}
```

### 5. Use TLS in Production

```yaml
network:
  tls:
    enabled: true
    min_version: "TLS13"
```

### 6. Implement IP Whitelisting

```go
// Custom middleware
func IPWhitelist(allowedIPs []string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        clientIP := c.IP()
        for _, ip := range allowedIPs {
            if ip == clientIP {
                return c.Next()
            }
        }
        return c.Status(403).SendString("Forbidden")
    }
}
```

### 7. Monitor Security Events

```go
// Log authentication failures
if err := authenticator.ValidateToken(token); err != nil {
    log.Warn().
        Str("ip", clientIP).
        Str("error", err.Error()).
        Msg("Authentication failed")
}
```

---

## Security Checklist

### Development

- [ ] Use strong, random JWT secrets
- [ ] Implement rate limiting
- [ ] Add security headers
- [ ] Enable CORS with specific origins
- [ ] Use HTTPS for all connections
- [ ] Implement input validation
- [ ] Enable audit logging

### Production

- [ ] Use TLS 1.3 minimum
- [ ] Implement mutual TLS (mTLS)
- [ ] Use database-backed user store
- [ ] Rotate secrets regularly
- [ ] Set up monitoring and alerting
- [ ] Implement IP whitelisting
- [ ] Use secure session storage
- [ ] Enable HSTS with preload
- [ ] Implement password hashing
- [ ] Set up automated security scanning
- [ ] Configure firewall rules
- [ ] Implement backup and disaster recovery
- [ ] Set up intrusion detection
- [ ] Conduct regular security audits
- [ ] Keep dependencies updated

### Compliance

- [ ] GDPR compliance (if applicable)
- [ ] HIPAA compliance (if applicable)
- [ ] PCI-DSS compliance (if handling payments)
- [ ] SOC 2 compliance (if required)
- [ ] Document security policies
- [ ] Implement data retention policies
- [ ] Set up incident response plan
- [ ] Conduct penetration testing

---

## Security Resources

### Testing Tools

- **OWASP ZAP**: Web application security scanner
- **Nmap**: Network security scanner
- **sqlmap**: SQL injection testing
- **Burp Suite**: Web vulnerability scanner

### Monitoring

- **Prometheus**: Metrics collection
- **Grafana**: Metrics visualization
- **ELK Stack**: Log aggregation and analysis
- **Wazuh**: Security monitoring

### Compliance

- **OWASP Top 10**: Web application security risks
- **CIS Benchmarks**: Security configuration guidelines
- **NIST Cybersecurity Framework**: Security standards

---

## Support

For security issues, please email: security@portask.io

**DO NOT** create public GitHub issues for security vulnerabilities.

---

**Last Updated:** October 7, 2025  
**Version:** 1.0.0
