package ciel

import (
	"context"
	"io"
	"os"
	"sync"
)

const SHELLPATH = "/bin/bash"

type Container struct {
	lock sync.RWMutex

	name       string
	fs         *Filesystem
	properties []string
	boot       bool

	booted   bool
	chrooted bool

	cancelBoot   chan struct{}
	cancelChroot chan struct{}
}

// New creates a container descriptor, but it won't start the container immediately.
//
// You may want to call Command() after this.
func New(name, baseDir string) *Container {
	c := &Container{
		name:       name,
		fs:         new(Filesystem),
		properties: []string{},
		boot:       true,
		cancelBoot: make(chan struct{}),
	}
	c.SetBaseDir(baseDir)
	return c
}

// Command calls the command line with shell ("SHELLPATH -l -c <cmdline>") in container,
// returns the exit code.
//
// Don't worry about mounting file system, starting container and the mode of booting.
// Please check out CommandRaw() for more details.
//
// NOTE: It calls CommandRaw() internally, using os.Stdin, os.Stdout, os.Stderr.
func (c *Container) Command(cmdline string) int {
	return c.CommandContext(context.Background(), cmdline)
}

// CommandRaw runs command in container.
//
// It will mount the root file system and start the container automatically,
// when they are not active. It can also choose boot-mode and chroot-mode automatically.
// You may change this behaviour by SetPreference().
//
// stdin, stdout and stderr can be nil.
func (c *Container) CommandRaw(proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	return c.CommandRawContext(context.Background(), proc, stdin, stdout, stderr, args...)
}

// CommandContext is Command() with context.
func (c *Container) CommandContext(ctx context.Context, cmdline string) int {
	return c.CommandRawContext(ctx, SHELLPATH, os.Stdin, os.Stdout, os.Stderr, "-l", "-c", cmdline)
}

// CommandRawContext is CommandRaw() with context.
func (c *Container) CommandRawContext(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	if !c.IsFileSystemMounted() {
		if err := c.Mount(); err != nil {
			panic(err)
		}
	}
	c.lock.RLock()
	booted := c.booted
	boot := c.boot
	c.lock.RUnlock()
	if booted {
		return c.systemdRun(ctx, proc, stdin, stdout, stderr, args...)
	} else {
		if boot && c.IsBootable() {
			c.systemdNspawnBoot()
			return c.systemdRun(ctx, proc, stdin, stdout, stderr, args...)
		} else {
			return c.systemdNspawnRun(ctx, proc, stdin, stdout, stderr, args...)
		}
	}
}

// Shutdown the container and unmount file system.
func (c *Container) Shutdown() error {
	return c.machinectlShutdown()
}

// IsContainerActive returns whether the container is running or not.
func (c *Container) IsContainerActive() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.booted || c.chrooted
}

// SetPreference changes the preference of container.
//
// <boot>: (default: true) CommandRaw() will boot system on container,
// if the file system is bootable.
// When you set it to "false", CommandRaw() will only chroot,
// even the file system is bootable.
func (c *Container) SetPreference(boot bool) {
	c.lock.Lock()
	c.boot = boot
	c.lock.Unlock()
}

// SetProperties specifies the properties of container (only for boot-mode).
//
// You may use SetProperty() instead. For clear settings, use SetProperties(nil).
func (c *Container) SetProperties(properties []string) {
	c.lock.Lock()
	if properties == nil {
		properties = []string{}
	}
	c.properties = properties
	c.lock.Unlock()
}

// SetProperty appends a property of container (only for boot-mode).
//
// For understanding what "properties" are,
// please check out https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html
//
// Example:
//     SetProperty("CPUQuota=80%")
//     SetProperty("MemoryMax=70%")
func (c *Container) SetProperty(property string) {
	c.lock.Lock()
	c.properties = append(c.properties, property)
	c.lock.Unlock()
}

// IsFileSystemActive returns whether the file system has been mounted or not.
func (c *Container) IsFileSystemMounted() bool {
	return c.fs.isMounted()
}

// IsBootable returns whether the file system is bootable or not.
//
// NOTE: The basis of determining is the file /usr/lib/systemd/systemd.
func (c *Container) IsBootable() bool {
	return c.fs.isBootable()
}

// SetBaseDir sets the base directory for components of the container.
func (c *Container) SetBaseDir(path string) {
	c.fs.setBaseDir(path)
}

// Mount the file system to a temporary directory.
// It will be called automatically by CommandRaw().
func (c *Container) Mount() error {
	return c.fs.mount()
}

// Unmount the file system, and cleans the temporary directories.
func (c *Container) Unmount() error {
	return c.fs.unmount()
}

// Filesystem returns a Filesystem structure copy of the internal one.
// So modify it won't take any effect in the container.
func (c *Container) Filesystem() Filesystem {
	var fs Filesystem
	fs = *c.fs
	return fs
}
