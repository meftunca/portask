package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// MessageEncryption provides end-to-end message encryption
type MessageEncryption struct {
	gcm cipher.AEAD
}

// EncryptionConfig holds encryption configuration
type EncryptionConfig struct {
	Algorithm string `yaml:"algorithm" json:"algorithm"`   // "aes-256-gcm"
	KeySource string `yaml:"key_source" json:"key_source"` // "static", "derived", "kms"
	StaticKey string `yaml:"static_key" json:"static_key"`
}

// DefaultEncryptionConfig returns default encryption configuration
func DefaultEncryptionConfig() *EncryptionConfig {
	return &EncryptionConfig{
		Algorithm: "aes-256-gcm",
		KeySource: "static",
		StaticKey: "portask-default-encryption-key-change-in-production-32bytes",
	}
}

// NewMessageEncryption creates a new message encryption instance
func NewMessageEncryption(config *EncryptionConfig) (*MessageEncryption, error) {
	if config == nil {
		config = DefaultEncryptionConfig()
	}

	// Derive 32-byte key from config
	key := deriveKey(config.StaticKey, 32)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &MessageEncryption{gcm: gcm}, nil
}

// EncryptedMessage represents an encrypted message with metadata
type EncryptedMessage struct {
	Ciphertext string            `json:"ciphertext"`
	Nonce      string            `json:"nonce"`
	Algorithm  string            `json:"algorithm"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Encrypt encrypts a message payload
func (me *MessageEncryption) Encrypt(plaintext []byte) (*EncryptedMessage, error) {
	// Generate random nonce
	nonce := make([]byte, me.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := me.gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedMessage{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Algorithm:  "aes-256-gcm",
	}, nil
}

// Decrypt decrypts an encrypted message
func (me *MessageEncryption) Decrypt(encMsg *EncryptedMessage) ([]byte, error) {
	// Decode ciphertext and nonce
	ciphertext, err := base64.StdEncoding.DecodeString(encMsg.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encMsg.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Verify algorithm
	if encMsg.Algorithm != "aes-256-gcm" {
		return nil, fmt.Errorf("unsupported encryption algorithm: %s", encMsg.Algorithm)
	}

	// Decrypt the ciphertext
	plaintext, err := me.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string message
func (me *MessageEncryption) EncryptString(plaintext string) (*EncryptedMessage, error) {
	return me.Encrypt([]byte(plaintext))
}

// DecryptString decrypts to a string message
func (me *MessageEncryption) DecryptString(encMsg *EncryptedMessage) (string, error) {
	plaintext, err := me.Decrypt(encMsg)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// MessageSigner provides message signing functionality
type MessageSigner struct {
	secretKey []byte
}

// NewMessageSigner creates a new message signer
func NewMessageSigner(secret string) *MessageSigner {
	key := deriveKey(secret, 32)
	return &MessageSigner{secretKey: key}
}

// SignedMessage represents a signed message
type SignedMessage struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
}

// Sign creates a signature for a message
func (ms *MessageSigner) Sign(payload []byte) *SignedMessage {
	timestamp := time.Now().Unix()

	// Create message to sign (payload + timestamp)
	message := fmt.Sprintf("%s:%d", string(payload), timestamp)

	// Create HMAC signature
	h := hmac.New(sha256.New, ms.secretKey)
	h.Write([]byte(message))
	signature := hex.EncodeToString(h.Sum(nil))

	return &SignedMessage{
		Payload:   base64.StdEncoding.EncodeToString(payload),
		Signature: signature,
		Timestamp: timestamp,
	}
}

// Verify verifies a signed message
func (ms *MessageSigner) Verify(signedMsg *SignedMessage) ([]byte, error) {
	// Decode payload
	payload, err := base64.StdEncoding.DecodeString(signedMsg.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	// Recreate message to verify
	message := fmt.Sprintf("%s:%d", string(payload), signedMsg.Timestamp)

	// Create expected signature
	h := hmac.New(sha256.New, ms.secretKey)
	h.Write([]byte(message))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Verify signature
	if !hmac.Equal([]byte(expectedSignature), []byte(signedMsg.Signature)) {
		return nil, fmt.Errorf("invalid signature")
	}

	return payload, nil
}

// EncryptedAndSignedMessage combines encryption and signing
type EncryptedAndSignedMessage struct {
	EncryptedMessage *EncryptedMessage `json:"encrypted_message"`
	SignedMessage    *SignedMessage    `json:"signed_message"`
}

// MessageSecurityProvider combines encryption and signing
type MessageSecurityProvider struct {
	encryption *MessageEncryption
	signer     *MessageSigner
}

// NewMessageSecurityProvider creates a new message security provider
func NewMessageSecurityProvider(encConfig *EncryptionConfig, signingSecret string) (*MessageSecurityProvider, error) {
	encryption, err := NewMessageEncryption(encConfig)
	if err != nil {
		return nil, err
	}

	signer := NewMessageSigner(signingSecret)

	return &MessageSecurityProvider{
		encryption: encryption,
		signer:     signer,
	}, nil
}

// SecureMessage encrypts and signs a message
func (msp *MessageSecurityProvider) SecureMessage(plaintext []byte) (*EncryptedAndSignedMessage, error) {
	// First encrypt the message
	encryptedMsg, err := msp.encryption.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	// Then sign the encrypted message
	encryptedBytes, _ := json.Marshal(encryptedMsg)
	signedMsg := msp.signer.Sign(encryptedBytes)

	return &EncryptedAndSignedMessage{
		EncryptedMessage: encryptedMsg,
		SignedMessage:    signedMsg,
	}, nil
}

// UnsecureMessage verifies and decrypts a message
func (msp *MessageSecurityProvider) UnsecureMessage(secureMsg *EncryptedAndSignedMessage) ([]byte, error) {
	// First verify the signature
	encryptedBytes, err := msp.signer.Verify(secureMsg.SignedMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	// Unmarshal the encrypted message
	var encryptedMsg EncryptedMessage
	if err := json.Unmarshal(encryptedBytes, &encryptedMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal encrypted message: %w", err)
	}

	// Then decrypt the message
	plaintext, err := msp.encryption.Decrypt(&encryptedMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	return plaintext, nil
}

// Helper functions

// deriveKey derives a key of specified length from a password using SHA-256
func deriveKey(password string, keyLength int) []byte {
	hash := sha256.Sum256([]byte(password))
	if keyLength <= 32 {
		return hash[:keyLength]
	}

	// For longer keys, concatenate multiple hashes
	key := make([]byte, keyLength)
	copy(key, hash[:])

	for i := 32; i < keyLength; i += 32 {
		hash = sha256.Sum256(hash[:])
		remaining := keyLength - i
		if remaining > 32 {
			remaining = 32
		}
		copy(key[i:], hash[:remaining])
	}

	return key
}
