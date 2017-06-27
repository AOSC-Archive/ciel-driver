package ciel

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func (fs *FileSystem) Merge(path, upperx, lowerx string, includeSelf bool) error {
	panic("not implemented")
	// FIXME: wip

	errResetWalk := errors.New("reset walk")
	uroot, lroot := fs.Layer(upperx), fs.Layer(lowerx)
	isBottom := fs.layers.Index(lowerx) == len(fs.layers)-1
	walkBase := filepath.Join(uroot, path)
	var err error
	for err == errResetWalk {
		err = filepath.Walk(walkBase, func(upath string, info os.FileInfo, err error) error {
			if !includeSelf && upath == walkBase {
				return nil
			}
			rel, _ := filepath.Rel(uroot, upath)
			lpath := filepath.Join(lroot, rel)

			utp, err := overlayTypeByInfo(info, err)
			if err != nil {
				return err
			}
			ltp, err := overlayTypeByLstat(lpath)
			if err != nil {
				return err
			}

			switch utp {
			case overlayTypeDir:
				switch ltp {
				case overlayTypeAir:
					// the lower layer had no effect on this position.
					if err := os.Rename(upath, lpath); err != nil {
						return err
					}
					return errResetWalk // sub-file list has been affected, reset this process.
				case overlayTypeDir:
					// copy attributes, and continue.
					return copyAttributes(upath, lpath)
				default:
					// whiteout or normal file, which can be a cover,
					// that removing them may let the content in lower layers appear.

					// "open" the directory
					os.Mkdir(lpath, 0000)
					if err := copyAttributes(upath, lpath); err != nil {
						return err
					}
					// "cover" all sub-files in the directory

					return nil
				}
			default:
				// whiteout or normal file, which acts as a cover.
				os.RemoveAll(lpath)
				if isBottom && utp == overlayTypeWhiteout {
					return nil
				}
				return os.Rename(upath, lpath)
			}

			return nil
		})
	}
	return err
}

type overlayType int

const (
	overlayTypeInvalid overlayType = iota

	overlayTypeAir
	overlayTypeWhiteout
	overlayTypeFile
	overlayTypeDir
)

func copyAttributes(src, dst string) error {
	// TODO: copyAttributes
	return nil
}

func createWhiteout(path string) error {
	return syscall.Mknod(path, 0000, 0x0000)
}

func overlayTypeByLstat(path string) (overlayType, error) {
	return overlayTypeByInfo(os.Lstat(path))
}

func overlayTypeByInfo(info os.FileInfo, err error) (overlayType, error) {
	if os.IsNotExist(err) {
		return overlayTypeAir, nil
	} else if err != nil {
		return overlayTypeInvalid, err
	}
	if info.IsDir() {
		return overlayTypeDir, nil
	}
	if isWhiteout(info) {
		return overlayTypeWhiteout, nil
	}
	return overlayTypeFile, nil
}

func isWhiteout(fi os.FileInfo) bool {
	const mask = os.ModeDevice | os.ModeCharDevice
	if fi.Mode()&mask != mask {
		return false
	}
	return fi.Sys().(*syscall.Stat_t).Rdev == 0
}
