package store

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"
)

func TestEncryptedEnvelopeRoundTripHasNoPlaintext(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	secret := struct {
		TaskID string `json:"task_id"`
		Path   string `json:"path"`
	}{TaskID: "private-task-marker", Path: "/var/tmp/example-workspace/build.log"}
	envelope, err := sealRecord("task", "opaque-record", 0, time.Now().UTC(), time.Now().UTC().Add(time.Hour), "", false, key, secret)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte(secret.TaskID), []byte(secret.Path), []byte("GoalBoard")} {
		if bytes.Contains(raw, forbidden) {
			t.Fatalf("ciphertext envelope leaked %q: %s", forbidden, raw)
		}
	}
	var decoded struct {
		TaskID string `json:"task_id"`
		Path   string `json:"path"`
	}
	if err := openRecord(envelope, key, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != secret {
		t.Fatalf("decoded=%#v want=%#v", decoded, secret)
	}
}

func TestEncryptedEnvelopeRejectsWrongKeyAndTampering(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	envelope, err := sealRecord("report", "opaque-record", 2, time.Now().UTC(), time.Now().UTC().Add(time.Hour), "", false, key, map[string]string{"value": "private"})
	if err != nil {
		t.Fatal(err)
	}
	wrong := bytes.Repeat([]byte{2}, 32)
	if err := openRecord(envelope, wrong, new(map[string]string)); err == nil {
		t.Fatal("wrong key decrypted record")
	}
	envelope.Ciphertext = envelope.Ciphertext[:len(envelope.Ciphertext)-1] + "A"
	if err := openRecord(envelope, key, new(map[string]string)); err == nil {
		t.Fatal("tampered record decrypted")
	}
}

func TestEncryptedEnvelopeBindsPublicMetadata(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	envelope, err := sealRecord("task", "opaque-record", 0, time.Now().UTC(), time.Now().UTC().Add(time.Hour), "original-public-key", false, key, map[string]string{"value": "private"})
	if err != nil {
		t.Fatal(err)
	}
	envelope.PublicKey = "attacker-public-key"
	if err := openRecord(envelope, key, new(map[string]string)); err == nil {
		t.Fatal("modified public metadata authenticated")
	}
}
