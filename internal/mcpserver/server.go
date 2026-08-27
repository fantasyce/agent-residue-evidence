package mcpserver

import (
	"context"
	"sync"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/app"
	"github.com/fantasyce/agent-residue-evidence/internal/capability"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/versioninfo"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type accessInput struct {
	OwnerHandle   string `json:"owner_handle,omitempty" jsonschema:"opaque recoverable owner handle; never use a task or report id"`
	ObservationID string `json:"observation_id,omitempty" jsonschema:"ephemeral observation id bound to this MCP session"`
	ReportID      string `json:"report_id,omitempty" jsonschema:"optional display binding; never authorizes access"`
}

type appendEventsInput struct {
	accessInput
	ExecutorHandle string               `json:"executor_handle,omitempty" jsonschema:"opaque append-only executor handle returned by delegate_task_executor"`
	Events         []contract.TaskEvent `json:"events" jsonschema:"safe generic agent-task-event/2.0 events; an empty array is a heartbeat"`
}

type reportInput struct {
	accessInput
	Revision int    `json:"revision,omitempty" jsonschema:"zero for the immutable observation report, positive for a verification revision"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"opaque cursor returned by the previous page"`
	Limit    int    `json:"limit,omitempty" jsonschema:"candidate groups per page, from 1 to 100"`
}

type delegateInput struct {
	accessInput
	ExpiresAt    time.Time            `json:"expires_at"`
	AllowedTypes []contract.EventType `json:"allowed_event_types"`
	AllowedRoots []string             `json:"allowed_root_aliases"`
}

type delegateOutput struct {
	ExecutorHandle string `json:"executor_handle"`
}

type resolveInput struct {
	accessInput
	CandidateID string `json:"candidate_id"`
}

type resolveOutput struct {
	CandidateID string `json:"candidate_id"`
	ExactPath   string `json:"exact_path"`
}

type retrospectiveInput struct {
	GrantHandle string               `json:"grant_handle"`
	Events      []contract.TaskEvent `json:"events,omitempty"`
}

type appendEventsOutput struct {
	Accepted   bool `json:"accepted"`
	EventCount int  `json:"event_count"`
}

type sessionOwners struct {
	mu     sync.Mutex
	owners map[*mcp.ServerSession]map[string]string
}

func newSessionOwners() *sessionOwners {
	return &sessionOwners{owners: map[*mcp.ServerSession]map[string]string{}}
}

func (owners *sessionOwners) put(session *mcp.ServerSession, observationID, ownerHandle string) {
	owners.mu.Lock()
	defer owners.mu.Unlock()
	if owners.owners[session] == nil {
		owners.owners[session] = map[string]string{}
	}
	owners.owners[session][observationID] = ownerHandle
}

func (owners *sessionOwners) resolve(session *mcp.ServerSession, input accessInput) (string, error) {
	if input.OwnerHandle != "" {
		return input.OwnerHandle, nil
	}
	owners.mu.Lock()
	defer owners.mu.Unlock()
	if input.ObservationID == "" || owners.owners[session] == nil || owners.owners[session][input.ObservationID] == "" {
		return "", capability.ErrAccessDenied
	}
	return owners.owners[session][input.ObservationID], nil
}

func (owners *sessionOwners) forget(session *mcp.ServerSession, observationID string) {
	owners.mu.Lock()
	defer owners.mu.Unlock()
	delete(owners.owners[session], observationID)
}

func New(service *app.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-residue-evidence", Version: versioninfo.Version}, nil)
	sessions := newSessionOwners()
	closedWorld := false
	mutating := func(title string) *mcp.ToolAnnotations {
		destructive := false
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closedWorld}
	}
	readOnly := func(title string) *mcp.ToolAnnotations {
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &closedWorld, IdempotentHint: true}
	}

	mcp.AddTool(server, &mcp.Tool{Name: "begin_task_observation", Description: "Start local task-scoped residue observation. Returns a recoverable owner handle unless EPHEMERAL is requested.", Annotations: mutating("Begin task residue observation")}, func(ctx context.Context, request *mcp.CallToolRequest, input contract.TaskScope) (*mcp.CallToolResult, app.BeginResult, error) {
		output, err := service.Begin(ctx, input)
		if err != nil {
			return nil, app.BeginResult{}, err
		}
		if output.RecoveryProfile == contract.RecoveryEphemeral {
			sessions.put(request.Session, output.ObservationID, output.OwnerHandle)
			output.OwnerHandle = ""
		}
		return nil, output, nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "append_task_events", Description: "Append safe events using owner, executor, or same-session ephemeral authority.", Annotations: mutating("Append task evidence events")}, func(ctx context.Context, request *mcp.CallToolRequest, input appendEventsInput) (*mcp.CallToolResult, appendEventsOutput, error) {
		handle := input.OwnerHandle
		if input.ExecutorHandle != "" {
			if handle != "" || input.ObservationID != "" {
				return nil, appendEventsOutput{}, capability.ErrAccessDenied
			}
			handle = input.ExecutorHandle
		}
		if handle == "" {
			var err error
			handle, err = sessions.resolve(request.Session, input.accessInput)
			if err != nil {
				return nil, appendEventsOutput{}, err
			}
		}
		err := service.AppendEvents(ctx, handle, input.Events)
		return nil, appendEventsOutput{Accepted: err == nil, EventCount: len(input.Events)}, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "end_task_observation", Description: "End one observation with owner authority and return a bounded grouped summary.", Annotations: mutating("End task residue observation")}, func(ctx context.Context, request *mcp.CallToolRequest, input accessInput) (*mcp.CallToolResult, app.ReportSummary, error) {
		handle, err := sessions.resolve(request.Session, input)
		if err != nil {
			return nil, app.ReportSummary{}, err
		}
		if _, err := service.End(ctx, handle); err != nil {
			return nil, app.ReportSummary{}, err
		}
		output, err := service.Summarize(handle, 0, "", 20)
		if input.ObservationID != "" {
			sessions.forget(request.Session, input.ObservationID)
		}
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "inspect_completed_task", Description: "Inspect a completed task only with an explicit local retrospective scope grant; evidence is always partial.", Annotations: mutating("Inspect completed task residue")}, func(ctx context.Context, _ *mcp.CallToolRequest, input retrospectiveInput) (*mcp.CallToolResult, app.ReportSummary, error) {
		if _, err := service.InspectCompletedAuthorized(ctx, input.GrantHandle, input.Events); err != nil {
			return nil, app.ReportSummary{}, err
		}
		output, err := service.Summarize(input.GrantHandle, 0, "", 20)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "get_residue_report", Description: "Read one bounded report page with owner or same-session ephemeral authority. IDs alone never authorize access.", Annotations: readOnly("Get residue evidence report")}, func(_ context.Context, request *mcp.CallToolRequest, input reportInput) (*mcp.CallToolResult, app.ReportSummary, error) {
		handle, err := sessions.resolve(request.Session, input.accessInput)
		if err != nil {
			return nil, app.ReportSummary{}, err
		}
		limit := input.Limit
		if limit == 0 {
			limit = 20
		}
		output, err := service.Summarize(handle, input.Revision, input.Cursor, limit)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "verify_task_residue", Description: "Append an immutable verification revision for the original candidates. ARE performs no cleanup.", Annotations: mutating("Verify current residue state")}, func(ctx context.Context, request *mcp.CallToolRequest, input accessInput) (*mcp.CallToolResult, app.ReportSummary, error) {
		handle, err := sessions.resolve(request.Session, input)
		if err != nil {
			return nil, app.ReportSummary{}, err
		}
		revision, err := service.Verify(ctx, handle)
		if err != nil {
			return nil, app.ReportSummary{}, err
		}
		output, err := service.Summarize(handle, revision.Revision, "", 20)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "delegate_task_executor", Description: "Mint a short-lived append-only executor handle from owner authority.", Annotations: mutating("Delegate task event append")}, func(_ context.Context, request *mcp.CallToolRequest, input delegateInput) (*mcp.CallToolResult, delegateOutput, error) {
		handle, err := sessions.resolve(request.Session, input.accessInput)
		if err != nil {
			return nil, delegateOutput{}, err
		}
		executor, err := service.DelegateExecutor(handle, input.ExpiresAt, input.AllowedTypes, input.AllowedRoots)
		return nil, delegateOutput{ExecutorHandle: executor}, err
	})

	mcp.AddTool(server, &mcp.Tool{Name: "resolve_residue_candidate", Description: "Reveal one existing candidate path only after explicit user approval. This is not a directory browser.", Annotations: readOnly("Resolve approved residue candidate")}, func(_ context.Context, request *mcp.CallToolRequest, input resolveInput) (*mcp.CallToolResult, resolveOutput, error) {
		handle, err := sessions.resolve(request.Session, input.accessInput)
		if err != nil {
			return nil, resolveOutput{}, err
		}
		exact, err := service.ResolveCandidate(handle, input.CandidateID)
		return nil, resolveOutput{CandidateID: input.CandidateID, ExactPath: exact}, err
	})

	return server
}

func Run(ctx context.Context, service *app.Service) error {
	return New(service).Run(ctx, &mcp.StdioTransport{})
}
