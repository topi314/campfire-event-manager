package server

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestSealTokenRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	sealed, err := sealToken(key, "eyJhbGciOiJIUzI1NiJ9")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("eyJ")) {
		t.Fatal("ciphertext contains plaintext prefix")
	}
	got, err := openToken(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got != "eyJhbGciOiJIUzI1NiJ9" {
		t.Fatalf("got %q", got)
	}
	if _, err := openToken(bytes.Repeat([]byte{1}, 32), sealed); err == nil {
		t.Fatal("expected decrypt failure with a different key")
	}
}
