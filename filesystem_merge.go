package ciel

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// MergeFile is the method to merge a file or directory from an upper layer
// to a lower layer.
func (fs *FileSystem) MergeFile(path, upper, lower string, excludeSelf bool) error {
	errResetWalk := errors.New("reset walk")
	uroot, lroot := fs.Layer(upper), fs.Layer(lower)
	lindex, maxindex := fs.layers.Index(lower), len(fs.layers)-1
	walkBase := filepath.Join(uroot, path)
	var err = errResetWalk
	for err == errResetWalk {
		err = filepath.Walk(walkBase, func(upath string, info os.FileInfo, err error) error {
			if excludeSelf && upath == walkBase {
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
					// the upper layer is a directory,
					// the lower layer is a whiteout or a normal file, which can be a cover,
					// that removing them may let the content in lower layers appear.

					// if the lower layer is at the bottom
					// or lower layers under the lower layer have another cover,
					// we can merge the upper one safely.
					nextfilelayer, havedir := fs.nextLayerHasFile(rel, lindex)
					if !havedir {
						os.Remove(lpath)
						if err := os.Rename(upath, lpath); err != nil {
							return err
						}
						return errResetWalk
					}

					// 1). "open" the directory
					os.Mkdir(lpath, 0000)
					if err := copyAttributes(upath, lpath); err != nil {
						return err
					}
					// 2). "cover" all sub-files in the directory
					for filename := range fs.readDirInRange(rel, lindex+1, nextfilelayer-1) {
						createWhiteout(filepath.Join(lpath, filename))
					}
					return nil
				}

			default:
				// the upper layer is a whiteout or a normal file, which acts as a cover.
				os.RemoveAll(lpath)
				err := os.Rename(upath, lpath)

				// a whiteout applied to the bottom?
				if lindex == maxindex && utp == overlayTypeWhiteout {
					os.Remove(lpath)
				}

				return err
			}

			// end of walk-function
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
	args := []string{
		"--no-target-directory",
		"--recursive",
		"--attributes-only",
		src,
		dst,
	}
	cmd := exec.Command("/bin/cp", args...)
	return cmd.Run()
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

func (fs *FileSystem) nextLayerHasFile(relpath string, startindex int) (index int, hasdir bool) {
	index = len(fs.layers)
	hasdir = false
	if startindex != len(fs.layers)-1 {
		for i := startindex + 1; i <= len(fs.layers)-1; i++ {
			iroot := filepath.Join(fs.base, fs.layers[i])
			ipath := filepath.Join(iroot, relpath)
			itp, _ := overlayTypeByLstat(ipath)
			switch itp {
			case overlayTypeFile, overlayTypeWhiteout:
				index = i
				return
			case overlayTypeDir:
				hasdir = true
			}
		}
	}
	return
}

func (fs *FileSystem) readDirInRange(relpath string, lbound, ubound int) map[string]bool {
	filelist := make(map[string]bool)
	for i := ubound; i >= lbound; i-- {
		iroot := filepath.Join(fs.base, fs.layers[i])
		ipath := filepath.Join(iroot, relpath)
		iinfo, err := os.Lstat(ipath)
		if os.IsNotExist(err) {
			continue
		}
		idir, err := os.Open(ipath)
		if err != nil {
			continue
		}
		iinfos, err := idir.Readdir(0) // 0: check all sub-files
		if err != nil {
			continue
		}
		for _, iiinfo := range iinfos {
			iitp, _ := overlayTypeByInfo(iiinfo, nil)
			if iitp == overlayTypeWhiteout {
				delete(filelist, iinfo.Name())
			} else {
				filelist[iinfo.Name()] = true
			}
		}
		idir.Close()
	}
	return filelist
}
