// Package cryptox provides shared encryption helpers for secrets at rest.
//
// The package expects high-entropy master keys. Do not pass user passwords
// directly; derive password-based keys with a dedicated password KDF first.
package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	// minMasterKeySize requires at least 256 bits of master key material for AES-256 key derivation.
	minMasterKeySize = 32
	// minSaltSize requires at least 128 bits of independent salt for HKDF-Extract.
	minSaltSize = 16
	// derivedKeySize is the AES-256 key size produced by HKDF-SHA256.
	derivedKeySize = 32
	// ciphertextVersionV1 identifies the first ciphertext format for AES-256-GCM.
	ciphertextVersionV1 = "v1"
	// ciphertextPrefixV1 prefixes encoded ciphertexts so future formats can coexist safely.
	ciphertextPrefixV1 = ciphertextVersionV1 + ":"
)

var (
	// ErrInvalidConfig indicates the cipher was configured with invalid keying material or state.
	ErrInvalidConfig = errors.New("invalid cryptox config")
	// ErrInvalidCiphertext indicates ciphertext decoding or authentication failed.
	ErrInvalidCiphertext = errors.New("invalid cryptox ciphertext")
	// ErrUnsupportedVersion indicates the ciphertext version is not supported by this package.
	ErrUnsupportedVersion = errors.New("unsupported cryptox ciphertext version")
)

// Config defines the minimum settings required to create a Cipher.
type Config struct {
	// MasterKey is high-entropy key material used as HKDF input keying material.
	MasterKey []byte
	// Salt is independent, non-secret HKDF salt; use a stable random value per application domain.
	Salt []byte
	// InfoPrefix namespaces derived keys for one application purpose, such as "agent/secret/".
	InfoPrefix string
}

// Cipher encrypts and decrypts values with AES-256-GCM keys derived by HKDF-SHA256.
//
// A Cipher is immutable after NewCipher returns, so it is safe for concurrent use by
// multiple goroutines. Each call derives a fresh per-scope key and uses a freshly
// allocated nonce and output buffer, sharing no mutable state across calls.
type Cipher struct {
	prk        []byte
	infoPrefix string
	random     io.Reader
}

// NewCipher creates a reusable cipher from high-entropy master key material and salt.
//
// It follows RFC 5869 HKDF by extracting a pseudorandom key once during initialization
// and expanding per scope during encryption and decryption. AES is provided by Go's
// standard library implementation of FIPS 197, and GCM uses the standard 96-bit nonce
// size from cipher.NewGCM as recommended by NIST SP 800-38D.
func NewCipher(cfg Config) (*Cipher, error) {
	if len(cfg.MasterKey) < minMasterKeySize {
		return nil, fmt.Errorf("%w: master key must be at least %d bytes", ErrInvalidConfig, minMasterKeySize)
	}
	if len(cfg.Salt) < minSaltSize {
		return nil, fmt.Errorf("%w: salt must be at least %d bytes", ErrInvalidConfig, minSaltSize)
	}

	masterKey := append([]byte(nil), cfg.MasterKey...)
	defer zeroBytes(masterKey)
	salt := append([]byte(nil), cfg.Salt...)

	prk, err := extractHKDF(masterKey, salt)
	if err != nil {
		return nil, fmt.Errorf("%w: extract pseudorandom key: %v", ErrInvalidConfig, err)
	}

	return &Cipher{
		prk:        prk,
		infoPrefix: cfg.InfoPrefix,
		random:     rand.Reader,
	}, nil
}

// Encrypt encrypts plaintext bytes for scope and returns a versioned encoded ciphertext.
//
// scope is a variadic list of context parts that participate in key derivation; the same
// parts, in the same order, must be supplied to Decrypt. Parts are joined unambiguously
// inside the package, so callers pass identifiers directly without pre-joining them.
func (c *Cipher) Encrypt(plaintext []byte, scope ...string) (string, error) {
	key, err := c.deriveKey(scope)
	if err != nil {
		return "", err
	}
	defer zeroBytes(key)

	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(c.randomReader(), nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, plaintext, []byte(ciphertextVersionV1))
	payload := make([]byte, 0, len(nonce)+len(sealed))
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)

	return ciphertextPrefixV1 + base64.RawURLEncoding.EncodeToString(payload), nil
}

// Decrypt decrypts a versioned encoded ciphertext for scope.
//
// scope must match the parts passed to Encrypt exactly, in the same order; otherwise key
// derivation yields a different key and authentication fails.
func (c *Cipher) Decrypt(encodedCiphertext string, scope ...string) ([]byte, error) {
	if !strings.HasPrefix(encodedCiphertext, ciphertextPrefixV1) {
		return nil, fmt.Errorf("%w: expected %s prefix", ErrUnsupportedVersion, ciphertextPrefixV1)
	}

	payload := strings.TrimPrefix(encodedCiphertext, ciphertextPrefixV1)
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode: %v", ErrInvalidCiphertext, err)
	}

	key, err := c.deriveKey(scope)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrInvalidCiphertext)
	}

	nonce := data[:nonceSize]
	sealed := data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, []byte(ciphertextVersionV1))
	if err != nil {
		return nil, fmt.Errorf("%w: gcm open: %v", ErrInvalidCiphertext, err)
	}
	return plaintext, nil
}

// EncryptString encrypts plaintext for scope and returns a versioned encoded ciphertext.
func (c *Cipher) EncryptString(plaintext string, scope ...string) (string, error) {
	return c.Encrypt([]byte(plaintext), scope...)
}

// DecryptString decrypts a versioned encoded ciphertext for scope and returns plaintext as a string.
func (c *Cipher) DecryptString(encodedCiphertext string, scope ...string) (string, error) {
	plaintext, err := c.Decrypt(encodedCiphertext, scope...)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// buildScope joins scope parts into a stable, length-prefixed context string.
//
// The length prefix makes the encoding unambiguous: ("ab","c") and ("a","bc") produce
// different results, so distinct part lists can never collide onto the same derived key.
func buildScope(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(strconv.Itoa(len(part)))
		builder.WriteByte(':')
		builder.WriteString(part)
		builder.WriteByte(';')
	}
	return builder.String()
}

// deriveKey expands the initialized HKDF pseudorandom key into one AES-256 key for scope.
func (c *Cipher) deriveKey(scope []string) ([]byte, error) {
	if c == nil || len(c.prk) == 0 {
		return nil, fmt.Errorf("%w: cipher is not initialized", ErrInvalidConfig)
	}

	key, err := expandHKDF(c.prk, buildInfo(c.infoPrefix, scope), derivedKeySize)
	if err != nil {
		return nil, fmt.Errorf("%w: expand scoped key: %v", ErrInvalidConfig, err)
	}
	return key, nil
}

// randomReader returns the configured random source and falls back to crypto/rand.
func (c *Cipher) randomReader() io.Reader {
	if c == nil || c.random == nil {
		return rand.Reader
	}
	return c.random
}

// buildInfo creates a domain-separated HKDF info string for one cipher and scope.
//
// The fixed leading parts bind every derived key to this package, ciphertext version, and
// the cipher's InfoPrefix; the caller-supplied scope parts follow as additional context.
func buildInfo(infoPrefix string, scope []string) string {
	parts := make([]string, 0, len(scope)+3)
	parts = append(parts, "cryptox", ciphertextVersionV1, infoPrefix)
	parts = append(parts, scope...)
	return buildScope(parts...)
}

// extractHKDF extracts an RFC 5869 pseudorandom key with SHA-256.
func extractHKDF(masterKey, salt []byte) ([]byte, error) {
	return hkdf.Extract(sha256.New, masterKey, salt)
}

// expandHKDF expands an RFC 5869 pseudorandom key with SHA-256.
func expandHKDF(prk []byte, info string, keyLength int) ([]byte, error) {
	return hkdf.Expand(sha256.New, prk, info, keyLength)
}

// newGCM creates an AES-GCM AEAD from a derived AES key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: aes new cipher: %v", ErrInvalidConfig, err)
	}
	return newGCMFromBlock(block)
}

// newGCMFromBlock creates a GCM AEAD from a block cipher.
func newGCMFromBlock(block cipher.Block) (cipher.AEAD, error) {
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: new gcm: %v", ErrInvalidConfig, err)
	}
	return gcm, nil
}

// zeroBytes overwrites a byte slice in place after temporary secret use.
func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
