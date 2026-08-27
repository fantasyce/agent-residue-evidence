package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

type Store struct {
	home       string
	tasksDir   string
	reportsDir string
	grantsDir  string
	eventsDir  string
	config     config
	mu         sync.Mutex
}

func Open(home string, options ...Option) (*Store, error) {
	if home == "" {
		return nil, errors.New("store home is required")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	configuration := config{
		capacity: 100 * 1024 * 1024, retention: 7 * 24 * time.Hour,
		interruption: 24 * time.Hour, clock: time.Now,
	}
	for _, option := range options {
		option(&configuration)
	}
	if configuration.capacity <= 0 || configuration.retention <= 0 || configuration.interruption <= 0 || configuration.clock == nil {
		return nil, errors.New("store configuration is invalid")
	}
	store := &Store{home: abs, tasksDir: filepath.Join(abs, "tasks"), reportsDir: filepath.Join(abs, "reports"), grantsDir: filepath.Join(abs, "grants"), eventsDir: filepath.Join(abs, "events"), config: configuration}
	for _, directory := range []string{store.home, store.tasksDir, store.reportsDir, store.grantsDir, store.eventsDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, err
		}
		if err := protectPrivatePath(directory, true); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func reportDigest(report contract.Report) (string, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 128*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("stored JSON has trailing data")
	}
	return nil
}

func atomicWriteJSON(path string, value any) (returnErr error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".write-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := protectPrivatePath(temporaryPath, false); err != nil {
		_ = temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	if err := encoder.Encode(value); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}
	return nil
}
