package capability

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	ownerPrefix    = "are2.owner."
	executorPrefix = "are2.executor."
)

var ErrAccessDenied = errors.New("access denied")

type Owner struct {
	OpaqueID    string   `json:"opaque_id"`
	RecordKey   [32]byte `json:"record_key"`
	SigningSeed [32]byte `json:"signing_seed"`
}

type Executor struct {
	OpaqueID     string   `json:"opaque_id"`
	GrantID      string   `json:"grant_id"`
	AppendKey    [32]byte `json:"append_key"`
	ExpiresUnix  int64    `json:"expires_unix"`
	AllowedTypes []string `json:"allowed_types,omitempty"`
	AllowedRoots []string `json:"allowed_roots,omitempty"`
	Signature    []byte   `json:"signature"`
}

type executorClaims struct {
	OpaqueID     string   `json:"opaque_id"`
	GrantID      string   `json:"grant_id"`
	AppendKey    [32]byte `json:"append_key"`
	ExpiresUnix  int64    `json:"expires_unix"`
	AllowedTypes []string `json:"allowed_types,omitempty"`
	AllowedRoots []string `json:"allowed_roots,omitempty"`
}

func NewOwner() (Owner, error) {
	var owner Owner
	opaque, err := randomBytes(18)
	if err != nil {
		return Owner{}, err
	}
	owner.OpaqueID = base64.RawURLEncoding.EncodeToString(opaque)
	if _, err := rand.Read(owner.RecordKey[:]); err != nil {
		return Owner{}, err
	}
	if _, err := rand.Read(owner.SigningSeed[:]); err != nil {
		return Owner{}, err
	}
	return owner, nil
}

func (owner Owner) String() string {
	return ownerPrefix + encode(owner)
}

func ParseOwner(raw string) (Owner, error) {
	if !strings.HasPrefix(raw, ownerPrefix) {
		return Owner{}, ErrAccessDenied
	}
	var owner Owner
	if err := decode(strings.TrimPrefix(raw, ownerPrefix), &owner); err != nil || owner.OpaqueID == "" || allZero(owner.RecordKey[:]) || allZero(owner.SigningSeed[:]) {
		return Owner{}, ErrAccessDenied
	}
	return owner, nil
}

func NewExecutor(owner Owner, expiresAt time.Time, allowedTypes, allowedRoots []string) (Executor, error) {
	grant, err := randomBytes(18)
	if err != nil {
		return Executor{}, err
	}
	executor := Executor{
		OpaqueID: owner.OpaqueID, GrantID: base64.RawURLEncoding.EncodeToString(grant),
		ExpiresUnix: expiresAt.UTC().Unix(), AllowedTypes: sortedUnique(allowedTypes), AllowedRoots: sortedUnique(allowedRoots),
	}
	if _, err := rand.Read(executor.AppendKey[:]); err != nil {
		return Executor{}, err
	}
	claims, err := json.Marshal(executor.claims())
	if err != nil {
		return Executor{}, err
	}
	private := ed25519.NewKeyFromSeed(owner.SigningSeed[:])
	executor.Signature = ed25519.Sign(private, claims)
	return executor, nil
}

func (executor Executor) String() string {
	return executorPrefix + encode(executor)
}

func ParseExecutor(raw string, now time.Time) (Executor, error) {
	if !strings.HasPrefix(raw, executorPrefix) {
		return Executor{}, ErrAccessDenied
	}
	var executor Executor
	if err := decode(strings.TrimPrefix(raw, executorPrefix), &executor); err != nil || executor.OpaqueID == "" || executor.GrantID == "" || allZero(executor.AppendKey[:]) || len(executor.Signature) != ed25519.SignatureSize {
		return Executor{}, ErrAccessDenied
	}
	if !now.UTC().Before(time.Unix(executor.ExpiresUnix, 0).UTC()) {
		return Executor{}, ErrAccessDenied
	}
	return executor, nil
}

func (executor Executor) Verify(public ed25519.PublicKey) error {
	claims, err := json.Marshal(executor.claims())
	if err != nil || !ed25519.Verify(public, claims, executor.Signature) {
		return ErrAccessDenied
	}
	return nil
}

func (executor Executor) claims() executorClaims {
	return executorClaims{
		OpaqueID: executor.OpaqueID, GrantID: executor.GrantID, AppendKey: executor.AppendKey,
		ExpiresUnix: executor.ExpiresUnix, AllowedTypes: executor.AllowedTypes, AllowedRoots: executor.AllowedRoots,
	}
}

func encode(value any) string {
	raw, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decode(raw string, target any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	_, err := rand.Read(value)
	return value, err
}

func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}
