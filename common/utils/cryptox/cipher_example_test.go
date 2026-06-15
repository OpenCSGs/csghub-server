package cryptox_test

import (
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/utils/cryptox"
)

// ExampleCipher_EncryptString demonstrates how to initialize a cipher, encrypt a
// string, and decrypt it with the same multi-part scope.
func ExampleCipher_EncryptString() {
	// Initialize one reusable cipher with high-entropy master key material and a
	// stable random salt for this application domain. InfoPrefix separates this
	// use case from other secrets encrypted with the same master key and salt.
	cipher, err := cryptox.NewCipher(cryptox.Config{
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		Salt:       []byte("0123456789abcdef"),
		InfoPrefix: "agent/credential/",
	})
	if err != nil {
		panic(err)
	}

	// Scope values are extra context for key derivation. Pass each identifier as
	// its own argument instead of pre-joining them, and use the same values in the
	// same order when decrypting.
	encrypted, err := cipher.EncryptString("secret-token", "user-123", "credential-456")
	if err != nil {
		panic(err)
	}

	decrypted, err := cipher.DecryptString(encrypted, "user-123", "credential-456")
	if err != nil {
		panic(err)
	}

	fmt.Println("versioned:", strings.HasPrefix(encrypted, "v1:"))
	fmt.Println("plaintext:", decrypted)

	// Output:
	// versioned: true
	// plaintext: secret-token
}
