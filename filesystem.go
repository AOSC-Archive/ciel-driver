package ciel

import (
	"path/filepath"
	"strings"
	"sync"
)

// Layers is type of array of strings. It contains the full name of directories
// eg. ["99-upper", "50-custom", "00-bottom"]. You must sort your layers from
// top to bottom, ie. the top layer must be the first one.
type Layers []string

// Path returns the full name of directory.
//
// Example: Path("custom") returns "50-custom"
func (ll Layers) Path(name string) string {
	return ll[ll.Index(name)]
}

// Index returns the index of layer in array.
func (ll Layers) Index(name string) int {
	for pos, fullname := range ll {
		fullnameSlice := strings.SplitN(fullname, "-", 2)
		if name == fullnameSlice[1] {
			return pos
		}
	}
	panic("no such layer: " + name)
}

// FileSystem contains the layers of overlay file system and implements
// methods to operate it, such as Mount() and Unmount().
type FileSystem struct {
	lock sync.RWMutex

	layers     Layers
	layersMask []bool

	base   string
	target string

	mounted bool
}

// WorkDirSuffix is the suffix of workdir. It appends to the upperdir (TopLayer).
const WorkDirSuffix = ".work"

// TopLayer do the same thing of Layer(...), but it only returns the top layer,
// the "difference layer".
func (fs *FileSystem) TopLayer() string {
	return filepath.Join(fs.base, fs.layers[0])
}

// TopLayerWorkDir returns the full name of directory of "workdir", the temporary
// directory for overlay file system. It uses WorkDirSuffix as the suffix.
func (fs *FileSystem) TopLayerWorkDir() string {
	return filepath.Join(fs.base, fs.layers[0]+WorkDirSuffix)
}

// Layer returns the full name of the directory of the layer.
// Same as Layers.Path(name) .
func (fs *FileSystem) Layer(name string) string {
	return filepath.Join(fs.base, fs.layers.Path(name))
}

// DisableAll disables all layer, it will go into effect at the next mount.
func (fs *FileSystem) DisableAll() {
	for i := range fs.layersMask {
		fs.layersMask[i] = false
	}
}

// EnableAll enables all layer, it will go into effect at the next mount.
func (fs *FileSystem) EnableAll() {
	for i := range fs.layersMask {
		fs.layersMask[i] = true
	}
}

// DisableLayer disables a layer in file system, it will go into effect at the next mount.
func (fs *FileSystem) DisableLayer(names ...string) {
	for _, name := range names {
		fs.layersMask[fs.layers.Index(name)] = false
	}
}

// EnableLayer enables a layer in file system, it will go into effect at the next mount.
func (fs *FileSystem) EnableLayer(names ...string) {
	for _, name := range names {
		fs.layersMask[fs.layers.Index(name)] = true
	}
}

// TargetDir returns the path of merged directory.
//
// Do not access TargetDir before or after file system is active (mounted).
// It may be a null string, or does not exist.
func (fs *FileSystem) TargetDir() string {
	return fs.target
}

func newFileSystem(base string, layers Layers) *FileSystem {
	fs := new(FileSystem)
	fs.base = base
	fs.layers = layers
	fs.layersMask = make([]bool, len(fs.layers))
	fs.EnableAll()
	return fs
}
