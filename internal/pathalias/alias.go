package pathalias

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

type Binding struct {
	Alias    string `json:"alias"`
	Path     string `json:"path"`
	Identity string `json:"identity"`
}

type Table struct {
	Bindings []Binding `json:"bindings"`
}

func New(validated scope.Validated) (Table, error) {
	if len(validated.Roots) == 0 {
		return Table{}, errors.New("at least one task root is required")
	}
	bindings := make([]Binding, 0, len(validated.Roots))
	for index, root := range validated.Roots {
		alias := "workspace://"
		if index > 0 {
			alias = fmt.Sprintf("temp://%d", index-1)
		}
		identity, err := stableIdentity(root.Path)
		if err != nil {
			return Table{}, err
		}
		bindings = append(bindings, Binding{Alias: alias, Path: filepath.Clean(root.Path), Identity: identity})
	}
	return Table{Bindings: bindings}, nil
}

func (table Table) Project(exact string) (string, error) {
	exact = filepath.Clean(exact)
	for _, binding := range table.Bindings {
		relative, err := filepath.Rel(binding.Path, exact)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if relative == "." {
			return binding.Alias, nil
		}
		return strings.TrimSuffix(binding.Alias, "/") + "/" + filepath.ToSlash(relative), nil
	}
	return "", errors.New("path is outside bound roots")
}

func (table Table) Resolve(alias string) (string, error) {
	for _, binding := range table.Bindings {
		prefix := strings.TrimSuffix(binding.Alias, "/")
		if alias != binding.Alias && !strings.HasPrefix(alias, prefix+"/") {
			continue
		}
		identity, err := stableIdentity(binding.Path)
		if err != nil || identity != binding.Identity {
			return "", errors.New("bound root identity changed")
		}
		relative := strings.TrimPrefix(alias, prefix)
		relative = strings.TrimPrefix(relative, "/")
		if relative == "" {
			return binding.Path, nil
		}
		if filepath.IsAbs(relative) || hasTraversal(relative) {
			return "", errors.New("alias path is unsafe")
		}
		resolved := filepath.Clean(filepath.Join(binding.Path, filepath.FromSlash(relative)))
		check, err := filepath.Rel(binding.Path, resolved)
		if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
			return "", errors.New("alias path escapes bound root")
		}
		return resolved, nil
	}
	return "", errors.New("unknown path alias")
}

func hasTraversal(value string) bool {
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}
