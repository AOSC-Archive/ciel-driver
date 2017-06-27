package ciel

import (
	"encoding/base64"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

type Layers []string

func (ll Layers) Path(name string) string {
	return ll[ll.Index(name)]
}

func (ll Layers) Index(name string) int {
	for pos, fullname := range ll {
		fullnameSlice := strings.SplitN(fullname, "-", 2)
		if name == fullnameSlice[1] {
			return pos
		}
	}
	panic("no such lowerdir")
}

type FileSystem struct {
	lock sync.RWMutex

	layers     Layers
	layersMask []bool

	base   string
	target string

	mounted bool
}

const SystemdPath = "/usr/lib/systemd/systemd"
const WorkDirSuffix = ".work"

func (fs *FileSystem) TopLayer() string {
	return filepath.Join(fs.base, fs.layers[0])
}
func (fs *FileSystem) TopLayerWorkDir() string {
	return filepath.Join(fs.base, fs.layers[0]+WorkDirSuffix)
}
func (fs *FileSystem) Layer(name string) string {
	return filepath.Join(fs.base, fs.layers.Path(name))
}
func (fs *FileSystem) DisableAll() {
	for i := range fs.layersMask {
		fs.layersMask[i] = false
	}
}
func (fs *FileSystem) EnableAll() {
	for i := range fs.layersMask {
		fs.layersMask[i] = true
	}
}
func (fs *FileSystem) DisableLayer(names ...string) {
	for _, name := range names {
		fs.layersMask[fs.layers.Index(name)] = false
	}
}
func (fs *FileSystem) EnableLayer(names ...string) {
	for _, name := range names {
		fs.layersMask[fs.layers.Index(name)] = true
	}
}
func (fs *FileSystem) TargetDir() string {
	return fs.target
}

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
	return true
}

// IsFileSystemActive returns whether the file system has been mounted or not.
func (fs *FileSystem) IsMounted() bool {
	fs.lock.RLock()
	defer fs.lock.RUnlock()
	return fs.mounted
}

// Mount the file system to a temporary directory.
// It will be called automatically by CommandRaw().
func (fs *FileSystem) Mount() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if fs.mounted {
		return nil
	}

	os.Mkdir(fs.TopLayer(), 0755)
	lowersToMount := []string{}
	for i := range fs.layers {
		if i != 0 && fs.layersMask[i] {
			dirname := filepath.Join(fs.base, fs.layers[i])
			os.Mkdir(dirname, 0755)
			lowersToMount = append(lowersToMount, dirname)
		}
	}

	fs.target = "/tmp/ciel." + randomFilename()
	os.Mkdir(fs.TargetDir(), 0755)
	os.Mkdir(fs.TopLayerWorkDir(), 0755)
	reterr := fsMount(fs.TargetDir(), fs.TopLayer(), fs.TopLayerWorkDir(), lowersToMount)
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

func newFileSystem(base string, layers Layers) *FileSystem {
	fs := new(FileSystem)
	fs.base = base
	fs.layers = layers
	fs.layersMask = make([]bool, len(fs.layers))
	fs.EnableAll()
	return fs
}

func randomFilename() string {
	const SIZE = 8
	rd := make([]byte, SIZE)
	if _, err := rand.Read(rd); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(rd)
}

func fsMount(path string, upperdir string, workdir string, lowerdirs []string) error {
	return syscall.Mount("overlay", path, "overlay", 0,
		"lowerdir="+strings.Join(lowerdirs, ":")+",upperdir="+upperdir+",workdir="+workdir)
}

func fsUnmount(path string) error {
	return syscall.Unmount(path, 0)
}
