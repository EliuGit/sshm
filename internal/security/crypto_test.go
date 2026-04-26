package security

import (
	"path/filepath"
	"testing"
)

func TestLoadOrCreateKeyAndRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "app.key")
	crypto, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	ciphertext, err := crypto.Encrypt("secret-value")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	plaintext, err := crypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "secret-value" {
		t.Fatalf("Decrypt() = %q, want %q", plaintext, "secret-value")
	}
}
