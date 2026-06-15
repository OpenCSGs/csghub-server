package cryptox

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var (
	testMasterKey = []byte("0123456789abcdef0123456789abcdef")
	testSalt      = []byte("0123456789abcdef")
)

type failingReader struct{}

type wrongBlockSize struct{}

// BlockSize returns a non-AES block size to force cipher.NewGCM validation failure.
func (w wrongBlockSize) BlockSize() int {
	return 8
}

// Encrypt is a no-op block method used only to satisfy cipher.Block in tests.
func (w wrongBlockSize) Encrypt(dst, src []byte) {
	copy(dst, src)
}

// Decrypt is a no-op block method used only to satisfy cipher.Block in tests.
func (w wrongBlockSize) Decrypt(dst, src []byte) {
	copy(dst, src)
}

// Read always fails so tests can verify nonce generation error handling.
func (f failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("random failed")
}

// TestNewCipherValidatesConfig verifies weak or missing keying material is rejected.
func TestNewCipherValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing master key",
			cfg: Config{
				Salt: testSalt,
			},
		},
		{
			name: "short master key",
			cfg: Config{
				MasterKey: []byte("short"),
				Salt:      testSalt,
			},
		},
		{
			name: "missing salt",
			cfg: Config{
				MasterKey: testMasterKey,
			},
		},
		{
			name: "short salt",
			cfg: Config{
				MasterKey: testMasterKey,
				Salt:      []byte("short"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cipher, err := NewCipher(tt.cfg)
			if err == nil {
				t.Fatal("expected config error")
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected ErrInvalidConfig, got %v", err)
			}
			if cipher != nil {
				t.Fatalf("expected nil cipher, got %#v", cipher)
			}
		})
	}
}

// TestCipherEncryptDecryptString verifies the string helper round-trips versioned ciphertext.
func TestCipherEncryptDecryptString(t *testing.T) {
	cipher := newTestCipher(t, "credential/token/")

	ciphertext, err := cipher.EncryptString("secret-token", "user-1")
	if err != nil {
		t.Fatalf("encrypt string: %v", err)
	}
	if !strings.HasPrefix(ciphertext, "v1:") {
		t.Fatalf("expected v1 ciphertext prefix, got %q", ciphertext)
	}
	if strings.Contains(ciphertext, "secret-token") {
		t.Fatalf("ciphertext contains plaintext: %q", ciphertext)
	}

	plaintext, err := cipher.DecryptString(ciphertext, "user-1")
	if err != nil {
		t.Fatalf("decrypt string: %v", err)
	}
	if plaintext != "secret-token" {
		t.Fatalf("expected plaintext %q, got %q", "secret-token", plaintext)
	}
}

// TestCipherEncryptDecryptBytes verifies the byte helper preserves arbitrary binary plaintext.
func TestCipherEncryptDecryptBytes(t *testing.T) {
	cipher := newTestCipher(t, "blob/")
	plaintext := []byte{0, 1, 2, 3, 255}

	ciphertext, err := cipher.Encrypt(plaintext, "repo-1")
	if err != nil {
		t.Fatalf("encrypt bytes: %v", err)
	}

	decrypted, err := cipher.Decrypt(ciphertext, "repo-1")
	if err != nil {
		t.Fatalf("decrypt bytes: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected decrypted bytes %v, got %v", plaintext, decrypted)
	}
}

// TestCipherEncryptDecryptMultiPartScope verifies multi-part scopes round-trip and resist part-boundary collisions.
func TestCipherEncryptDecryptMultiPartScope(t *testing.T) {
	cipher := newTestCipher(t, "multipart/")

	ciphertext, err := cipher.EncryptString("secret-token", "user", "42", "repo", "7")
	if err != nil {
		t.Fatalf("encrypt multi-part scope: %v", err)
	}

	plaintext, err := cipher.DecryptString(ciphertext, "user", "42", "repo", "7")
	if err != nil {
		t.Fatalf("decrypt multi-part scope: %v", err)
	}
	if plaintext != "secret-token" {
		t.Fatalf("expected plaintext %q, got %q", "secret-token", plaintext)
	}

	// A scope whose parts concatenate to the same bytes but split differently must not decrypt,
	// proving the length-prefixed join keeps part boundaries part of the derived key.
	_, err = cipher.DecryptString(ciphertext, "user", "427", "epo", "7")
	if err == nil {
		t.Fatal("expected decrypt error for ambiguous part regrouping")
	}
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext, got %v", err)
	}
}

// TestCipherEncryptDecryptEmptyPlaintext verifies empty plaintext remains distinct from failures.
func TestCipherEncryptDecryptEmptyPlaintext(t *testing.T) {
	cipher := newTestCipher(t, "empty/")

	ciphertext, err := cipher.EncryptString("", "")
	if err != nil {
		t.Fatalf("encrypt empty plaintext: %v", err)
	}

	plaintext, err := cipher.DecryptString(ciphertext, "")
	if err != nil {
		t.Fatalf("decrypt empty plaintext: %v", err)
	}
	if plaintext != "" {
		t.Fatalf("expected empty plaintext, got %q", plaintext)
	}
}

// TestCipherUsesDifferentNonceForEachEncryption verifies AES-GCM uses a fresh nonce per encryption.
func TestCipherUsesDifferentNonceForEachEncryption(t *testing.T) {
	cipher := newTestCipher(t, "nonce/")

	first, err := cipher.EncryptString("same-plaintext", "same-scope")
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	second, err := cipher.EncryptString("same-plaintext", "same-scope")
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}
	if first == second {
		t.Fatal("expected different ciphertexts for repeated encryption")
	}
}

// TestCipherRejectsWrongScope verifies scope participates in key derivation.
func TestCipherRejectsWrongScope(t *testing.T) {
	cipher := newTestCipher(t, "scope/")

	ciphertext, err := cipher.EncryptString("secret-token", "user-1")
	if err != nil {
		t.Fatalf("encrypt string: %v", err)
	}

	_, err = cipher.DecryptString(ciphertext, "user-2")
	if err == nil {
		t.Fatal("expected decrypt error for wrong scope")
	}
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext, got %v", err)
	}
}

// TestCipherRejectsWrongInfoPrefix verifies InfoPrefix domain-separates derived keys.
func TestCipherRejectsWrongInfoPrefix(t *testing.T) {
	first := newTestCipher(t, "first/")
	second := newTestCipher(t, "second/")

	ciphertext, err := first.EncryptString("secret-token", "shared-scope")
	if err != nil {
		t.Fatalf("encrypt string: %v", err)
	}

	_, err = second.DecryptString(ciphertext, "shared-scope")
	if err == nil {
		t.Fatal("expected decrypt error for wrong info prefix")
	}
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext, got %v", err)
	}
}

// TestCipherRejectsMalformedCiphertext verifies version, encoding, and length validation.
func TestCipherRejectsMalformedCiphertext(t *testing.T) {
	cipher := newTestCipher(t, "malformed/")

	tests := []struct {
		name       string
		ciphertext string
		wantErr    error
	}{
		{
			name:       "unsupported version",
			ciphertext: "v2:abcd",
			wantErr:    ErrUnsupportedVersion,
		},
		{
			name:       "missing separator",
			ciphertext: "not-versioned",
			wantErr:    ErrUnsupportedVersion,
		},
		{
			name:       "invalid base64",
			ciphertext: "v1:not valid base64",
			wantErr:    ErrInvalidCiphertext,
		},
		{
			name:       "too short",
			ciphertext: "v1:AQID",
			wantErr:    ErrInvalidCiphertext,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cipher.DecryptString(tt.ciphertext, "scope")
			if err == nil {
				t.Fatal("expected decrypt error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestCipherDetectsTamperedCiphertext verifies AEAD integrity: any single-bit change in the
// nonce or sealed bytes must cause authentication to fail rather than return wrong plaintext.
func TestCipherDetectsTamperedCiphertext(t *testing.T) {
	cipher := newTestCipher(t, "tamper/")

	encoded, err := cipher.EncryptString("secret-token", "user-1")
	if err != nil {
		t.Fatalf("encrypt string: %v", err)
	}

	payload := strings.TrimPrefix(encoded, "v1:")
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	// Flip one bit in every byte position (nonce + ciphertext + tag) and require rejection.
	for i := range raw {
		mutated := append([]byte(nil), raw...)
		mutated[i] ^= 0x01
		tampered := "v1:" + base64.RawURLEncoding.EncodeToString(mutated)

		_, err := cipher.DecryptString(tampered, "user-1")
		if err == nil {
			t.Fatalf("expected tamper at byte %d to fail authentication", i)
		}
		if !errors.Is(err, ErrInvalidCiphertext) {
			t.Fatalf("expected ErrInvalidCiphertext at byte %d, got %v", i, err)
		}
	}
}

// TestCipherConcurrentUseIsRaceFree verifies a single Cipher can encrypt and decrypt from many
// goroutines at once. Run with -race to catch shared mutable state.
func TestCipherConcurrentUseIsRaceFree(t *testing.T) {
	cipher := newTestCipher(t, "concurrent/")

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			scope := "worker-" + strconv.Itoa(id)
			for i := 0; i < 50; i++ {
				ciphertext, err := cipher.EncryptString("payload", scope)
				if err != nil {
					errCh <- err
					return
				}
				plaintext, err := cipher.DecryptString(ciphertext, scope)
				if err != nil {
					errCh <- err
					return
				}
				if plaintext != "payload" {
					errCh <- errors.New("plaintext mismatch under concurrency")
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent cipher use failed: %v", err)
	}
}

// TestCipherReturnsRandomReaderError verifies nonce generation failures are returned.
func TestCipherReturnsRandomReaderError(t *testing.T) {
	cipher := newTestCipher(t, "random/")
	cipher.random = failingReader{}

	_, err := cipher.EncryptString("secret", "scope")
	if err == nil {
		t.Fatal("expected random reader error")
	}
	if !strings.Contains(err.Error(), "generate nonce") {
		t.Fatalf("expected nonce error, got %v", err)
	}
}

// TestCipherRejectsUninitializedCipher verifies nil and zero-value ciphers fail closed.
func TestCipherRejectsUninitializedCipher(t *testing.T) {
	var nilCipher *Cipher
	_, err := nilCipher.EncryptString("secret", "scope")
	if err == nil {
		t.Fatal("expected nil cipher encrypt error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}

	_, err = nilCipher.DecryptString("v1:AQID", "scope")
	if err == nil {
		t.Fatal("expected nil cipher decrypt error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}

	cipher := &Cipher{}
	_, err = cipher.EncryptString("secret", "scope")
	if err == nil {
		t.Fatal("expected uninitialized cipher encrypt error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

// TestCipherFallsBackToCryptoRandWhenReaderIsNil verifies a nil internal reader remains usable.
func TestCipherFallsBackToCryptoRandWhenReaderIsNil(t *testing.T) {
	cipher := newTestCipher(t, "nil-random/")
	cipher.random = nil

	ciphertext, err := cipher.EncryptString("secret", "scope")
	if err != nil {
		t.Fatalf("encrypt with nil random reader: %v", err)
	}
	plaintext, err := cipher.DecryptString(ciphertext, "scope")
	if err != nil {
		t.Fatalf("decrypt with nil random reader: %v", err)
	}
	if plaintext != "secret" {
		t.Fatalf("expected plaintext %q, got %q", "secret", plaintext)
	}
}

// TestNewGCMRejectsInvalidKeyLength verifies AES key length errors are wrapped consistently.
func TestNewGCMRejectsInvalidKeyLength(t *testing.T) {
	_, err := newGCM([]byte("short"))
	if err == nil {
		t.Fatal("expected invalid key length error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

// TestNewGCMRejectsInvalidBlockSize verifies GCM block-size errors are wrapped consistently.
func TestNewGCMRejectsInvalidBlockSize(t *testing.T) {
	_, err := newGCMFromBlock(wrongBlockSize{})
	if err == nil {
		t.Fatal("expected invalid block size error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

// TestNewCipherCopiesConfigSlices verifies caller mutation cannot alter initialized cipher state.
func TestNewCipherCopiesConfigSlices(t *testing.T) {
	masterKey := append([]byte(nil), testMasterKey...)
	salt := append([]byte(nil), testSalt...)

	cipher, err := NewCipher(Config{
		MasterKey:  masterKey,
		Salt:       salt,
		InfoPrefix: "copy/",
	})
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	for i := range masterKey {
		masterKey[i] = 0
	}
	for i := range salt {
		salt[i] = 0
	}

	ciphertext, err := cipher.EncryptString("secret", "scope")
	if err != nil {
		t.Fatalf("encrypt after config mutation: %v", err)
	}
	plaintext, err := cipher.DecryptString(ciphertext, "scope")
	if err != nil {
		t.Fatalf("decrypt after config mutation: %v", err)
	}
	if plaintext != "secret" {
		t.Fatalf("expected plaintext %q, got %q", "secret", plaintext)
	}
}

// TestBuildScopeIsStableAndUnambiguous verifies length-prefixed scope construction.
func TestBuildScopeIsStableAndUnambiguous(t *testing.T) {
	if buildScope() != "" {
		t.Fatalf("expected empty scope for no parts, got %q", buildScope())
	}

	first := buildScope("ab", "c")
	second := buildScope("a", "bc")

	if first == second {
		t.Fatalf("expected different scopes, got %q", first)
	}
	if first != buildScope("ab", "c") {
		t.Fatalf("expected stable scope, got %q", first)
	}
}

// newTestCipher creates a test cipher with valid keying material.
func newTestCipher(t *testing.T, infoPrefix string) *Cipher {
	t.Helper()

	cipher, err := NewCipher(Config{
		MasterKey:  testMasterKey,
		Salt:       testSalt,
		InfoPrefix: infoPrefix,
	})
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	if cipher.random == nil {
		t.Fatal("cipher random reader must be initialized")
	}
	return cipher
}
