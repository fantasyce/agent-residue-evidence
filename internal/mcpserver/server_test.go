package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/app"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testService(t *testing.T) *app.Service {
	t.Helper()
	state, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return app.New(state)
}

func testClient(t *testing.T, service *app.Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := New(service).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "are-test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return clientSession
}

func TestToolSurfaceIsEvidenceOnly(t *testing.T) {
	client := testClient(t, testService(t))
	listed, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("tool is not closed-world: %#v", tool)
		}
		for _, forbidden := range []string{"cleanup", "delete", "execute"} {
			if tool.Name == forbidden {
				t.Fatalf("forbidden tool: %s", tool.Name)
			}
		}
	}
	sort.Strings(names)
	want := []string{"append_task_events", "begin_task_observation", "delegate_task_executor", "end_task_observation", "get_residue_report", "inspect_completed_task", "resolve_residue_candidate", "verify_task_residue"}
	if stringList(names) != stringList(want) {
		t.Fatalf("tools=%v want=%v", names, want)
	}
}

func TestBeginEndGetVerifyRoundTrip(t *testing.T) {
	root := t.TempDir()
	client := testClient(t, testService(t))
	begin, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "begin_task_observation", Arguments: map[string]any{"task_id": "task-mcp", "workspace": root, "observation_mode": "GUIDED", "recovery_profile": "RECOVERABLE"}})
	if err != nil || begin.IsError {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	var begun app.BeginResult
	decodeStructured(t, begin.StructuredContent, &begun)
	if begun.OwnerHandle == "" {
		t.Fatalf("begin did not return recoverable owner handle: %#v", begun)
	}
	if err := os.WriteFile(filepath.Join(root, "mcp.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	end, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "end_task_observation", Arguments: map[string]any{"owner_handle": begun.OwnerHandle}})
	if err != nil || end.IsError {
		t.Fatalf("end=%#v err=%v", end, err)
	}
	var report app.ReportSummary
	decodeStructured(t, end.StructuredContent, &report)
	wantStatus := contract.ReportReviewRequired
	if len(report.Limitations) > 0 {
		wantStatus = contract.ReportPartialEvidence
	}
	if report.Status != wantStatus || report.CandidateTotal != 1 || len(report.Candidates) != 1 {
		t.Fatalf("report=%#v", report)
	}
	for _, item := range []struct {
		name      string
		arguments map[string]any
	}{
		{name: "get_residue_report", arguments: map[string]any{"owner_handle": begun.OwnerHandle, "revision": 0, "limit": 20}},
		{name: "verify_task_residue", arguments: map[string]any{"owner_handle": begun.OwnerHandle}},
	} {
		result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: item.name, Arguments: item.arguments})
		if err != nil || result.IsError {
			t.Fatalf("%s=%#v err=%v", item.name, result, err)
		}
	}
}

func TestReportIDAloneAndWrongOwnerCannotAccessAnotherTask(t *testing.T) {
	root := t.TempDir()
	client := testClient(t, testService(t))
	begin, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "begin_task_observation", Arguments: map[string]any{"task_id": "private-task", "workspace": root}})
	if err != nil || begin.IsError {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	var begun app.BeginResult
	decodeStructured(t, begin.StructuredContent, &begun)
	end, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "end_task_observation", Arguments: map[string]any{"owner_handle": begun.OwnerHandle}})
	if err != nil || end.IsError {
		t.Fatalf("end=%#v err=%v", end, err)
	}
	var summary app.ReportSummary
	decodeStructured(t, end.StructuredContent, &summary)
	for _, arguments := range []map[string]any{
		{"report_id": summary.ReportID},
		{"owner_handle": "are2.owner.invalid", "report_id": summary.ReportID},
		{"owner_handle": begun.ObservationID, "report_id": summary.ReportID},
	} {
		result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_residue_report", Arguments: arguments})
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("unauthorized arguments accepted: %#v result=%#v", arguments, result)
		}
	}
}

func TestPublicIdentifiersNeverAuthorizeCrossTaskOperations(t *testing.T) {
	root := t.TempDir()
	client := testClient(t, testService(t))
	begin, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "begin_task_observation", Arguments: map[string]any{"task_id": "task-a", "workspace": root}})
	if err != nil || begin.IsError {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	var begun app.BeginResult
	decodeStructured(t, begin.StructuredContent, &begun)
	if err := os.WriteFile(filepath.Join(root, "private.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ended, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "end_task_observation", Arguments: map[string]any{"owner_handle": begun.OwnerHandle}})
	if err != nil || ended.IsError {
		t.Fatalf("end=%#v err=%v", ended, err)
	}
	var summary app.ReportSummary
	decodeStructured(t, ended.StructuredContent, &summary)
	candidateID := ""
	if len(summary.Candidates) > 0 {
		candidateID = summary.Candidates[0].ID
	}
	attacks := []struct {
		name string
		args map[string]any
	}{
		{"append_task_events", map[string]any{"observation_id": begun.ObservationID, "events": []any{}}},
		{"end_task_observation", map[string]any{"observation_id": begun.ObservationID}},
		{"get_residue_report", map[string]any{"report_id": summary.ReportID}},
		{"verify_task_residue", map[string]any{"report_id": summary.ReportID}},
		{"delegate_task_executor", map[string]any{"report_id": summary.ReportID, "expires_at": time.Now().Add(time.Minute), "allowed_event_types": []any{"artifact_declared"}, "allowed_root_aliases": []any{"workspace://"}}},
		{"resolve_residue_candidate", map[string]any{"report_id": summary.ReportID, "candidate_id": candidateID}},
		{"inspect_completed_task", map[string]any{"grant_handle": summary.ReportID, "events": []any{}}},
	}
	for _, attack := range attacks {
		result, callErr := client.CallTool(context.Background(), &mcp.CallToolParams{Name: attack.name, Arguments: attack.args})
		if callErr == nil && (result == nil || !result.IsError) {
			t.Fatalf("%s accepted public identifiers: %#v", attack.name, result)
		}
	}
}

func TestEphemeralObservationStaysInSessionAndCannotResumeAfterRestart(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	state, err := store.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	service := app.New(state)
	client := testClient(t, service)
	begin, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "begin_task_observation", Arguments: map[string]any{"task_id": "ephemeral-task", "workspace": root, "recovery_profile": "EPHEMERAL"}})
	if err != nil || begin.IsError {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	var begun app.BeginResult
	decodeStructured(t, begin.StructuredContent, &begun)
	if begun.OwnerHandle != "" || begun.ObservationID == "" {
		t.Fatalf("ephemeral authority exposed: %#v", begun)
	}
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "append_task_events", Arguments: map[string]any{"observation_id": begun.ObservationID, "events": []any{}}})
	if err != nil || result.IsError {
		t.Fatalf("same-session append=%#v err=%v", result, err)
	}
	restarted, err := store.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	restartedClient := testClient(t, app.New(restarted))
	result, err = restartedClient.CallTool(context.Background(), &mcp.CallToolParams{Name: "end_task_observation", Arguments: map[string]any{"observation_id": begun.ObservationID}})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatalf("ephemeral task resumed after restart: %#v", result)
	}
}

func TestRetrospectiveToolRequiresExplicitGrant(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "historical.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	service := testService(t)
	now := time.Now().UTC()
	grant, err := service.GrantRetrospective(app.InspectCompletedInput{Scope: contract.TaskScope{TaskID: "retro-mcp", Workspace: root}, StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	client := testClient(t, service)
	without, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_completed_task", Arguments: map[string]any{"events": []any{}}})
	if err == nil && (without == nil || !without.IsError) {
		t.Fatalf("ungranted inspection accepted: %#v", without)
	}
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_completed_task", Arguments: map[string]any{"grant_handle": grant.GrantHandle, "events": []any{}}})
	if err != nil || result.IsError {
		t.Fatalf("inspection=%#v err=%v", result, err)
	}
	var summary app.ReportSummary
	decodeStructured(t, result.StructuredContent, &summary)
	if summary.Status != contract.ReportPartialEvidence || summary.ObservationMode != contract.ObservationRetrospective {
		t.Fatalf("summary=%#v", summary)
	}
}

func TestInvalidScopeAndUnknownFieldsAreRejected(t *testing.T) {
	client := testClient(t, testService(t))
	for _, arguments := range []map[string]any{
		{"task_id": "task-root", "workspace": string(filepath.Separator)},
		{"task_id": "task-extra", "workspace": t.TempDir(), "raw_command": "forbidden"},
	} {
		result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "begin_task_observation", Arguments: arguments})
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("arguments accepted: %#v result=%#v err=%v", arguments, result, err)
		}
	}
}

func decodeStructured(t *testing.T, value any, target any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}

func stringList(values []string) string {
	raw, _ := json.Marshal(values)
	return string(raw)
}
