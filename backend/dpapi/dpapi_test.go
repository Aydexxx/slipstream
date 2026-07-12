package dpapi

import (
	"bytes"
	"testing"
)

func TestProtectUnprotectRoundTrip(t *testing.T) {
	secret := []byte("[Interface]\nPrivateKey = abc123==\nAddress = 10.66.66.2/32\n")

	blob, err := Protect(secret)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if bytes.Contains(blob, []byte("PrivateKey")) {
		t.Fatal("ciphertext still contains plaintext markers")
	}

	got, err := Unprotect(blob)
	if err != nil {
		t.Fatalf("Unprotect: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("round-trip mismatch:\n got %q\nwant %q", got, secret)
	}
}

func TestUnprotectRejectsGarbage(t *testing.T) {
	if _, err := Unprotect([]byte("not a real dpapi blob")); err == nil {
		t.Fatal("expected Unprotect to fail on non-DPAPI input")
	}
}

func TestEmptyInputsRejected(t *testing.T) {
	if _, err := Protect(nil); err == nil {
		t.Error("Protect(nil) should error")
	}
	if _, err := Unprotect(nil); err == nil {
		t.Error("Unprotect(nil) should error")
	}
}
