package workflow

import (
	"strconv"
	"strings"
)

// A Path points at a step inside nested branches, e.g. "2/y/0" is the first step of
// the Yes branch of the third top-level step. The empty path means "finished".

// Branch markers in a path.
const (
	BranchYes = "y"
	BranchNo  = "n"
)

// RootPath is the path of the top-level step at index i.
func RootPath(i int) string {
	return strconv.Itoa(i)
}

func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// StepAt returns the step a path points at.
func StepAt(steps []Step, path string) (Step, bool) {
	parts := splitPath(path)
	if len(parts) == 0 || len(parts)%2 == 0 {
		return Step{}, false
	}
	list := steps
	for i := 0; ; i += 2 {
		idx, err := strconv.Atoi(parts[i])
		if err != nil || idx < 0 || idx >= len(list) {
			return Step{}, false
		}
		if i == len(parts)-1 {
			return list[idx], true
		}
		s := list[idx]
		if s.Type != StepSplit {
			return Step{}, false
		}
		switch parts[i+1] {
		case BranchYes:
			list = s.Yes
		case BranchNo:
			list = s.No
		default:
			return Step{}, false
		}
	}
}

// NextPath returns the step after path: the next step in the same list, or the step
// after the enclosing split when a branch ends. Returns "" when the workflow is done.
func NextPath(steps []Step, path string) string {
	parts := splitPath(path)
	for len(parts) > 0 {
		last := len(parts) - 1
		idx, err := strconv.Atoi(parts[last])
		if err != nil {
			return ""
		}
		parts[last] = strconv.Itoa(idx + 1)
		candidate := strings.Join(parts, "/")
		if _, ok := StepAt(steps, candidate); ok {
			return candidate
		}
		// End of this list: climb out of the branch to the split itself.
		if len(parts) < 3 {
			return ""
		}
		parts = parts[:len(parts)-2]
	}
	return ""
}

// EnterBranch returns the first step of a split's branch, or the step after the split
// when that branch is empty.
func EnterBranch(steps []Step, splitPath, branch string) string {
	first := splitPath + "/" + branch + "/0"
	if _, ok := StepAt(steps, first); ok {
		return first
	}
	return NextPath(steps, splitPath)
}

// TopIndex is the top-level step index a path belongs to.
func TopIndex(path string) int {
	parts := splitPath(path)
	if len(parts) == 0 {
		return 0
	}
	i, _ := strconv.Atoi(parts[0])
	return i
}
