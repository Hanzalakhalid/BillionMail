//go:build linux

package public

import (
	"os"

	"github.com/g0rbe/go-chattr"
)

func changeFileAttribute(file *os.File, attr string) error {
	var flag int32
	switch attr[1] {
	case 'i':
		flag = chattr.FS_IMMUTABLE_FL
	case 'a':
		flag = chattr.FS_APPEND_FL
	case 'd':
		flag = chattr.FS_NODUMP_FL
	case 's':
		flag = chattr.FS_SYNC_FL
	case 'c':
		flag = chattr.FS_COMPR_FL
	}
	if attr[0] == '+' {
		return chattr.SetAttr(file, flag)
	}
	return chattr.UnsetAttr(file, flag)
}
