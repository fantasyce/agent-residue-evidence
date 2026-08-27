package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

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
	want := []string{"append_task_events", "begin_task_observation", "end_task_observation", "get_residue_report", "inspect_completed_task", "verify_task_residue"}
	if stringList(names) != stringList(want) {
		t.Fatalf("tools=%v want=%v", names, want)
	}
}

func TestBeginEndGetVerifyRoundTrip(t *testing.T) {
	root := t.TempDir()
	client := testClient(t, testService(t))
	begin, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "begin_task_observation", Arguments: map[string]any{"task_id": "task-mcp", "workspace": root}})
	if err != nil || begin.IsError {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	if err := os.WriteFile(filepath.Join(root, "mcp.log"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	end, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "end_task_observation", Arguments: map[string]any{"task_id": "task-mcp"}})
	if err != nil || end.IsError {
		t.Fatalf("end=%#v err=%v", end, err)
	}
	var report contract.Report
	decodeStructured(t, end.StructuredContent, &report)
	wantStatus := contract.ReportReviewRequired
	if len(report.Limitations) > 0 {
		wantStatus = contract.ReportPartialEvidence
	}
	if report.Status != wantStatus {
		t.Fatalf("report=%#v", report)
	}
	for _, tool := range []string{"get_residue_report", "verify_task_residue"} {
		result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: map[string]any{"report_id": report.ReportID}})
		if err != nil || result.IsError {
			t.Fatalf("%s=%#v err=%v", tool, result, err)
		}
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
