package process

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type NativeAdapter interface {
	Snapshot(context.Context) ([]Metadata, error)
	ListeningPorts(context.Context, Identity) ([]Port, error)
	HoldsAnyPath(context.Context, Identity, []string) (bool, error)
}

type Observer struct {
	roots  []string
	native NativeAdapter
	now    func() time.Time
}

func NewObserver(roots []string, native NativeAdapter) *Observer {
	cleaned := make([]string, 0, len(roots))
	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if resolved, err := filepath.EvalSymlinks(cleanRoot); err == nil {
			cleanRoot = resolved
		}
		cleaned = append(cleaned, cleanRoot)
	}
	return &Observer{roots: cleaned, native: native, now: time.Now}
}

func NewNativeObserver(roots []string) *Observer {
	return NewObserver(roots, nativeAdapter{})
}

func (o *Observer) Baseline(ctx context.Context) (Baseline, error) {
	processes, err := o.native.Snapshot(ctx)
	if err != nil {
		return Baseline{}, err
	}
	sortMetadata(processes)
	return Baseline{CapturedAt: o.now().UTC(), Processes: processes}, nil
}

func (o *Observer) Resolve(ctx context.Context, hints Hints) ([]Evidence, []Limitation) {
	processes, err := o.native.Snapshot(ctx)
	if err != nil {
		return nil, []Limitation{{Operation: "process_snapshot", Detail: err.Error()}}
	}
	byPID := make(map[int]Metadata, len(processes))
	for _, metadata := range processes {
		byPID[metadata.Identity.PID] = metadata
	}
	attributed := make(map[int]Attribution)
	bindStableHints(attributed, byPID, hints.ReceiptProcesses, AttributionReceipt)
	bindStableHints(attributed, byPID, hints.EventProcesses, AttributionEvent)
	for _, metadata := range processes {
		if o.withinTaskRoot(metadata.WorkingDir) {
			if _, exists := attributed[metadata.Identity.PID]; !exists {
				attributed[metadata.Identity.PID] = AttributionWorkingDirectory
			}
		}
	}

	limitations := []Limitation{}
	if len(hints.CandidatePaths) > 0 {
		for _, metadata := range processes {
			if _, exists := attributed[metadata.Identity.PID]; exists {
				continue
			}
			holds, err := o.native.HoldsAnyPath(ctx, metadata.Identity, hints.CandidatePaths)
			if err != nil {
				limitations = appendLimitationOnce(limitations, Limitation{Operation: "open_path_attribution", Detail: err.Error()})
				continue
			}
			if holds {
				attributed[metadata.Identity.PID] = AttributionOpenPath
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, metadata := range processes {
			if _, exists := attributed[metadata.Identity.PID]; exists {
				continue
			}
			if _, parentAttributed := attributed[metadata.ParentPID]; parentAttributed {
				attributed[metadata.Identity.PID] = AttributionDescendant
				changed = true
			}
		}
	}

	evidence := make([]Evidence, 0, len(attributed))
	for pid, reason := range attributed {
		metadata := byPID[pid]
		ports, err := o.native.ListeningPorts(ctx, metadata.Identity)
		if err != nil {
			limitations = append(limitations, Limitation{Operation: "listening_ports", Detail: fmt.Sprintf("pid %d: %v", pid, err)})
		} else {
			sort.Slice(ports, func(i, j int) bool {
				if ports[i].Protocol == ports[j].Protocol {
					if ports[i].Address == ports[j].Address {
						return ports[i].Number < ports[j].Number
					}
					return ports[i].Address < ports[j].Address
				}
				return ports[i].Protocol < ports[j].Protocol
			})
		}
		evidence = append(evidence, Evidence{Identity: metadata.Identity, ParentPID: metadata.ParentPID, Reason: reason, Ports: ports})
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].Identity.PID < evidence[j].Identity.PID })
	return evidence, limitations
}

func (o *Observer) Verify(ctx context.Context, identities []Identity) (map[int]Evidence, []Limitation) {
	evidence, limitations := o.Resolve(ctx, Hints{EventProcesses: identities})
	verified := make(map[int]Evidence, len(evidence))
	for _, item := range evidence {
		for _, identity := range identities {
			if sameIdentity(item.Identity, identity) {
				verified[item.Identity.PID] = item
				break
			}
		}
	}
	return verified, limitations
}

func bindStableHints(target map[int]Attribution, processes map[int]Metadata, hints []Identity, reason Attribution) {
	for _, hint := range hints {
		metadata, exists := processes[hint.PID]
		if !exists || !sameIdentity(metadata.Identity, hint) {
			continue
		}
		if _, alreadyBound := target[hint.PID]; !alreadyBound {
			target[hint.PID] = reason
		}
	}
}

func sameIdentity(left, right Identity) bool {
	return left.PID == right.PID && left.CreatedAt.Equal(right.CreatedAt)
}

func (o *Observer) withinTaskRoot(path string) bool {
	if path == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleanPath); err == nil {
		cleanPath = resolved
	}
	for _, root := range o.roots {
		relative, err := filepath.Rel(root, cleanPath)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func sortMetadata(processes []Metadata) {
	sort.Slice(processes, func(i, j int) bool { return processes[i].Identity.PID < processes[j].Identity.PID })
}

func appendLimitationOnce(values []Limitation, value Limitation) []Limitation {
	for _, existing := range values {
		if existing.Operation == value.Operation && existing.Detail == value.Detail {
			return values
		}
	}
	return append(values, value)
}
