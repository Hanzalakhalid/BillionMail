//go:build !linux

package public

import (
	"fmt"
	"os"
	"runtime"
)

func changeFileAttribute(_ *os.File, _ string) error {
	return fmt.Errorf("filesystem attributes are not supported on %s", runtime.GOOS)
}
