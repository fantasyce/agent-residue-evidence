package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const encryptedFormatVersion = "are-encrypted-record/2.0"

type encryptedEnvelope struct {
	FormatVersion string    `json:"format_version"`
	RecordKind    string    `json:"record_kind"`
	OpaqueID      string    `json:"opaque_id"`
	Revision      int       `json:"revision"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	PublicKey     string    `json:"public_key,omitempty"`
	Protected     bool      `json:"protected,omitempty"`
	Nonce         string    `json:"nonce"`
	Ciphertext    string    `json:"ciphertext"`
}

func sealRecord(kind, opaqueID string, revision int, createdAt, expiresAt time.Time, publicKey string, protected bool, key []byte, value any) (encryptedEnvelope, error) {
	if len(key) != 32 || kind == "" || opaqueID == "" || revision < 0 || createdAt.IsZero() || expiresAt.IsZero() {
		return encryptedEnvelope{}, errors.New("invalid encrypted record parameters")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedEnvelope{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedEnvelope{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return encryptedEnvelope{}, err
	}
	plaintext, err := json.Marshal(value)
	if err != nil {
		return encryptedEnvelope{}, err
	}
	envelope := encryptedEnvelope{
		FormatVersion: encryptedFormatVersion, RecordKind: kind, OpaqueID: opaqueID, Revision: revision,
		CreatedAt: createdAt.UTC(), ExpiresAt: expiresAt.UTC(), PublicKey: publicKey, Protected: protected, Nonce: base64.RawURLEncoding.EncodeToString(nonce),
	}
	envelope.Ciphertext = base64.RawURLEncoding.EncodeToString(aead.Seal(nil, nonce, plaintext, envelope.associatedData()))
	return envelope, nil
}

func openRecord(envelope encryptedEnvelope, key []byte, target any) error {
	if len(key) != 32 || envelope.FormatVersion != encryptedFormatVersion || envelope.RecordKind == "" || envelope.OpaqueID == "" || envelope.Revision < 0 {
		return errors.New("encrypted record authentication failed")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return errors.New("encrypted record authentication failed")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return errors.New("encrypted record authentication failed")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return errors.New("encrypted record authentication failed")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return errors.New("encrypted record authentication failed")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, envelope.associatedData())
	if err != nil {
		return errors.New("encrypted record authentication failed")
	}
	if err := json.Unmarshal(plaintext, target); err != nil {
		return errors.New("encrypted record authentication failed")
	}
	return nil
}

func (envelope encryptedEnvelope) associatedData() []byte {
	return []byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s\x00%s\x00%s\x00%t",
		envelope.FormatVersion, envelope.RecordKind, envelope.OpaqueID, envelope.Revision,
		envelope.CreatedAt.UTC().Format(time.RFC3339Nano), envelope.ExpiresAt.UTC().Format(time.RFC3339Nano), envelope.PublicKey, envelope.Protected))
}
