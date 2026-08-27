package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasyce/agent-residue-evidence/internal/app"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func invoke(t *testing.T, args []string, input any) (int, bytes.Buffer, bytes.Buffer) {
	t.Helper()
	var stdin, stdout, stderr bytes.Buffer
	if input != nil {
		if err := json.NewEncoder(&stdin).Encode(input); err != nil {
			t.Fatal(err)
		}
	}
	code := Run(args, &stdin, &stdout, &stderr)
	return code, stdout, stderr
}

func TestCLIJSONLifecycle(t *testing.T) {
	t.Setenv("ARE_HOME", t.TempDir())
	root := t.TempDir()
	code, stdout, stderr := invoke(t, []string{"begin"}, contract.TaskScope{TaskID: "task-cli", Workspace: root})
	if code != 0 {
		t.Fatalf("begin=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var begin app.BeginResult
	if err := json.Unmarshal(stdout.Bytes(), &begin); err != nil || begin.ObservationID == "" {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	if err := os.WriteFile(filepath.Join(root, "test.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = invoke(t, []string{"end"}, map[string]string{"task_id": "task-cli"})
	if code != 0 {
		t.Fatalf("end=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var report contract.Report
	err := json.Unmarshal(stdout.Bytes(), &report)
	wantStatus := contract.ReportReviewRequired
	if len(report.Limitations) > 0 {
		wantStatus = contract.ReportPartialEvidence
	}
	if err != nil || report.Status != wantStatus {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	code, stdout, stderr = invoke(t, []string{"report", "get"}, map[string]string{"report_id": report.ReportID})
	if code != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(report.ReportID)) {
		t.Fatalf("get=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCLIDoctorAndFailuresAreMachineReadable(t *testing.T) {
	t.Setenv("ARE_HOME", t.TempDir())
	code, stdout, stderr := invoke(t, []string{"doctor"}, nil)
	if code != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"network_access":false`)) {
		t.Fatalf("doctor=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	code, _, stderr = invoke(t, []string{"begin"}, map[string]string{"task_id": "missing-workspace"})
	if code == 0 || !bytes.Contains(stderr.Bytes(), []byte(`"error"`)) {
		t.Fatalf("failure=%d stderr=%s", code, stderr.String())
	}
}
