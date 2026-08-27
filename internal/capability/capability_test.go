package capability

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestOwnerHandleRoundTripAndWrongKindDenial(t *testing.T) {
	owner, err := NewOwner()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseOwner(owner.String())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.OpaqueID != owner.OpaqueID || parsed.RecordKey != owner.RecordKey || parsed.SigningSeed != owner.SigningSeed {
		t.Fatalf("parsed owner does not match: %#v %#v", parsed, owner)
	}
	if _, err := ParseExecutor(owner.String(), time.Now().UTC()); err == nil {
		t.Fatal("owner handle accepted as executor")
	}
}

func TestExecutorCannotForgeOwnerAndExpires(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	owner, err := NewOwner()
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewExecutor(owner, now.Add(time.Hour), []string{"artifact_declared"}, []string{"workspace"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseExecutor(executor.String(), now)
	if err != nil {
		t.Fatal(err)
	}
	public := ed25519.NewKeyFromSeed(owner.SigningSeed[:]).Public().(ed25519.PublicKey)
	if err := parsed.Verify(public); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseOwner(executor.String()); err == nil {
		t.Fatal("executor handle accepted as owner")
	}
	if _, err := ParseExecutor(executor.String(), now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired executor accepted")
	}
}

func TestMalformedHandlesReturnGenericAccessDenied(t *testing.T) {
	for _, raw := range []string{"", "report-guess", "are2.owner.not-base64", "are2.executor.not-base64"} {
		if _, err := ParseOwner(raw); err == nil || err.Error() != ErrAccessDenied.Error() {
			t.Fatalf("raw=%q err=%v", raw, err)
		}
	}
}
