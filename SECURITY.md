# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

**Please DO NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to: **security@portask.dev** (or create a private security advisory on GitHub)

### What to Include

When reporting a vulnerability, please include:

1. **Description of the vulnerability**

   - Type of issue (e.g., buffer overflow, SQL injection, cross-site scripting)
   - Full paths of source file(s) related to the manifestation of the issue
   - The location of the affected source code (tag/branch/commit or direct URL)

2. **Step-by-step instructions to reproduce**

   - Proof-of-concept or exploit code (if possible)
   - Impact of the issue, including how an attacker might exploit it

3. **Your contact information**
   - Name (or alias)
   - Email address
   - Any other way we can contact you

### What to Expect

- **Acknowledgment**: We will acknowledge your email within 48 hours
- **Investigation**: We will investigate and confirm the vulnerability
- **Timeline**: We will provide an estimated timeline for a fix
- **Credit**: We will credit you for the discovery (unless you prefer to remain anonymous)
- **Disclosure**: We will coordinate with you on public disclosure timing

## Security Best Practices

When using Portask in production:

### 1. Network Security

```yaml
# Use strong authentication
auth:
  enabled: true
  jwt_secret: "use-a-strong-random-secret-here"

# Enable TLS for all connections
network:
  tls_enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
```

### 2. Access Control

- Use strong passwords for Redis/Dragonfly connections
- Implement network-level access controls (firewalls, security groups)
- Run Portask as a non-root user
- Limit connection pool sizes to prevent resource exhaustion

### 3. Input Validation

- Validate message sizes (set max_message_size)
- Sanitize topic/queue names
- Implement rate limiting for producers

### 4. Monitoring

- Enable audit logging
- Monitor for unusual patterns
- Set up alerts for error spikes
- Track failed authentication attempts

### 5. Updates

- Keep Portask up to date with the latest security patches
- Subscribe to security announcements
- Review CHANGELOG.md for security fixes

## Known Security Considerations

### 1. Message Size Limits

Default max message size is 10MB. Adjust based on your threat model:

```yaml
performance:
  max_message_size: 10485760 # 10MB
```

### 2. Connection Limits

Prevent resource exhaustion with connection limits:

```yaml
network:
  max_connections: 10000
  connection_timeout: "30s"
```

### 3. Storage Security

When using persistent storage:

```yaml
storage:
  type: "dragonfly"
  dragonfly:
    password: "strong-password-here"
    tls_enabled: true
```

### 4. Admin UI Access

The admin UI should be protected:

```yaml
api:
  admin_enabled: true
  admin_auth_required: true
  admin_username: "admin"
  admin_password: "hashed-password"
```

## Security Scanning

We use automated security scanning:

- **GoSec**: Static analysis security scanner
- **Dependabot**: Dependency vulnerability scanning
- **Trivy**: Container image scanning
- **CodeQL**: Semantic code analysis

Run security checks locally:

```bash
# Run GoSec
make security

# Scan dependencies
go list -json -m all | nancy sleuth

# Scan Docker image
trivy image portask:latest
```

## Compliance

Portask aims to be compliant with:

- OWASP Top 10
- CWE Top 25
- GDPR (data handling guidelines)

## Security Headers

When deploying with a reverse proxy, use security headers:

```nginx
# Nginx example
add_header X-Frame-Options "SAMEORIGIN";
add_header X-Content-Type-Options "nosniff";
add_header X-XSS-Protection "1; mode=block";
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";
```

## Incident Response

In case of a security incident:

1. **Isolate**: Disconnect affected systems
2. **Assess**: Determine the scope and impact
3. **Contain**: Stop the attack/vulnerability
4. **Remediate**: Apply fixes and patches
5. **Monitor**: Watch for continued activity
6. **Document**: Record timeline and actions
7. **Disclose**: Coordinate public disclosure

## Contact

For security concerns:

- Email: security@portask.dev
- GitHub Security Advisories: [Create Advisory](https://github.com/meftunca/portask/security/advisories/new)

---

Thank you for helping keep Portask secure! 🔒
