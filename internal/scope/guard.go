package scope

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

var (
	ErrScopeTooBroad    = errors.New("task scope is too broad")
	ErrUnsafePath       = errors.New("task scope path is unsafe")
	ErrOverlappingRoots = errors.New("task scope roots overlap")
)

type Root struct {
	Path     string `json:"path"`
	Identity string `json:"identity"`
}

type Validated struct {
	TaskID string `json:"task_id"`
	Roots  []Root `json:"roots"`
}

type Guard struct {
	home string
}

func NewGuard() Guard {
	home, _ := os.UserHomeDir()
	return Guard{home: filepath.Clean(home)}
}

func (g Guard) Validate(input contract.TaskScope) (Validated, error) {
	if input.TaskID == "" || len(input.TaskID) > 256 || input.Workspace == "" {
		return Validated{}, errors.New("task_id and workspace are required")
	}
	rawRoots := append([]string{input.Workspace}, input.TempRoots...)
	roots := make([]Root, 0, len(rawRoots))
	for _, raw := range rawRoots {
		root, err := g.validateRoot(raw)
		if err != nil {
			return Validated{}, err
		}
		for _, prior := range roots {
			if pathsOverlap(prior.Path, root.Path) {
				return Validated{}, fmt.Errorf("%w: %s and %s", ErrOverlappingRoots, prior.Path, root.Path)
			}
		}
		roots = append(roots, root)
	}
	return Validated{TaskID: input.TaskID, Roots: roots}, nil
}

func (g Guard) validateRoot(raw string) (Root, error) {
	if raw == "" || hasParentTraversal(raw) {
		return Root{}, ErrUnsafePath
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return Root{}, fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	abs = filepath.Clean(abs)
	volume := filepath.VolumeName(abs)
	volumeRoot := volume + string(filepath.Separator)
	if abs == string(filepath.Separator) || (volume != "" && strings.EqualFold(abs, volumeRoot)) || samePath(abs, g.home) {
		return Root{}, fmt.Errorf("%w: %s", ErrScopeTooBroad, abs)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return Root{}, fmt.Errorf("scope root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Root{}, fmt.Errorf("%w: root must be a real directory", ErrUnsafePath)
	}
	return Root{Path: abs, Identity: fmt.Sprintf("%#v", info.Sys())}, nil
}

func hasParentTraversal(path string) bool {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

func pathsOverlap(a, b string) bool {
	if samePath(a, b) {
		return true
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	if b == "" {
		return false
	}
	if filepath.Separator == '\\' {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
