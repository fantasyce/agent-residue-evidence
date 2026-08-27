package process

import (
	"encoding/json"
	"time"
)

type Identity struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

type Metadata struct {
	Identity   Identity `json:"identity"`
	ParentPID  int      `json:"parent_pid"`
	WorkingDir string   `json:"working_directory,omitempty"`
}

type Port struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Number   int    `json:"number"`
}

type Attribution string

const (
	AttributionEvent            Attribution = "event"
	AttributionReceipt          Attribution = "receipt"
	AttributionWorkingDirectory Attribution = "working_directory"
	AttributionOpenPath         Attribution = "open_candidate_path"
	AttributionDescendant       Attribution = "descendant"
)

type Evidence struct {
	Identity  Identity    `json:"identity"`
	ParentPID int         `json:"parent_pid"`
	Reason    Attribution `json:"reason"`
	Ports     []Port      `json:"ports,omitempty"`
}

type Limitation struct {
	Operation string `json:"operation"`
	Detail    string `json:"detail"`
}

type Hints struct {
	EventProcesses   []Identity
	ReceiptProcesses []Identity
	CandidatePaths   []string
}

type Baseline struct {
	CapturedAt time.Time  `json:"captured_at"`
	Processes  []Metadata `json:"processes"`
}

func (b Baseline) String() string {
	encoded, err := json.Marshal(b)
	if err != nil {
		return ""
	}
	return string(encoded)
}
