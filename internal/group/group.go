package group

import (
	"encoding/base64"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func Candidates(input []contract.Candidate) []contract.CandidateGroup {
	candidates := append([]contract.Candidate(nil), input...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Path == candidates[j].Path {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Path < candidates[j].Path
	})
	used := make([]bool, len(candidates))
	groups := make([]contract.CandidateGroup, 0, len(candidates))
	for index, candidate := range candidates {
		if used[index] {
			continue
		}
		group := contract.CandidateGroup{ID: candidate.ID, Kind: candidate.Kind, Path: candidate.Path, EvidenceLevel: candidate.EvidenceLevel, CurrentStatus: candidate.CurrentStatus, AggregateSizeBytes: candidate.SizeBytes}
		if candidate.Kind == contract.CandidateDirectory && candidate.Path != "" {
			for childIndex := index + 1; childIndex < len(candidates); childIndex++ {
				child := candidates[childIndex]
				if !descendant(candidate.Path, child.Path) {
					continue
				}
				if child.EvidenceLevel != candidate.EvidenceLevel || child.CurrentStatus != candidate.CurrentStatus || len(child.Conflicts) > 0 {
					continue
				}
				used[childIndex] = true
				group.DescendantCount++
				group.AggregateSizeBytes += child.SizeBytes
				if len(group.SamplePaths) < 3 {
					group.SamplePaths = append(group.SamplePaths, child.Path)
				}
			}
		}
		groups = append(groups, group)
	}
	return groups
}

func Page(items []contract.CandidateGroup, cursor string, limit int) (contract.CandidatePage, error) {
	if limit < 1 || limit > 100 {
		return contract.CandidatePage{}, errors.New("page limit must be between 1 and 100")
	}
	start := 0
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return contract.CandidatePage{}, errors.New("invalid page cursor")
		}
		start, err = strconv.Atoi(string(raw))
		if err != nil || start < 0 || start > len(items) {
			return contract.CandidatePage{}, errors.New("invalid page cursor")
		}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := contract.CandidatePage{Items: append([]contract.CandidateGroup(nil), items[start:end]...), Total: len(items)}
	if end < len(items) {
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	return page, nil
}

func descendant(parent, child string) bool {
	return child != parent && strings.HasPrefix(child, strings.TrimSuffix(parent, "/")+"/")
}
