package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	if err := json.Unmarshal(stdout.Bytes(), &begin); err != nil || begin.ObservationID == "" || begin.OwnerHandle == "" {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	if err := os.WriteFile(filepath.Join(root, "test.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = invoke(t, []string{"end"}, map[string]string{"owner_handle": begin.OwnerHandle})
	if code != 0 {
		t.Fatalf("end=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var report app.ReportSummary
	err := json.Unmarshal(stdout.Bytes(), &report)
	wantStatus := contract.ReportReviewRequired
	if len(report.Limitations) > 0 {
		wantStatus = contract.ReportPartialEvidence
	}
	if err != nil || report.Status != wantStatus {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	code, stdout, stderr = invoke(t, []string{"report", "get"}, map[string]any{"owner_handle": begin.OwnerHandle, "revision": 0, "limit": 20})
	if code != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(report.ReportID)) {
		t.Fatalf("get=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCLIRejectsIdentifierOnlyAccessAndEphemeralBegin(t *testing.T) {
	t.Setenv("ARE_HOME", t.TempDir())
	root := t.TempDir()
	code, _, stderr := invoke(t, []string{"begin"}, contract.TaskScope{TaskID: "ephemeral-cli", Workspace: root, RecoveryProfile: contract.RecoveryEphemeral})
	if code == 0 || !bytes.Contains(stderr.Bytes(), []byte("EPHEMERAL requires a persistent MCP session")) {
		t.Fatalf("ephemeral code=%d stderr=%s", code, stderr.String())
	}
	code, _, stderr = invoke(t, []string{"report", "get"}, map[string]string{"report_id": "report-guessed"})
	if code == 0 || !bytes.Contains(stderr.Bytes(), []byte("access denied")) {
		t.Fatalf("identifier-only code=%d stderr=%s", code, stderr.String())
	}
	secret := "are2.owner.must-not-be-echoed"
	code, _, stderr = invoke(t, []string{"report", "get"}, map[string]string{"owner_handle": secret})
	if code == 0 || bytes.Contains(stderr.Bytes(), []byte(secret)) {
		t.Fatalf("secret echoed code=%d stderr=%s", code, stderr.String())
	}
}

func TestCLIRetrospectiveGrantIsExplicitAndSingleUse(t *testing.T) {
	t.Setenv("ARE_HOME", t.TempDir())
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "historical.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := app.InspectCompletedInput{Scope: contract.TaskScope{TaskID: "retro-cli", Workspace: root}, StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour)}
	code, stdout, stderr := invoke(t, []string{"grant", "retrospective"}, request)
	if code != 0 {
		t.Fatalf("grant=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var grant app.RetrospectiveGrantResult
	if err := json.Unmarshal(stdout.Bytes(), &grant); err != nil || grant.GrantHandle == "" {
		t.Fatalf("grant=%#v err=%v", grant, err)
	}
	input := map[string]any{"grant_handle": grant.GrantHandle, "events": []any{}}
	code, stdout, stderr = invoke(t, []string{"inspect-completed"}, input)
	if code != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"observation_mode":"RETROSPECTIVE"`)) {
		t.Fatalf("inspect=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	code, _, _ = invoke(t, []string{"inspect-completed"}, input)
	if code == 0 {
		t.Fatal("single-use grant reused")
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
