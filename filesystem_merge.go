package ciel

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func fsMerge(upper string, lower string, isBottom, includeSelf bool) error {
	errResetWalk := errors.New("reset walk")
	var err error
	for err == errResetWalk {
		err = filepath.Walk(upper, func(upath string, info os.FileInfo, err error) error {
			if info == nil {
				return err
			}
			if !includeSelf && upath == upper {
				return nil
			}
			rel, _ := filepath.Rel(upper, upath)
			lpath := filepath.Join(lower, rel)
			lpath = lpath
			if isWhiteout(info) {
				// NOTE: Delete file/directory in lower directory
				// os.RemoveAll(lpath)
				println("removeAll", lpath)
				if !isBottom {
					// os.Rename(upath, lpath)
					println("rename", lpath)
				}
			}

			return nil
		})
	}
	return err
}

func isWhiteout(fi os.FileInfo) bool {
	const mask = os.ModeDevice | os.ModeCharDevice
	if fi.Mode()&mask != mask {
		return false
	}
	return fi.Sys().(*syscall.Stat_t).Rdev == 0
}
