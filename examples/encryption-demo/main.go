package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/meftunca/portask/pkg/auth"
)

func main() {
	fmt.Println("🔒 Portask Message Encryption Demo")
	fmt.Println("==================================")

	// Initialize encryption
	encConfig := auth.DefaultEncryptionConfig()
	encryption, err := auth.NewMessageEncryption(encConfig)
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}

	// Initialize message security provider
	securityProvider, err := auth.NewMessageSecurityProvider(encConfig, "signing-secret-key")
	if err != nil {
		log.Fatalf("Failed to initialize security provider: %v", err)
	}

	fmt.Println("1. Basic Encryption Demo")
	fmt.Println("------------------------")

	// Original message
	originalMessage := "Hello, this is a secret message from Portask!"
	fmt.Printf("Original message: %s\n", originalMessage)

	// Encrypt the message
	encryptedMsg, err := encryption.EncryptString(originalMessage)
	if err != nil {
		log.Fatalf("Failed to encrypt message: %v", err)
	}

	fmt.Printf("Encrypted message: %s\n", encryptedMsg.Ciphertext[:50]+"...")
	fmt.Printf("Nonce: %s\n", encryptedMsg.Nonce)
	fmt.Printf("Algorithm: %s\n", encryptedMsg.Algorithm)

	// Decrypt the message
	decryptedMessage, err := encryption.DecryptString(encryptedMsg)
	if err != nil {
		log.Fatalf("Failed to decrypt message: %v", err)
	}

	fmt.Printf("Decrypted message: %s\n", decryptedMessage)
	fmt.Printf("Messages match: %t\n\n", originalMessage == decryptedMessage)

	fmt.Println("2. Message Signing Demo")
	fmt.Println("-----------------------")

	// Initialize signer
	signer := auth.NewMessageSigner("signing-secret")

	// Sign a message
	messageToSign := "This message needs integrity verification"
	fmt.Printf("Original message: %s\n", messageToSign)

	signedMsg := signer.Sign([]byte(messageToSign))
	fmt.Printf("Signature: %s\n", signedMsg.Signature[:20]+"...")
	fmt.Printf("Timestamp: %d\n", signedMsg.Timestamp)

	// Verify the message
	verifiedPayload, err := signer.Verify(signedMsg)
	if err != nil {
		log.Fatalf("Failed to verify message: %v", err)
	}

	fmt.Printf("Verified message: %s\n", string(verifiedPayload))
	fmt.Printf("Verification successful: %t\n\n", string(verifiedPayload) == messageToSign)

	fmt.Println("3. Combined Security Demo")
	fmt.Println("-------------------------")

	// Message that needs both encryption and signing
	sensitiveMessage := "Top secret: Database password is super_secure_123!"
	fmt.Printf("Original sensitive message: %s\n", sensitiveMessage)

	// Secure the message (encrypt + sign)
	secureMsg, err := securityProvider.SecureMessage([]byte(sensitiveMessage))
	if err != nil {
		log.Fatalf("Failed to secure message: %v", err)
	}

	fmt.Printf("Secured message ciphertext: %s\n", secureMsg.EncryptedMessage.Ciphertext[:30]+"...")
	fmt.Printf("Secured message signature: %s\n", secureMsg.SignedMessage.Signature[:20]+"...")

	// Unsecure the message (verify + decrypt)
	recoveredMessage, err := securityProvider.UnsecureMessage(secureMsg)
	if err != nil {
		log.Fatalf("Failed to unsecure message: %v", err)
	}

	fmt.Printf("Recovered message: %s\n", string(recoveredMessage))
	fmt.Printf("Messages match: %t\n\n", string(recoveredMessage) == sensitiveMessage)

	fmt.Println("4. Authentication Demo")
	fmt.Println("----------------------")

	// Test authentication flow
	testAuthenticationFlow()

	fmt.Println("\n✅ All demos completed successfully!")
}

func testAuthenticationFlow() {
	// Create a test user
	auditLogger := auth.NewInMemoryAuditLogger()
	authConfig := auth.DefaultAuthConfig()
	authenticator := auth.NewAuthenticator(authConfig, auditLogger)

	// Create a test user
	user, err := authenticator.CreateUser("testuser", "test@example.com", []string{auth.RoleProducer})
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("Created user: %s (%s)\n", user.Username, user.Email)

	// Generate JWT token
	token, err := authenticator.GenerateJWT(user)
	if err != nil {
		log.Fatalf("Failed to generate JWT: %v", err)
	}

	fmt.Printf("Generated JWT token: %s\n", token[:30]+"...")

	// Validate JWT token
	claims, err := authenticator.ValidateJWT(token)
	if err != nil {
		log.Fatalf("Failed to validate JWT: %v", err)
	}

	fmt.Printf("JWT validation successful: %s\n", claims.Username)

	// Generate API key
	apiKey, err := authenticator.GenerateAPIKey(user.ID, "test-key", "Test API key",
		[]string{auth.PermissionMessageWrite}, nil)
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	fmt.Printf("Generated API key: %s\n", apiKey.Key[:16]+"...")

	// Validate API key
	validatedKey, err := authenticator.ValidateAPIKey(apiKey.Key)
	if err != nil {
		log.Fatalf("Failed to validate API key: %v", err)
	}

	fmt.Printf("API key validation successful: %s\n", validatedKey.Name)
}

// HTTP client demo
func demonstrateHTTPAuth() {
	fmt.Println("5. HTTP Authentication Demo")
	fmt.Println("---------------------------")

	// Login request
	loginData := map[string]string{
		"username": "admin",
		"email":    "admin@portask.local",
	}

	loginJSON, _ := json.Marshal(loginData)

	// Make login request
	resp, err := http.Post("http://localhost:8080/auth/login", "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		fmt.Printf("Login request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var loginResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&loginResp)
		fmt.Printf("Login successful! Token: %s\n", loginResp["token"].(string)[:30]+"...")

		// Use token for authenticated request
		token := loginResp["token"].(string)
		req, _ := http.NewRequest("GET", "http://localhost:8080/auth/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Authenticated request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Println("Authenticated request successful!")
		}
	} else {
		fmt.Printf("Login failed with status: %d\n", resp.StatusCode)
	}
}
