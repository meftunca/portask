package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// SecurityMiddleware provides security headers
type SecurityMiddleware struct {
	config *SecurityConfig
}

// SecurityConfig holds security middleware configuration
type SecurityConfig struct {
	// Content Security Policy
	ContentSecurityPolicy string `json:"content_security_policy"`
	
	// X-Frame-Options
	XFrameOptions string `json:"x_frame_options"`
	
	// X-Content-Type-Options
	XContentTypeOptions string `json:"x_content_type_options"`
	
	// Strict-Transport-Security
	HSTSMaxAge            int  `json:"hsts_max_age"`
	HSTSIncludeSubdomains bool `json:"hsts_include_subdomains"`
	HSTSPreload           bool `json:"hsts_preload"`
	
	// X-XSS-Protection
	XSSProtection string `json:"xss_protection"`
	
	// Referrer-Policy
	ReferrerPolicy string `json:"referrer_policy"`
	
	// Permissions-Policy
	PermissionsPolicy string `json:"permissions_policy"`
	
	// Custom headers
	CustomHeaders map[string]string `json:"custom_headers"`
}

// DefaultSecurityConfig returns secure default configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'",
		XFrameOptions:         "DENY",
		XContentTypeOptions:   "nosniff",
		HSTSMaxAge:            31536000, // 1 year
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
		XSSProtection:         "1; mode=block",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
		CustomHeaders:         make(map[string]string),
	}
}

// ProductionSecurityConfig returns production-ready security configuration
func ProductionSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		ContentSecurityPolicy: "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
		XFrameOptions:         "DENY",
		XContentTypeOptions:   "nosniff",
		HSTSMaxAge:            63072000, // 2 years
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
		XSSProtection:         "1; mode=block",
		ReferrerPolicy:        "no-referrer",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=()",
		CustomHeaders: map[string]string{
			"X-Powered-By": "", // Remove X-Powered-By header
		},
	}
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config *SecurityConfig) *SecurityMiddleware {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	return &SecurityMiddleware{
		config: config,
	}
}

// Middleware returns a Fiber middleware handler
func (m *SecurityMiddleware) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Content Security Policy
		if m.config.ContentSecurityPolicy != "" {
			c.Set("Content-Security-Policy", m.config.ContentSecurityPolicy)
		}
		
		// X-Frame-Options
		if m.config.XFrameOptions != "" {
			c.Set("X-Frame-Options", m.config.XFrameOptions)
		}
		
		// X-Content-Type-Options
		if m.config.XContentTypeOptions != "" {
			c.Set("X-Content-Type-Options", m.config.XContentTypeOptions)
		}
		
		// Strict-Transport-Security (HSTS)
		if m.config.HSTSMaxAge > 0 {
			hstsValue := fmt.Sprintf("max-age=%d", m.config.HSTSMaxAge)
			if m.config.HSTSIncludeSubdomains {
				hstsValue += "; includeSubDomains"
			}
			if m.config.HSTSPreload {
				hstsValue += "; preload"
			}
			c.Set("Strict-Transport-Security", hstsValue)
		}
		
		// X-XSS-Protection
		if m.config.XSSProtection != "" {
			c.Set("X-XSS-Protection", m.config.XSSProtection)
		}
		
		// Referrer-Policy
		if m.config.ReferrerPolicy != "" {
			c.Set("Referrer-Policy", m.config.ReferrerPolicy)
		}
		
		// Permissions-Policy
		if m.config.PermissionsPolicy != "" {
			c.Set("Permissions-Policy", m.config.PermissionsPolicy)
		}
		
		// Custom headers
		for key, value := range m.config.CustomHeaders {
			if value == "" {
				// Remove header if value is empty
				c.Response().Header.Del(key)
			} else {
				c.Set(key, value)
			}
		}
		
		// Continue to next handler
		return c.Next()
	}
}

// CORS middleware with secure defaults
type CORSConfig struct {
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	ExposeHeaders    []string `json:"expose_headers"`
	MaxAge           int      `json:"max_age"`
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"},
		AllowCredentials: false,
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		MaxAge:           3600,
	}
}

// ProductionCORSConfig returns production CORS configuration
func ProductionCORSConfig(allowedOrigins []string) *CORSConfig {
	return &CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key", "X-Request-ID"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		MaxAge:           7200,
	}
}

// SecurityMiddlewareExample provides usage examples
func SecurityMiddlewareExample() string {
	return `
# Security Middleware Usage Examples:

## 1. Basic Usage (Development)
security := api.NewSecurityMiddleware(api.DefaultSecurityConfig())
app.Use(security.Middleware())

## 2. Production Configuration
security := api.NewSecurityMiddleware(api.ProductionSecurityConfig())
app.Use(security.Middleware())

## 3. Custom Configuration
config := &api.SecurityConfig{
    ContentSecurityPolicy: "default-src 'self'",
    XFrameOptions: "SAMEORIGIN",
    HSTSMaxAge: 31536000,
    CustomHeaders: map[string]string{
        "X-Custom-Header": "value",
        "X-Powered-By": "", // Remove header
    },
}
security := api.NewSecurityMiddleware(config)
app.Use(security.Middleware())

## 4. With CORS
cors := api.ProductionCORSConfig([]string{
    "https://example.com",
    "https://app.example.com",
})
// Use with fiber/cors middleware

## Security Headers Added:
- Content-Security-Policy
- X-Frame-Options
- X-Content-Type-Options
- Strict-Transport-Security (HSTS)
- X-XSS-Protection
- Referrer-Policy
- Permissions-Policy

## Best Practices:
1. Use ProductionSecurityConfig() in production
2. Enable HSTS only with valid SSL/TLS
3. Adjust CSP based on your application needs
4. Test headers with securityheaders.com
5. Consider using report-uri for CSP violations
`
}

