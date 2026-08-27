package mcpserver

import (
	"context"

	"github.com/fantasyce/agent-residue-evidence/internal/app"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/versioninfo"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type taskIDInput struct {
	TaskID string `json:"task_id" jsonschema:"task identifier used when observation began"`
}

type reportIDInput struct {
	ReportID string `json:"report_id" jsonschema:"ARE report identifier"`
}

type appendEventsInput struct {
	TaskID string               `json:"task_id" jsonschema:"active observed task identifier"`
	Events []contract.TaskEvent `json:"events" jsonschema:"safe generic task events; an empty array is a heartbeat"`
}

type appendEventsOutput struct {
	TaskID     string `json:"task_id"`
	Accepted   bool   `json:"accepted"`
	EventCount int    `json:"event_count"`
}

func New(service *app.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-residue-evidence", Version: versioninfo.Version}, nil)
	closedWorld := false
	mutating := func(title string) *mcp.ToolAnnotations {
		destructive := false
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closedWorld}
	}
	readOnly := func(title string) *mcp.ToolAnnotations {
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &closedWorld, IdempotentHint: true}
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "begin_task_observation", Description: "Start local task-scoped residue observation before testing or building. ARE records evidence only and never cleans resources.",
		Annotations: mutating("Begin task residue observation"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input contract.TaskScope) (*mcp.CallToolResult, app.BeginResult, error) {
		output, err := service.Begin(ctx, input)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "append_task_events", Description: "Append optional safe generic events or an empty heartbeat to an active local task observation. Raw commands, environments, transcripts, secrets, and file contents are rejected.",
		Annotations: mutating("Append task evidence events"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input appendEventsInput) (*mcp.CallToolResult, appendEventsOutput, error) {
		err := service.AppendEvents(ctx, input.TaskID, input.Events)
		return nil, appendEventsOutput{TaskID: input.TaskID, Accepted: err == nil, EventCount: len(input.Events)}, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "end_task_observation", Description: "End one task-scoped observation and return local residue evidence for Agent and user review. ARE does not delete, stop, or close anything.",
		Annotations: mutating("End task residue observation"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input taskIDInput) (*mcp.CallToolResult, contract.Report, error) {
		output, err := service.End(ctx, input.TaskID)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "inspect_completed_task", Description: "Inspect an already completed task only inside explicit roots and a time window. Without a prospective baseline, evidence is always marked partial and never BASELINE_OBSERVED.",
		Annotations: mutating("Inspect completed task residue"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.InspectCompletedInput) (*mcp.CallToolResult, contract.Report, error) {
		output, err := service.InspectCompleted(ctx, input)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_residue_report", Description: "Read an existing local ARE evidence report without observing or changing user resources.",
		Annotations: readOnly("Get residue evidence report"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input reportIDInput) (*mcp.CallToolResult, contract.Report, error) {
		output, err := service.GetReport(input.ReportID)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "verify_task_residue", Description: "Recheck only stable candidates from an existing ARE report after the Agent and user handle cleanup. ARE performs no cleanup itself.",
		Annotations: mutating("Verify current residue state"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input reportIDInput) (*mcp.CallToolResult, contract.Report, error) {
		output, err := service.Verify(ctx, input.ReportID)
		return nil, output, err
	})
	return server
}

func Run(ctx context.Context, service *app.Service) error {
	return New(service).Run(ctx, &mcp.StdioTransport{})
}
