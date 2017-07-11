package ciel

import (
	"context"
	"io"
	"os"
	"sync"
)

// ShellPath is the path of shell in container.
const ShellPath = "/bin/bash"

// FileSystemLayers specifies the layer structure of file system
var FileSystemLayers Layers

// Container represents an instance of your container.
//
// FIXME: It's not coroutine-safe so far.
type Container struct {
	lock sync.RWMutex

	Name       string
	Fs         *FileSystem
	properties []string

	boot       bool
	booted     bool
	cancelBoot chan struct{}

	chrooted bool
}

// New creates a container descriptor, but it won't start the container immediately.
//
// You may want to call Command() after this.
func New(name, baseDir string) *Container {
	c := &Container{
		Name:       name,
		properties: []string{},
		boot:       true,
		cancelBoot: make(chan struct{}),
	}
	c.Fs = newFileSystem(baseDir, FileSystemLayers)
	return c
}

// Command calls the command line with shell ("ShellPath -l -c <cmdline>") in container,
// returns the exit code.
//
// Don't worry about mounting file system, starting container and the mode of booting.
// Please check out CommandRaw() for more details.
//
// NOTE: It calls CommandRaw() internally, using os.Stdin, os.Stdout, os.Stderr.
func (c *Container) Command(cmdline string) int {
	return c.CommandContext(context.Background(), cmdline)
}

// Shell opens the shell in container.
func (c *Container) Shell() int {
	return c.ShellContext(context.Background())
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
	return c.CommandRawContext(ctx, ShellPath, os.Stdin, os.Stdout, os.Stderr, "-l", "-c", cmdline)
}

// ShellContext is Shell() with context.
func (c *Container) ShellContext(ctx context.Context) int {
	return c.CommandRawContext(ctx, ShellPath, os.Stdin, os.Stdout, os.Stderr)
}

// CommandRawContext is CommandRaw() with context.
func (c *Container) CommandRawContext(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	if !c.Fs.IsMounted() {
		if err := c.Fs.Mount(); err != nil {
			panic(err)
		}
	}
	c.lock.RLock()
	booted := c.booted
	boot := c.boot
	c.lock.RUnlock()
	if booted {
		return c.systemdRun(ctx, proc, stdin, stdout, stderr, args...)
	}
	if boot && c.Fs.IsBootable() {
		c.systemdNspawnBoot()
		return c.systemdRun(ctx, proc, stdin, stdout, stderr, args...)
	}
	return c.systemdNspawnRun(ctx, proc, stdin, stdout, stderr, args...)
}

// Shutdown the container and unmount file system.
func (c *Container) Shutdown() error {
	return c.machinectlShutdown()
}

// IsActive returns whether the container is running or not.
func (c *Container) IsActive() bool {
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
