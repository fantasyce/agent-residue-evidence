package store

import (
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
)

func newTestStore(t *testing.T, options ...Option) *Store {
	t.Helper()
	state, err := Open(t.TempDir(), options...)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func testBaseline(taskID string, at time.Time) fsobserve.Baseline {
	return fsobserve.Baseline{CapturedAt: at, Entries: map[string]fsobserve.Entry{}}
}

func testReport(taskID, reportID string, at time.Time) contract.Report {
	return contract.Report{SchemaVersion: contract.ReportSchemaVersion, ReportID: reportID, TaskID: taskID, ObservationMode: contract.ObservationGuided, Status: contract.ReportNoCandidates, CreatedAt: at, Candidates: []contract.Candidate{}}
}
