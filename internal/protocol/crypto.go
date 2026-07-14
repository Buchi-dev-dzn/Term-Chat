package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const keyLen = chacha20poly1305.KeySize

func DeriveKey(room, passcode string) []byte {
	salt := sha256.Sum256([]byte("termchat|" + room))
	return argon2.IDKey([]byte(passcode), salt[:16], 1, 64*1024, 4, keyLen)
}

func AuthVerifier(room, passcode string) []byte {
	sum := sha256.Sum256([]byte(room + "\x00" + passcode + "\x00" + Version))
	return sum[:]
}

func EncryptEnvelope(room, sender, passcode, body string, now time.Time) (Envelope, error) {
	key := DeriveKey(room, passcode)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return Envelope{}, fmt.Errorf("cipher init: %w", err)
	}

	plaintext, err := json.Marshal(ChatMessage{Body: body})
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal message: %w", err)
	}

	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return Envelope{}, fmt.Errorf("nonce: %w", err)
	}

	env := Envelope{
		Version: Version,
		Room:    room,
		Sender:  sender,
		SentAt:  now.UTC(),
		Nonce:   nonce,
	}
	env.Ciphertext = aead.Seal(nil, nonce, plaintext, envelopeAAD(env))
	return env, nil
}

func DecryptEnvelope(env Envelope, passcode string) (ChatMessage, error) {
	if env.Version != Version {
		return ChatMessage{}, fmt.Errorf("unsupported version: %s", env.Version)
	}

	key := DeriveKey(env.Room, passcode)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("cipher init: %w", err)
	}

	plaintext, err := aead.Open(nil, env.Nonce, env.Ciphertext, envelopeAAD(env))
	if err != nil {
		return ChatMessage{}, fmt.Errorf("decrypt envelope: %w", err)
	}

	var msg ChatMessage
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		return ChatMessage{}, fmt.Errorf("decode message: %w", err)
	}
	return msg, nil
}

func envelopeAAD(env Envelope) []byte {
	sum := sha256.Sum256([]byte(env.Version + "|" + env.Room + "|" + env.Sender + "|" + env.SentAt.UTC().Format(time.RFC3339Nano)))
	out := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(out, sum[:])
	return out
}
