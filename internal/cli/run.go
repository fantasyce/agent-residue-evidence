package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fantasyce/agent-residue-evidence/internal/app"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/store"
	"github.com/fantasyce/agent-residue-evidence/internal/versioninfo"
)

type taskIDInput struct {
	TaskID string `json:"task_id"`
}

type reportIDInput struct {
	ReportID string `json:"report_id"`
}

type eventsInput struct {
	TaskID string               `json:"task_id"`
	Events []contract.TaskEvent `json:"events"`
}

type doctorResult struct {
	Healthy       bool   `json:"healthy"`
	Version       string `json:"version"`
	StateHome     string `json:"state_home"`
	NetworkAccess bool   `json:"network_access"`
	Telemetry     bool   `json:"telemetry"`
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintln(stdout, versioninfo.String())
		return 0
	}
	home, err := stateHome()
	if err != nil {
		return fail(stderr, err)
	}
	state, err := store.Open(home)
	if err != nil {
		return fail(stderr, err)
	}
	service := app.New(state)

	var result any
	usageFailure := false
	switch {
	case len(args) == 1 && args[0] == "begin":
		var input contract.TaskScope
		if err = decodeOne(stdin, &input); err == nil {
			result, err = service.Begin(contextBackground(), input)
		}
	case len(args) == 2 && args[0] == "event" && args[1] == "append":
		var input eventsInput
		if err = decodeOne(stdin, &input); err == nil {
			err = service.AppendEvents(contextBackground(), input.TaskID, input.Events)
			result = map[string]any{"task_id": input.TaskID, "accepted": err == nil, "event_count": len(input.Events)}
		}
	case len(args) == 1 && args[0] == "end":
		var input taskIDInput
		if err = decodeOne(stdin, &input); err == nil {
			result, err = service.End(contextBackground(), input.TaskID)
		}
	case len(args) == 1 && args[0] == "inspect-completed":
		var input app.InspectCompletedInput
		if err = decodeOne(stdin, &input); err == nil {
			result, err = service.InspectCompleted(contextBackground(), input)
		}
	case len(args) == 2 && args[0] == "report" && args[1] == "get":
		var input reportIDInput
		if err = decodeOne(stdin, &input); err == nil {
			result, err = service.GetReport(input.ReportID)
		}
	case len(args) == 2 && args[0] == "report" && args[1] == "retain":
		var input reportIDInput
		if err = decodeOne(stdin, &input); err == nil {
			err = state.RetainReport(input.ReportID)
			result = map[string]any{"report_id": input.ReportID, "retained": err == nil}
		}
	case len(args) == 2 && args[0] == "report" && args[1] == "forget":
		var input reportIDInput
		if err = decodeOne(stdin, &input); err == nil {
			err = state.ForgetReport(input.ReportID)
			result = map[string]any{"report_id": input.ReportID, "forgotten": err == nil}
		}
	case len(args) == 1 && args[0] == "verify":
		var input reportIDInput
		if err = decodeOne(stdin, &input); err == nil {
			result, err = service.Verify(contextBackground(), input.ReportID)
		}
	case len(args) == 1 && args[0] == "doctor":
		result = doctorResult{Healthy: true, Version: versioninfo.Version, StateHome: home, NetworkAccess: false, Telemetry: false}
	case len(args) == 1 && args[0] == "mcp":
		err = errors.New("MCP transport is not linked in this build stage")
	default:
		err = errors.New("usage: agent-residue-evidence begin|event append|end|inspect-completed|report get|report retain|report forget|verify|doctor|mcp|--version")
		usageFailure = true
	}
	if err != nil {
		code := fail(stderr, err)
		if usageFailure {
			return 2
		}
		return code
	}
	if result == nil {
		result = map[string]bool{"ok": true}
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return fail(stderr, err)
	}
	return 0
}

var contextBackground = func() context.Context { return context.Background() }

func decodeOne(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 16*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func fail(stderr io.Writer, err error) int {
	var buffer bytes.Buffer
	_ = json.NewEncoder(&buffer).Encode(map[string]string{"error": err.Error()})
	_, _ = io.Copy(stderr, &buffer)
	return 1
}

func stateHome() (string, error) {
	if override := os.Getenv("ARE_HOME"); override != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "AgentResidueEvidence"), nil
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "AgentResidueEvidence"), nil
		}
		return filepath.Join(home, "AppData", "Local", "AgentResidueEvidence"), nil
	default:
		if state := os.Getenv("XDG_STATE_HOME"); state != "" {
			return filepath.Join(state, "agent-residue-evidence"), nil
		}
		return filepath.Join(home, ".local", "state", "agent-residue-evidence"), nil
	}
}
