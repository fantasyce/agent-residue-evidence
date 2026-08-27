package fsobserve

import (
	"encoding/json"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

type Limits struct {
	MaxEntries  int
	MaxBytes    int64
	MaxDuration time.Duration
}

type Entry struct {
	RootIndex int       `json:"root_index"`
	Relative  string    `json:"relative"`
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	Identity  string    `json:"identity"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	Mode      uint32    `json:"mode"`
	Symlink   bool      `json:"symlink"`
}

type Baseline struct {
	Scope      scope.Validated  `json:"scope"`
	CapturedAt time.Time        `json:"captured_at"`
	Entries    map[string]Entry `json:"entries"`
	RootIDs    []string         `json:"root_ids"`
}

type Diff struct {
	Candidates  []contract.Candidate `json:"candidates"`
	Removed     []string             `json:"removed,omitempty"`
	Limitations []string             `json:"limitations,omitempty"`
}

func (d Diff) String() string {
	encoded, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return string(encoded)
}
