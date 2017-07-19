package ciel

import (
	"encoding/base64"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// SystemdPath is the path of systemd's excutable binary file in container.
// We use it to determine if the container has installed Systemd.
const SystemdPath = "/usr/lib/systemd/systemd"

// IsBootable returns whether the file system is bootable or not.
//
// NOTE: The basis of determining is the file /usr/lib/systemd/systemd.
func (fs *FileSystem) IsBootable() bool {
	fs.lock.RLock()
	defer fs.lock.RUnlock()
	if !fs.mounted {
		return false
	}
	if _, err := os.Stat(fs.TargetDir() + SystemdPath); os.IsNotExist(err) {
		return false
	}
	infolog.Println("IsBootable: true")
	return true
}

// IsMounted returns whether the file system has been mounted or not.
func (fs *FileSystem) IsMounted() bool {
	fs.lock.RLock()
	defer fs.lock.RUnlock()
	return fs.mounted
}

func (fs *FileSystem) BuildDirs() (err error) {
	e := os.Mkdir(fs.TopLayer(), 0755)
	if e != nil && !os.IsExist(e) {
		errlog.Println("BuildDirs: os.Mkdir() =>", e)
		if err == nil {
			err = e
		}
	}
	for _, layer := range fs.layers {
		dirname := filepath.Join(fs.base, layer)
		e := os.Mkdir(dirname, 0755)
		if e != nil && !os.IsExist(e) {
			errlog.Println("BuildDirs: os.Mkdir() =>", e)
			if err == nil {
				err = e
			}
		}
	}
	return
}

// Mount the file system to a temporary directory.
// It will be called automatically by CommandRaw().
func (fs *FileSystem) Mount() error {
	return fs.mount(true)
}

// MountReadOnly mounts the file system to a temporary directory, read-only.
func (fs *FileSystem) MountReadOnly() error {
	return fs.mount(false)
}

func (fs *FileSystem) mount(rw bool) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if fs.mounted {
		return nil
	}

	if err := fs.BuildDirs(); err != nil {
		return err
	}

	lowersToMount := []string{}
	for i := range fs.layers {
		if i != 0 && fs.layersMask[i] {
			dirname := filepath.Join(fs.base, fs.layers[i])
			lowersToMount = append(lowersToMount, dirname)
		}
	}

	fs.target = "/tmp/ciel." + randomFilename()
	os.Mkdir(fs.TargetDir(), 0755)
	os.Mkdir(fs.TopLayerWorkDir(), 0755)
	reterr := fsMount(fs.TargetDir(), rw, fs.TopLayer(), fs.TopLayerWorkDir(), lowersToMount)
	if reterr == nil {
		fs.mounted = true
	}
	return reterr
}

// Unmount the file system, and cleans the temporary directories.
func (fs *FileSystem) Unmount() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if !fs.mounted {
		return nil
	}

	if err := fsUnmount(fs.TargetDir()); err != nil {
		return err
	}
	defer func() {
		fs.mounted = false
	}()
	err1 := os.Remove(fs.TargetDir())
	err2 := os.RemoveAll(fs.TopLayerWorkDir())
	if err2 != nil {
		return err2
	}
	if err1 != nil {
		return err1
	}
	return nil
}

func randomFilename() string {
	const SIZE = 8
	rd := make([]byte, SIZE)
	if _, err := rand.Read(rd); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(rd)
}

func fsMount(path string, rw bool, upperdir string, workdir string, lowerdirs []string) error {
	var option string
	if rw {
		option = "lowerdir=" + strings.Join(lowerdirs, ":") + ",upperdir=" + upperdir + ",workdir=" + workdir
	} else {
		option = "lowerdir=" + strings.Join(append(lowerdirs, upperdir), ":")
	}
	infolog.Println("mount", path)
	dbglog.Println("fsMount: syscall.Mount() <=", path, option)
	err := syscall.Mount("overlay", path, "overlay", 0, option)
	dbglog.Println("fsMount: syscall.Mount() =>", err)
	return err
}

func fsUnmount(path string) error {
	infolog.Println("umount", path)
	dbglog.Println("fsUnmount: syscall.Unmount() <=", path)
	err := syscall.Unmount(path, 0)
	dbglog.Println("fsUnmount: syscall.Unmount() =>", err)
	return err
}
