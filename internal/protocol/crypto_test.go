package protocol

import (
	"testing"
	"time"
)

func TestDeriveKeyDeterministic(t *testing.T) {
	a := DeriveKey("lobby", "secret")
	b := DeriveKey("lobby", "secret")
	c := DeriveKey("other", "secret")

	if string(a) != string(b) {
		t.Fatal("same inputs should derive same key")
	}
	if string(a) == string(c) {
		t.Fatal("different room should derive different key")
	}
}

func TestEncryptDecryptEnvelope(t *testing.T) {
	env, err := EncryptEnvelope("lobby", "alice", "secret", "hello", time.Unix(1710000000, 0).UTC())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	msg, err := DecryptEnvelope(env, "secret")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if msg.Body != "hello" {
		t.Fatalf("body=%q", msg.Body)
	}
}

func TestRejectTamperedCiphertext(t *testing.T) {
	env, err := EncryptEnvelope("lobby", "alice", "secret", "hello", time.Now().UTC())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	env.Ciphertext[0] ^= 0xff
	if _, err := DecryptEnvelope(env, "secret"); err == nil {
		t.Fatal("expected tampered ciphertext to fail")
	}
}
